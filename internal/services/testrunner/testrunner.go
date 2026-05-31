package testrunner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/exar/babycoder/internal/services/ai_provider"
)

// FailingTest describes a single failing test extracted from raw test output
// by the AI summarization pass.
type FailingTest struct {
	Name    string `json:"name"`
	Output  string `json:"output"`
	Message string `json:"message"`
}

// TestSummary is the language-agnostic summary of a test command's results.
type TestSummary struct {
	Status        string        `json:"status"` // pass, fail, unknown
	TotalTests    int           `json:"total_tests"`
	PassedTests   int           `json:"passed_tests"`
	FailedTests   int           `json:"failed_tests"`
	SkippedTests  int           `json:"skipped_tests"`
	Duration      float64       `json:"duration"`
	FailingTests  []FailingTest `json:"failing_tests"`
	Summary       string        `json:"summary"`
	RawOutputTail string        `json:"raw_output_tail"`
	LastCommand   string        `json:"last_command"`
	LastRunTime   time.Time     `json:"last_run_time"`
	IsRunning     bool          `json:"is_running"`
}

// aiTestSummary is the strict JSON shape we ask the model to return.
type aiTestSummary struct {
	Status       string        `json:"status"`
	TotalTests   int           `json:"total_tests"`
	PassedTests  int           `json:"passed_tests"`
	FailedTests  int           `json:"failed_tests"`
	SkippedTests int           `json:"skipped_tests"`
	Duration     float64       `json:"duration"`
	Summary      string        `json:"summary"`
	FailingTests []FailingTest `json:"failing_tests"`
}

// TestRunner runs a user-supplied test command and uses an AI provider to
// turn raw output into a structured pass/fail summary. Language-agnostic.
type TestRunner struct {
	projectRoot string
	provider    ai_provider.Provider

	mutex       sync.RWMutex
	lastCommand string
	summary     TestSummary
	lastRun     time.Time
	isRunning   bool
	needsRun    bool // Flag set after any source file is edited.
}

// NewTestRunner creates a new test runner.
func NewTestRunner(projectRoot string, provider ai_provider.Provider) *TestRunner {
	return &TestRunner{
		projectRoot: projectRoot,
		provider:    provider,
		summary:     TestSummary{Status: "unknown"},
	}
}

// SetCommand records the test command to use for subsequent background runs.
func (runner *TestRunner) SetCommand(command string) {
	runner.mutex.Lock()
	defer runner.mutex.Unlock()
	runner.lastCommand = command
}

// LastCommand returns the last-known test command, or empty if unset.
func (runner *TestRunner) LastCommand() string {
	runner.mutex.RLock()
	defer runner.mutex.RUnlock()
	return runner.lastCommand
}

// MarkDirty marks that tests need to be run (called after any file edit).
func (runner *TestRunner) MarkDirty() {
	runner.mutex.Lock()
	defer runner.mutex.Unlock()
	runner.needsRun = true
}

// RunIfDirty runs tests only if files were modified since the last run AND
// a test command has been previously supplied.
func (runner *TestRunner) RunIfDirty(parentContext context.Context) error {
	runner.mutex.Lock()
	needsRun := runner.needsRun
	command := runner.lastCommand
	runner.mutex.Unlock()

	if !needsRun || command == "" {
		return nil
	}

	return runner.RunTests(parentContext, command)
}

// RunTests executes the supplied test command, captures combined output, and
// uses the AI provider to extract a pass/fail summary plus failing test
// details. The command is remembered for future background runs.
func (runner *TestRunner) RunTests(parentContext context.Context, command string) error {
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("run_tests: command must not be empty")
	}

	runner.mutex.Lock()
	if runner.isRunning {
		runner.mutex.Unlock()
		return nil
	}
	runner.isRunning = true
	runner.needsRun = false
	runner.lastCommand = command
	runner.mutex.Unlock()

	startTime := time.Now()
	defer func() {
		runner.mutex.Lock()
		runner.isRunning = false
		runner.lastRun = time.Now()
		runner.mutex.Unlock()
	}()

	executionContext, cancel := context.WithTimeout(parentContext, 10*time.Minute)
	defer cancel()

	shellCommand := exec.CommandContext(executionContext, "sh", "-c", command)
	shellCommand.Dir = runner.projectRoot

	var stdoutBuffer, stderrBuffer bytes.Buffer
	shellCommand.Stdout = &stdoutBuffer
	shellCommand.Stderr = &stderrBuffer

	runError := shellCommand.Run()
	wallClockDuration := time.Since(startTime).Seconds()
	exitCode := 0
	if shellCommand.ProcessState != nil {
		exitCode = shellCommand.ProcessState.ExitCode()
	}

	combinedOutput := stdoutBuffer.String()
	if stderrBuffer.Len() > 0 {
		if combinedOutput != "" {
			combinedOutput += "\n"
		}
		combinedOutput += stderrBuffer.String()
	}

	summary := runner.summarize(parentContext, command, exitCode, wallClockDuration, combinedOutput)
	summary.LastCommand = command
	summary.LastRunTime = time.Now()
	summary.RawOutputTail = tailString(combinedOutput, 4000)

	runner.mutex.Lock()
	runner.summary = summary
	runner.mutex.Unlock()

	_ = runError
	return nil
}

// summarize calls the AI provider to convert raw test output into a
// TestSummary. Falls back to a heuristic on failure.
func (runner *TestRunner) summarize(parentContext context.Context, command string, exitCode int, wallClockDuration float64, rawOutput string) TestSummary {
	const maximumOutputCharacters = 16000
	truncatedOutput := rawOutput
	wasTruncated := false
	if len(truncatedOutput) > maximumOutputCharacters {
		truncatedOutput = "...[output truncated, showing last portion]...\n" +
			truncatedOutput[len(truncatedOutput)-maximumOutputCharacters:]
		wasTruncated = true
	}

	systemPrompt := `You analyze the output of a developer's test command and return a strict JSON object summarizing the results.

The JSON object MUST conform to this schema:
{
  "status": "pass" | "fail",
  "total_tests": integer,
  "passed_tests": integer,
  "failed_tests": integer,
  "skipped_tests": integer,
  "duration": number,
  "summary": string,
  "failing_tests": [
    { "name": string, "message": string, "output": string }
  ]
}

Rules:
- "status" is "pass" if and only if every test passed. Any failure means "fail".
- If you cannot determine a count, use 0. If the test framework is unknown, do your best inferring from output.
- "failing_tests[].output" should be a SHORT excerpt (a few lines) of the most relevant failure context.
- Return ONLY the JSON object. No prose, no markdown fences.`

	userPrompt := fmt.Sprintf(
		"Test command: %s\nExit code: %d\nWall-clock duration (seconds): %.2f\nTruncated: %v\n\n--- BEGIN OUTPUT ---\n%s\n--- END OUTPUT ---",
		command, exitCode, wallClockDuration, wasTruncated, truncatedOutput,
	)

	request := ai_provider.ChatCompletionRequest{
		Messages: []ai_provider.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	response, providerError := runner.provider.ChatCompletion(parentContext, request)
	if providerError != nil || len(response.Choices) == 0 {
		return fallbackSummary(exitCode, wallClockDuration, rawOutput)
	}

	rawResponse := strings.TrimSpace(response.Choices[0].Message.Content)
	jsonPayload := extractJSONObject(rawResponse)
	if jsonPayload == "" {
		return fallbackSummary(exitCode, wallClockDuration, rawOutput)
	}

	var parsed aiTestSummary
	if unmarshalError := json.Unmarshal([]byte(jsonPayload), &parsed); unmarshalError != nil {
		return fallbackSummary(exitCode, wallClockDuration, rawOutput)
	}

	status := "fail"
	if strings.EqualFold(strings.TrimSpace(parsed.Status), "pass") {
		status = "pass"
	}
	if parsed.Duration == 0 {
		parsed.Duration = wallClockDuration
	}

	return TestSummary{
		Status:       status,
		TotalTests:   parsed.TotalTests,
		PassedTests:  parsed.PassedTests,
		FailedTests:  parsed.FailedTests,
		SkippedTests: parsed.SkippedTests,
		Duration:     parsed.Duration,
		Summary:      parsed.Summary,
		FailingTests: parsed.FailingTests,
	}
}

func fallbackSummary(exitCode int, wallClockDuration float64, rawOutput string) TestSummary {
	status := "pass"
	if exitCode != 0 {
		status = "fail"
	}
	failingTests := []FailingTest{}
	if exitCode != 0 {
		excerpt := tailString(strings.TrimSpace(rawOutput), 1000)
		failingTests = append(failingTests, FailingTest{
			Name:    "unknown",
			Message: fmt.Sprintf("test command exited with code %d", exitCode),
			Output:  excerpt,
		})
	}
	return TestSummary{
		Status:       status,
		FailingTests: failingTests,
		Duration:     wallClockDuration,
		Summary:      fmt.Sprintf("AI summarization unavailable; reporting raw exit code %d.", exitCode),
	}
}

func extractJSONObject(text string) string {
	startIndex := strings.Index(text, "{")
	if startIndex < 0 {
		return ""
	}
	depth := 0
	for index := startIndex; index < len(text); index++ {
		switch text[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[startIndex : index+1]
			}
		}
	}
	return ""
}

func tailString(text string, maximumCharacters int) string {
	if len(text) <= maximumCharacters {
		return text
	}
	return "...[truncated]...\n" + text[len(text)-maximumCharacters:]
}

// GetSummary returns the current test summary (thread-safe).
func (runner *TestRunner) GetSummary() TestSummary {
	runner.mutex.RLock()
	defer runner.mutex.RUnlock()

	summaryCopy := runner.summary
	summaryCopy.IsRunning = runner.isRunning
	return summaryCopy
}

// GetFailingTests returns all currently failing tests (thread-safe).
func (runner *TestRunner) GetFailingTests() []FailingTest {
	runner.mutex.RLock()
	defer runner.mutex.RUnlock()

	failingCopy := make([]FailingTest, len(runner.summary.FailingTests))
	copy(failingCopy, runner.summary.FailingTests)
	return failingCopy
}

// IsRunning returns whether tests are currently executing.
func (runner *TestRunner) IsRunning() bool {
	runner.mutex.RLock()
	defer runner.mutex.RUnlock()
	return runner.isRunning
}

// GetLastRunTime returns when tests were last executed.
func (runner *TestRunner) GetLastRunTime() time.Time {
	runner.mutex.RLock()
	defer runner.mutex.RUnlock()
	return runner.lastRun
}
