package tools

import (
	"fmt"
	"strings"

	"github.com/exar/babycoder/internal/services/ai_provider"
	"github.com/exar/babycoder/internal/services/testrunner"
)

// GetTestStatusTool returns current test status summary
type GetTestStatusTool struct {
	testRunner *testrunner.TestRunner
}

// GetDefinition returns the tool definition for the AI provider
func (tool *GetTestStatusTool) GetDefinition() ai_provider.Tool {
	return ai_provider.Tool{
		Type: "function",
		Function: ai_provider.ToolFunction{
			Name:        "get_test_status",
			Description: "Get current test status summary including pass/fail counts, duration, and last run time. Use this to check if tests are passing after making changes.",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
				"required":   []string{},
			},
		},
	}
}

// Execute runs the tool
func (tool *GetTestStatusTool) Execute(arguments map[string]interface{}) (string, error) {
	summary := tool.testRunner.GetSummary()

	var result strings.Builder
	result.WriteString("=== Test Status ===\n\n")

	if summary.IsRunning {
		result.WriteString("⏳ Tests are currently running...\n\n")
		if !summary.LastRunTime.IsZero() {
			result.WriteString(fmt.Sprintf("Last completed run: %s\n", summary.LastRunTime.Format("2006-01-02 15:04:05")))
		}
		return result.String(), nil
	}

	if summary.TotalTests == 0 {
		result.WriteString("ℹ No tests have been run yet.\n")
		result.WriteString("Tests run automatically after file changes or use 'run_tests' to execute now.\n")
		return result.String(), nil
	}

	// Overall status
	if summary.FailedTests == 0 {
		result.WriteString("✓ ALL TESTS PASSING\n\n")
	} else {
		result.WriteString(fmt.Sprintf("✗ %d TEST(S) FAILING\n\n", summary.FailedTests))
	}

	// Statistics
	result.WriteString(fmt.Sprintf("Total:    %d tests\n", summary.TotalTests))
	result.WriteString(fmt.Sprintf("Passed:   %d (%.1f%%)\n", summary.PassedTests,
		float64(summary.PassedTests)/float64(summary.TotalTests)*100))
	result.WriteString(fmt.Sprintf("Failed:   %d\n", summary.FailedTests))
	if summary.SkippedTests > 0 {
		result.WriteString(fmt.Sprintf("Skipped:  %d\n", summary.SkippedTests))
	}
	result.WriteString(fmt.Sprintf("Duration: %.2fs\n", summary.TotalDuration))
	result.WriteString(fmt.Sprintf("Last run: %s\n", summary.LastRunTime.Format("2006-01-02 15:04:05")))

	if summary.FailedTests > 0 {
		result.WriteString("\nℹ Use 'get_failing_tests' to see failure details.\n")
	}

	return result.String(), nil
}

// GetFailingTestsTool returns detailed information about failing tests
type GetFailingTestsTool struct {
	testRunner *testrunner.TestRunner
}

// GetDefinition returns the tool definition for the AI provider
func (tool *GetFailingTestsTool) GetDefinition() ai_provider.Tool {
	return ai_provider.Tool{
		Type: "function",
		Function: ai_provider.ToolFunction{
			Name:        "get_failing_tests",
			Description: "Get detailed information about all currently failing tests including error messages and output. Use this to understand what needs to be fixed.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"package_filter": map[string]interface{}{
						"type":        "string",
						"description": "Optional: filter to specific package (e.g., 'github.com/exar/babycoder/internal/services/agent')",
					},
				},
				"required": []string{},
			},
		},
	}
}

// Execute runs the tool
func (tool *GetFailingTestsTool) Execute(arguments map[string]interface{}) (string, error) {
	packageFilter := ""
	if pkgArg, exists := arguments["package_filter"]; exists {
		if pkgStr, ok := pkgArg.(string); ok {
			packageFilter = pkgStr
		}
	}

	failingTests := tool.testRunner.GetFailingTests()

	if len(failingTests) == 0 {
		return "✓ No failing tests! All tests are passing.\n", nil
	}

	// Filter by package if requested
	if packageFilter != "" {
		var filtered []testrunner.TestResult
		for _, test := range failingTests {
			if test.PackageName == packageFilter {
				filtered = append(filtered, test)
			}
		}
		failingTests = filtered

		if len(failingTests) == 0 {
			return fmt.Sprintf("✓ No failing tests in package '%s'\n", packageFilter), nil
		}
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("=== Failing Tests (%d) ===\n\n", len(failingTests)))

	// Group by package
	packageTests := make(map[string][]testrunner.TestResult)
	for _, test := range failingTests {
		packageTests[test.PackageName] = append(packageTests[test.PackageName], test)
	}

	for packageName, tests := range packageTests {
		result.WriteString(fmt.Sprintf("Package: %s\n", packageName))
		result.WriteString(strings.Repeat("-", 60) + "\n\n")

		for _, test := range tests {
			result.WriteString(fmt.Sprintf("✗ %s (%.2fs)\n", test.TestName, test.Duration))
			
			// Format output (remove excessive whitespace, limit length)
			output := strings.TrimSpace(test.Output)
			if output != "" {
				lines := strings.Split(output, "\n")
				// Show last 20 lines of output (most relevant)
				startIdx := 0
				if len(lines) > 20 {
					startIdx = len(lines) - 20
					result.WriteString("  [...output truncated...]\n")
				}
				for _, line := range lines[startIdx:] {
					if strings.TrimSpace(line) != "" {
						result.WriteString("  " + line + "\n")
					}
				}
			}
			result.WriteString("\n")
		}
	}

	return result.String(), nil
}

// RunTestsTool forces immediate test execution
type RunTestsTool struct {
	testRunner *testrunner.TestRunner
}

// GetDefinition returns the tool definition for the AI provider
func (tool *RunTestsTool) GetDefinition() ai_provider.Tool {
	return ai_provider.Tool{
		Type: "function",
		Function: ai_provider.ToolFunction{
			Name:        "run_tests",
			Description: "Force immediate test execution (bypasses automatic debouncing). Use this when you want fresh test results right now, such as after fixing a bug.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"package_filter": map[string]interface{}{
						"type":        "string",
						"description": "Optional: run tests only for specific package (e.g., './internal/services/agent'). Defaults to './...' (all packages)",
					},
				},
				"required": []string{},
			},
		},
	}
}

// Execute runs the tool
func (tool *RunTestsTool) Execute(arguments map[string]interface{}) (string, error) {
	packageFilter := ""
	if pkgArg, exists := arguments["package_filter"]; exists {
		if pkgStr, ok := pkgArg.(string); ok {
			packageFilter = pkgStr
		}
	}

	// Check if already running
	if tool.testRunner.IsRunning() {
		return "⏳ Tests are already running. Please wait for them to complete.\n", nil
	}

	// Run tests synchronously
	err := tool.testRunner.RunTests(packageFilter)
	if err != nil {
		return "", fmt.Errorf("failed to run tests: %w", err)
	}

	// Get summary
	summary := tool.testRunner.GetSummary()

	var result strings.Builder
	result.WriteString("=== Test Run Complete ===\n\n")

	if summary.FailedTests == 0 {
		result.WriteString("✓ ALL TESTS PASSED\n\n")
	} else {
		result.WriteString(fmt.Sprintf("✗ %d TEST(S) FAILED\n\n", summary.FailedTests))
	}

	result.WriteString(fmt.Sprintf("Total:    %d tests\n", summary.TotalTests))
	result.WriteString(fmt.Sprintf("Passed:   %d\n", summary.PassedTests))
	result.WriteString(fmt.Sprintf("Failed:   %d\n", summary.FailedTests))
	if summary.SkippedTests > 0 {
		result.WriteString(fmt.Sprintf("Skipped:  %d\n", summary.SkippedTests))
	}
	result.WriteString(fmt.Sprintf("Duration: %.2fs\n", summary.TotalDuration))

	if summary.FailedTests > 0 {
		result.WriteString("\nℹ Use 'get_failing_tests' to see detailed failure information.\n")
	}

	return result.String(), nil
}
