package testrunner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// TestResult represents the result of a single test
type TestResult struct {
	PackageName string    `json:"package_name"`
	TestName    string    `json:"test_name"`
	Status      string    `json:"status"` // pass, fail, skip
	Duration    float64   `json:"duration"` // seconds
	Output      string    `json:"output"` // failure message/logs
	LastRun     time.Time `json:"last_run"`
}

// TestSummary represents aggregate test statistics
type TestSummary struct {
	TotalTests    int       `json:"total_tests"`
	PassedTests   int       `json:"passed_tests"`
	FailedTests   int       `json:"failed_tests"`
	SkippedTests  int       `json:"skipped_tests"`
	TotalDuration float64   `json:"total_duration"` // seconds
	LastRunTime   time.Time `json:"last_run_time"`
	IsRunning     bool      `json:"is_running"`
}

// TestRunner manages test execution and result caching
type TestRunner struct {
	projectRoot   string
	mutex         sync.RWMutex
	results       []TestResult
	summary       TestSummary
	lastRun       time.Time
	isRunning     bool
	needsRun      bool // Flag indicating Go files were modified
}

// NewTestRunner creates a new test runner
func NewTestRunner(projectRoot string) *TestRunner {
	return &TestRunner{
		projectRoot: projectRoot,
		results:     []TestResult{},
		needsRun:    false,
	}
}

// MarkDirty marks that tests need to be run (called after Go file edits)
func (runner *TestRunner) MarkDirty() {
	runner.mutex.Lock()
	defer runner.mutex.Unlock()
	runner.needsRun = true
}

// RunIfDirty runs tests only if files were modified since last run
func (runner *TestRunner) RunIfDirty() error {
	runner.mutex.Lock()
	needsRun := runner.needsRun
	runner.mutex.Unlock()

	if !needsRun {
		return nil // No changes, skip
	}

	return runner.RunTests("")
}

// RunTests executes go test and parses results (can specify package filter)
func (runner *TestRunner) RunTests(packageFilter string) error {
	runner.mutex.Lock()
	if runner.isRunning {
		runner.mutex.Unlock()
		return nil // Already running
	}
	runner.isRunning = true
	runner.needsRun = false // Clear dirty flag
	runner.mutex.Unlock()

	defer func() {
		runner.mutex.Lock()
		runner.isRunning = false
		runner.lastRun = time.Now()
		runner.mutex.Unlock()
	}()

	// Build test command
	args := []string{"test", "-json"}
	
	if packageFilter != "" {
		args = append(args, packageFilter)
	} else {
		args = append(args, "./...")
	}

	command := exec.Command("go", args...)
	command.Dir = runner.projectRoot

	// Capture stdout
	stdout, err := command.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := command.Start(); err != nil {
		return fmt.Errorf("failed to start go test: %w", err)
	}

	// Parse JSON output line by line
	results := []TestResult{}
	scanner := bufio.NewScanner(stdout)
	testOutputs := make(map[string][]string) // Collect output per test

	for scanner.Scan() {
		line := scanner.Text()
		
		var event TestEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue // Skip non-JSON lines
		}

		switch event.Action {
		case "run":
			// Test started
			continue
			
		case "output":
			// Collect output for this test
			if event.Test != "" {
				key := event.Package + "::" + event.Test
				testOutputs[key] = append(testOutputs[key], event.Output)
			}
			
		case "pass", "fail", "skip":
			// Test completed
			if event.Test == "" {
				continue // Package-level result, skip
			}
			
			key := event.Package + "::" + event.Test
			output := strings.Join(testOutputs[key], "")
			
			result := TestResult{
				PackageName: event.Package,
				TestName:    event.Test,
				Status:      event.Action,
				Duration:    event.Elapsed,
				Output:      output,
				LastRun:     time.Now(),
			}
			
			results = append(results, result)
			delete(testOutputs, key) // Clean up
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading test output: %w", err)
	}

	if err := command.Wait(); err != nil {
		// go test returns non-zero if tests fail, which is expected
		// We still want to process the results
	}

	// Update cached results
	runner.mutex.Lock()
	runner.results = results
	runner.summary = runner.computeSummary(results)
	runner.mutex.Unlock()

	return nil
}

// TestEvent represents a single event from go test -json
type TestEvent struct {
	Time    string  `json:"Time"`
	Action  string  `json:"Action"` // run, output, pass, fail, skip
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Output  string  `json:"Output"`
	Elapsed float64 `json:"Elapsed"` // seconds
}

// computeSummary calculates aggregate statistics
func (runner *TestRunner) computeSummary(results []TestResult) TestSummary {
	summary := TestSummary{
		TotalTests:    len(results),
		PassedTests:   0,
		FailedTests:   0,
		SkippedTests:  0,
		TotalDuration: 0,
		LastRunTime:   time.Now(),
		IsRunning:     false,
	}

	for _, result := range results {
		summary.TotalDuration += result.Duration
		
		switch result.Status {
		case "pass":
			summary.PassedTests++
		case "fail":
			summary.FailedTests++
		case "skip":
			summary.SkippedTests++
		}
	}

	return summary
}

// GetSummary returns the current test summary (thread-safe)
func (runner *TestRunner) GetSummary() TestSummary {
	runner.mutex.RLock()
	defer runner.mutex.RUnlock()
	
	summary := runner.summary
	summary.IsRunning = runner.isRunning
	return summary
}

// GetFailingTests returns all failed tests (thread-safe)
func (runner *TestRunner) GetFailingTests() []TestResult {
	runner.mutex.RLock()
	defer runner.mutex.RUnlock()

	var failing []TestResult
	for _, result := range runner.results {
		if result.Status == "fail" {
			failing = append(failing, result)
		}
	}
	return failing
}

// GetAllResults returns all test results (thread-safe)
func (runner *TestRunner) GetAllResults() []TestResult {
	runner.mutex.RLock()
	defer runner.mutex.RUnlock()

	// Return a copy
	resultsCopy := make([]TestResult, len(runner.results))
	copy(resultsCopy, runner.results)
	return resultsCopy
}

// GetPackageResults returns results for a specific package
func (runner *TestRunner) GetPackageResults(packageName string) []TestResult {
	runner.mutex.RLock()
	defer runner.mutex.RUnlock()

	var packageResults []TestResult
	for _, result := range runner.results {
		if result.PackageName == packageName {
			packageResults = append(packageResults, result)
		}
	}
	return packageResults
}

// IsRunning returns whether tests are currently executing
func (runner *TestRunner) IsRunning() bool {
	runner.mutex.RLock()
	defer runner.mutex.RUnlock()
	return runner.isRunning
}

// GetLastRunTime returns when tests were last executed
func (runner *TestRunner) GetLastRunTime() time.Time {
	runner.mutex.RLock()
	defer runner.mutex.RUnlock()
	return runner.lastRun
}
