package analyzer

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

// Diagnostic represents a single issue found in code, extracted from a
// build/lint command's output by an AI summarization pass.
type Diagnostic struct {
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Severity string `json:"severity"` // error, warning, info
	Message  string `json:"message"`
	Source   string `json:"source"` // the command that produced it
}

// AnalysisResult is the structured shape we expect the AI to return after
// reading raw build/lint output.
type AnalysisResult struct {
	Status      string       `json:"status"` // "pass" or "fail"
	Summary     string       `json:"summary"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Analyzer runs a user-supplied build/lint command and uses an AI provider
// to turn raw output into structured diagnostics with a binary pass/fail
// status. It is language-agnostic.
type Analyzer struct {
	projectRoot string
	provider    ai_provider.Provider

	mutex        sync.RWMutex
	lastCommand  string
	lastStatus   string
	lastSummary  string
	diagnostics  []Diagnostic
	lastAnalysis time.Time
	isAnalyzing  bool
}

// NewAnalyzer creates a new code analyzer. The provider is used to summarize
// raw command output into structured diagnostics.
func NewAnalyzer(projectRoot string, provider ai_provider.Provider) *Analyzer {
	return &Analyzer{
		projectRoot: projectRoot,
		provider:    provider,
		diagnostics: []Diagnostic{},
		lastStatus:  "unknown",
	}
}

// SetCommand records the build/lint command to use for subsequent background
// runs (e.g. after file edits). Does not run the command.
func (analyzer *Analyzer) SetCommand(command string) {
	analyzer.mutex.Lock()
	defer analyzer.mutex.Unlock()
	analyzer.lastCommand = command
}

// LastCommand returns the most recently used build/lint command, or empty
// string if none has been set.
func (analyzer *Analyzer) LastCommand() string {
	analyzer.mutex.RLock()
	defer analyzer.mutex.RUnlock()
	return analyzer.lastCommand
}

// AnalyzeAsync triggers a background analysis using the last-known command.
// No-op if no command has ever been supplied.
func (analyzer *Analyzer) AnalyzeAsync() {
	analyzer.mutex.RLock()
	command := analyzer.lastCommand
	analyzer.mutex.RUnlock()
	if command == "" {
		return
	}
	go func() {
		// Background analyses get their own context with a generous timeout
		// so they cannot block forever on a hung build.
		backgroundContext, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		_ = analyzer.Analyze(backgroundContext, command)
	}()
}

// Analyze runs the supplied command in the project root, captures its
// combined output, and uses the AI provider to extract pass/fail status
// and structured diagnostics. The command is remembered for future
// background runs.
func (analyzer *Analyzer) Analyze(parentContext context.Context, command string) error {
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("analyze: command must not be empty")
	}

	analyzer.mutex.Lock()
	if analyzer.isAnalyzing {
		analyzer.mutex.Unlock()
		return nil // Already running; skip to avoid pile-up.
	}
	analyzer.isAnalyzing = true
	analyzer.lastCommand = command
	analyzer.mutex.Unlock()

	defer func() {
		analyzer.mutex.Lock()
		analyzer.isAnalyzing = false
		analyzer.lastAnalysis = time.Now()
		analyzer.mutex.Unlock()
	}()

	// Run the command, capturing both streams.
	executionContext, cancel := context.WithTimeout(parentContext, 5*time.Minute)
	defer cancel()

	shellCommand := exec.CommandContext(executionContext, "sh", "-c", command)
	shellCommand.Dir = analyzer.projectRoot

	var stdoutBuffer, stderrBuffer bytes.Buffer
	shellCommand.Stdout = &stdoutBuffer
	shellCommand.Stderr = &stderrBuffer

	runError := shellCommand.Run()
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

	// Summarize via AI even on exit 0 — a successful exit code is the
	// strongest pass signal, but we still let the model produce a summary.
	analysisResult, summarizeError := analyzer.summarize(parentContext, command, exitCode, combinedOutput)
	if summarizeError != nil {
		// Fall back to a heuristic so the agent still gets a usable result.
		analysisResult = fallbackResult(exitCode, combinedOutput, command)
	}

	analyzer.mutex.Lock()
	analyzer.lastStatus = analysisResult.Status
	analyzer.lastSummary = analysisResult.Summary
	analyzer.diagnostics = analysisResult.Diagnostics
	analyzer.mutex.Unlock()

	// We deliberately do not return runError to the caller: a non-zero exit
	// code is the normal "fail" path and the structured result already
	// reflects it.
	_ = runError
	return nil
}

// summarize asks the AI provider to convert raw command output into a
// structured AnalysisResult. Returns an error if the model response cannot
// be parsed.
func (analyzer *Analyzer) summarize(parentContext context.Context, command string, exitCode int, rawOutput string) (AnalysisResult, error) {
	// Cap raw output to avoid blowing the model's context.
	const maximumOutputCharacters = 16000
	truncatedOutput := rawOutput
	wasTruncated := false
	if len(truncatedOutput) > maximumOutputCharacters {
		// Keep the tail — error messages typically appear last.
		truncatedOutput = "...[output truncated, showing last portion]...\n" +
			truncatedOutput[len(truncatedOutput)-maximumOutputCharacters:]
		wasTruncated = true
	}

	systemPrompt := `You analyze the output of a developer's build, compile, or lint command and return a strict JSON object describing the result.

The JSON object MUST conform to this schema:
{
  "status": "pass" | "fail",
  "summary": string,
  "diagnostics": [
    {
      "file_path": string,
      "line": integer,
      "column": integer,
      "severity": "error" | "warning" | "info",
      "message": string,
      "source": string
    }
  ]
}

Rules:
- "status" is "fail" if the command produced any errors that prevented success, otherwise "pass".
- Each diagnostic represents one distinct issue. Use line/column 0 if unknown.
- "source" should identify the originating tool (e.g. "build", "lint", "compiler"); if unknown, use the command name.
- Return ONLY the JSON object. No prose, no markdown fences.`

	userPrompt := fmt.Sprintf(
		"Command executed: %s\nExit code: %d\nTruncated: %v\n\n--- BEGIN OUTPUT ---\n%s\n--- END OUTPUT ---",
		command, exitCode, wasTruncated, truncatedOutput,
	)

	request := ai_provider.ChatCompletionRequest{
		Messages: []ai_provider.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	response, providerError := analyzer.provider.ChatCompletion(parentContext, request)
	if providerError != nil {
		return AnalysisResult{}, fmt.Errorf("ai summarization failed: %w", providerError)
	}
	if len(response.Choices) == 0 {
		return AnalysisResult{}, fmt.Errorf("ai summarization returned no choices")
	}

	rawResponse := strings.TrimSpace(response.Choices[0].Message.Content)
	jsonPayload := extractJSONObject(rawResponse)
	if jsonPayload == "" {
		return AnalysisResult{}, fmt.Errorf("ai summarization returned no JSON: %q", rawResponse)
	}

	var result AnalysisResult
	if unmarshalError := json.Unmarshal([]byte(jsonPayload), &result); unmarshalError != nil {
		return AnalysisResult{}, fmt.Errorf("failed to parse ai response: %w", unmarshalError)
	}

	// Normalize status.
	switch strings.ToLower(strings.TrimSpace(result.Status)) {
	case "pass":
		result.Status = "pass"
	default:
		result.Status = "fail"
	}

	// Default source on diagnostics that omit it.
	commandName := firstWord(command)
	for index := range result.Diagnostics {
		if strings.TrimSpace(result.Diagnostics[index].Source) == "" {
			result.Diagnostics[index].Source = commandName
		}
		if strings.TrimSpace(result.Diagnostics[index].Severity) == "" {
			result.Diagnostics[index].Severity = "error"
		}
	}

	return result, nil
}

// fallbackResult is used when the AI summarization fails; produces a
// minimally useful result from exit code alone.
func fallbackResult(exitCode int, rawOutput string, command string) AnalysisResult {
	status := "pass"
	if exitCode != 0 {
		status = "fail"
	}
	summary := fmt.Sprintf("AI summarization unavailable; reporting raw exit code %d.", exitCode)
	diagnostics := []Diagnostic{}
	if exitCode != 0 {
		excerpt := strings.TrimSpace(rawOutput)
		if len(excerpt) > 1000 {
			excerpt = excerpt[len(excerpt)-1000:]
		}
		diagnostics = append(diagnostics, Diagnostic{
			Severity: "error",
			Message:  excerpt,
			Source:   firstWord(command),
		})
	}
	return AnalysisResult{Status: status, Summary: summary, Diagnostics: diagnostics}
}

// extractJSONObject returns the first balanced {...} substring found, to be
// resilient against models that wrap JSON in prose or code fences.
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

func firstWord(command string) string {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return "command"
	}
	for index := 0; index < len(trimmed); index++ {
		if trimmed[index] == ' ' || trimmed[index] == '\t' {
			return trimmed[:index]
		}
	}
	return trimmed
}

// GetDiagnostics returns all current diagnostics (thread-safe).
func (analyzer *Analyzer) GetDiagnostics() []Diagnostic {
	analyzer.mutex.RLock()
	defer analyzer.mutex.RUnlock()

	diagnosticsCopy := make([]Diagnostic, len(analyzer.diagnostics))
	copy(diagnosticsCopy, analyzer.diagnostics)
	return diagnosticsCopy
}

// GetFileDiagnostics returns diagnostics for a specific file.
func (analyzer *Analyzer) GetFileDiagnostics(filePath string) []Diagnostic {
	analyzer.mutex.RLock()
	defer analyzer.mutex.RUnlock()

	var fileDiagnostics []Diagnostic
	for _, diagnostic := range analyzer.diagnostics {
		if diagnostic.FilePath == filePath {
			fileDiagnostics = append(fileDiagnostics, diagnostic)
		}
	}
	return fileDiagnostics
}

// GetStatus returns the current analysis status (thread-safe).
func (analyzer *Analyzer) GetStatus() map[string]any {
	analyzer.mutex.RLock()
	defer analyzer.mutex.RUnlock()

	errorCount := 0
	warningCount := 0
	for _, diagnostic := range analyzer.diagnostics {
		switch diagnostic.Severity {
		case "error":
			errorCount++
		case "warning":
			warningCount++
		}
	}

	return map[string]any{
		"is_analyzing":  analyzer.isAnalyzing,
		"last_analysis": analyzer.lastAnalysis,
		"last_command":  analyzer.lastCommand,
		"last_status":   analyzer.lastStatus,
		"last_summary":  analyzer.lastSummary,
		"error_count":   errorCount,
		"warning_count": warningCount,
	}
}
