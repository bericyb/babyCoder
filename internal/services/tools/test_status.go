package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/exar/babycoder/internal/services/ai_provider"
	"github.com/exar/babycoder/internal/services/testrunner"
)

// GetTestStatusTool returns the most recent test summary.
type GetTestStatusTool struct {
	testRunner *testrunner.TestRunner
}

// GetDefinition returns the tool definition for the AI provider.
func (tool *GetTestStatusTool) GetDefinition() ai_provider.Tool {
	return ai_provider.Tool{
		Type: "function",
		Function: ai_provider.ToolFunction{
			Name:        "get_test_status",
			Description: "Get the most recent test summary (pass/fail status, counts, duration). Reflects the last run of run_tests.",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
				"required":   []string{},
			},
		},
	}
}

// Execute runs the tool.
func (tool *GetTestStatusTool) Execute(arguments map[string]any) (string, error) {
	summary := tool.testRunner.GetSummary()

	var output strings.Builder
	output.WriteString("=== Test Status ===\n\n")

	if summary.IsRunning {
		output.WriteString("Tests are currently running...\n\n")
		if !summary.LastRunTime.IsZero() {
			output.WriteString(fmt.Sprintf("Last completed run: %s\n", summary.LastRunTime.Format("2006-01-02 15:04:05")))
		}
		return output.String(), nil
	}

	if summary.LastCommand == "" {
		output.WriteString("No test command has been run yet.\n")
		output.WriteString("Call 'run_tests' with a test_command to execute tests for this project.\n")
		return output.String(), nil
	}

	output.WriteString(fmt.Sprintf("Command: %s\n", summary.LastCommand))
	output.WriteString(fmt.Sprintf("Status:  %s\n", strings.ToUpper(summary.Status)))
	if summary.Summary != "" {
		output.WriteString(fmt.Sprintf("Summary: %s\n", summary.Summary))
	}
	output.WriteString("\n")

	if summary.TotalTests > 0 {
		output.WriteString(fmt.Sprintf("Total:    %d tests\n", summary.TotalTests))
		passRate := float64(summary.PassedTests) / float64(summary.TotalTests) * 100
		output.WriteString(fmt.Sprintf("Passed:   %d (%.1f%%)\n", summary.PassedTests, passRate))
		output.WriteString(fmt.Sprintf("Failed:   %d\n", summary.FailedTests))
		if summary.SkippedTests > 0 {
			output.WriteString(fmt.Sprintf("Skipped:  %d\n", summary.SkippedTests))
		}
	}
	output.WriteString(fmt.Sprintf("Duration: %.2fs\n", summary.Duration))
	output.WriteString(fmt.Sprintf("Last run: %s\n", summary.LastRunTime.Format("2006-01-02 15:04:05")))

	if summary.FailedTests > 0 || len(summary.FailingTests) > 0 {
		output.WriteString("\nUse 'get_failing_tests' to see failure details.\n")
	}

	return output.String(), nil
}

// GetFailingTestsTool returns detailed information about failing tests.
type GetFailingTestsTool struct {
	testRunner *testrunner.TestRunner
}

// GetDefinition returns the tool definition for the AI provider.
func (tool *GetFailingTestsTool) GetDefinition() ai_provider.Tool {
	return ai_provider.Tool{
		Type: "function",
		Function: ai_provider.ToolFunction{
			Name:        "get_failing_tests",
			Description: "Get detailed information about all currently failing tests including names, messages, and output excerpts.",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
				"required":   []string{},
			},
		},
	}
}

// Execute runs the tool.
func (tool *GetFailingTestsTool) Execute(arguments map[string]any) (string, error) {
	failingTests := tool.testRunner.GetFailingTests()

	if len(failingTests) == 0 {
		return "No failing tests recorded.\n", nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("=== Failing Tests (%d) ===\n\n", len(failingTests)))

	for _, failingTest := range failingTests {
		name := failingTest.Name
		if name == "" {
			name = "(unnamed)"
		}
		output.WriteString(fmt.Sprintf("x %s\n", name))
		if failingTest.Message != "" {
			output.WriteString(fmt.Sprintf("  Message: %s\n", failingTest.Message))
		}
		if failingTest.Output != "" {
			output.WriteString("  Output:\n")
			for _, line := range strings.Split(strings.TrimSpace(failingTest.Output), "\n") {
				output.WriteString("    " + line + "\n")
			}
		}
		output.WriteString("\n")
	}

	return output.String(), nil
}

// RunTestsTool forces immediate test execution using a user-supplied command.
type RunTestsTool struct {
	testRunner *testrunner.TestRunner
}

// GetDefinition returns the tool definition for the AI provider.
func (tool *RunTestsTool) GetDefinition() ai_provider.Tool {
	return ai_provider.Tool{
		Type: "function",
		Function: ai_provider.ToolFunction{
			Name: "run_tests",
			Description: "Run the project's test suite using a shell command you supply (e.g. 'pytest', 'npm test', 'cargo test', 'mvn test'). " +
				"The command is remembered and automatically re-run in the background after subsequent file edits. Returns a structured pass/fail summary.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"test_command": map[string]any{
						"type":        "string",
						"description": "Shell command to execute from the project root. If omitted, re-uses the last command supplied (errors if none).",
					},
				},
				"required": []string{},
			},
		},
	}
}

// Execute runs the tool.
func (tool *RunTestsTool) Execute(arguments map[string]any) (string, error) {
	if tool.testRunner.IsRunning() {
		return "Tests are already running. Please wait for them to complete.\n", nil
	}

	testCommand := getStringArgDefault(arguments, "test_command", "")
	if testCommand == "" {
		testCommand = tool.testRunner.LastCommand()
	}
	if testCommand == "" {
		return "", fmt.Errorf("test_command is required on the first call (no previous command remembered)")
	}

	if runError := tool.testRunner.RunTests(context.Background(), testCommand); runError != nil {
		return "", fmt.Errorf("failed to run tests: %w", runError)
	}

	summary := tool.testRunner.GetSummary()

	var output strings.Builder
	output.WriteString("=== Test Run Complete ===\n\n")
	output.WriteString(fmt.Sprintf("Command: %s\n", testCommand))
	output.WriteString(fmt.Sprintf("Status:  %s\n", strings.ToUpper(summary.Status)))
	if summary.Summary != "" {
		output.WriteString(fmt.Sprintf("Summary: %s\n", summary.Summary))
	}
	output.WriteString("\n")

	if summary.TotalTests > 0 {
		output.WriteString(fmt.Sprintf("Total:    %d tests\n", summary.TotalTests))
		output.WriteString(fmt.Sprintf("Passed:   %d\n", summary.PassedTests))
		output.WriteString(fmt.Sprintf("Failed:   %d\n", summary.FailedTests))
		if summary.SkippedTests > 0 {
			output.WriteString(fmt.Sprintf("Skipped:  %d\n", summary.SkippedTests))
		}
	}
	output.WriteString(fmt.Sprintf("Duration: %.2fs\n", summary.Duration))

	if summary.FailedTests > 0 || len(summary.FailingTests) > 0 {
		output.WriteString("\nUse 'get_failing_tests' for detailed failure information.\n")
	}

	return output.String(), nil
}
