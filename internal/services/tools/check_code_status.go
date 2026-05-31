package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/exar/babycoder/internal/services/ai_provider"
	"github.com/exar/babycoder/internal/services/analyzer"
)

// CheckCodeStatusTool runs a user-supplied build/lint command and returns
// a structured pass/fail summary plus diagnostics, language-agnostic.
type CheckCodeStatusTool struct {
	analyzer *analyzer.Analyzer
}

// GetDefinition returns the tool definition for the AI provider.
func (tool *CheckCodeStatusTool) GetDefinition() ai_provider.Tool {
	return ai_provider.Tool{
		Type: "function",
		Function: ai_provider.ToolFunction{
			Name: "check_code_status",
			Description: "Run a build, compile, or lint command for the current project and return a structured pass/fail status along with extracted diagnostics. " +
				"You MUST provide the exact shell command to run (e.g. 'cargo check', 'npm run build', 'tsc --noEmit', 'pytest --collect-only'). " +
				"Once supplied, the command is remembered and re-run automatically in the background after file edits.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"build_command": map[string]any{
						"type":        "string",
						"description": "Shell command to execute from the project root. If omitted, re-uses the last command supplied (errors if none).",
					},
					"include_warnings": map[string]any{
						"type":        "boolean",
						"description": "Whether to include warnings in the output (default: true)",
					},
					"max_diagnostics": map[string]any{
						"type":        "integer",
						"description": "Maximum number of diagnostics to return per severity level (default: 20)",
					},
				},
				"required": []string{},
			},
		},
	}
}

// Execute runs the tool.
func (tool *CheckCodeStatusTool) Execute(arguments map[string]any) (string, error) {
	includeWarnings := getBoolArg(arguments, "include_warnings", true)
	maximumDiagnostics := getIntArgDefault(arguments, "max_diagnostics", 20)

	buildCommand := getStringArgDefault(arguments, "build_command", "")
	if buildCommand == "" {
		buildCommand = tool.analyzer.LastCommand()
	}
	if buildCommand == "" {
		return "", fmt.Errorf("build_command is required on the first call (no previous command remembered)")
	}

	if analysisError := tool.analyzer.Analyze(context.Background(), buildCommand); analysisError != nil {
		return "", fmt.Errorf("failed to analyze project: %w", analysisError)
	}

	allDiagnostics := tool.analyzer.GetDiagnostics()
	status := tool.analyzer.GetStatus()

	var errors []analyzer.Diagnostic
	var warnings []analyzer.Diagnostic
	var others []analyzer.Diagnostic

	for _, diagnostic := range allDiagnostics {
		switch diagnostic.Severity {
		case "error":
			errors = append(errors, diagnostic)
		case "warning":
			if includeWarnings {
				warnings = append(warnings, diagnostic)
			}
		default:
			others = append(others, diagnostic)
		}
	}

	sortDiagnostics := func(diagnostics []analyzer.Diagnostic) {
		sort.Slice(diagnostics, func(i, j int) bool {
			if diagnostics[i].FilePath != diagnostics[j].FilePath {
				return diagnostics[i].FilePath < diagnostics[j].FilePath
			}
			return diagnostics[i].Line < diagnostics[j].Line
		})
	}
	sortDiagnostics(errors)
	sortDiagnostics(warnings)
	sortDiagnostics(others)

	if len(errors) > maximumDiagnostics {
		errors = errors[:maximumDiagnostics]
	}
	if len(warnings) > maximumDiagnostics {
		warnings = warnings[:maximumDiagnostics]
	}
	if len(others) > maximumDiagnostics {
		others = others[:maximumDiagnostics]
	}

	var output strings.Builder
	output.WriteString("=== Code Status ===\n\n")
	output.WriteString(fmt.Sprintf("Command: %s\n", buildCommand))
	if lastStatus, ok := status["last_status"].(string); ok {
		output.WriteString(fmt.Sprintf("Status:  %s\n", strings.ToUpper(lastStatus)))
	}
	if lastSummary, ok := status["last_summary"].(string); ok && lastSummary != "" {
		output.WriteString(fmt.Sprintf("Summary: %s\n", lastSummary))
	}
	output.WriteString("\n")

	if len(errors) == 0 && len(warnings) == 0 && len(others) == 0 {
		output.WriteString("No issues extracted.\n")
		return output.String(), nil
	}

	output.WriteString(fmt.Sprintf("Issue counts: %d error(s), %d warning(s), %d other\n\n",
		len(errors), len(warnings), len(others)))

	writeSection := func(title string, diagnostics []analyzer.Diagnostic) {
		if len(diagnostics) == 0 {
			return
		}
		output.WriteString(fmt.Sprintf("=== %s ===\n", title))
		for _, diagnostic := range diagnostics {
			output.WriteString(fmt.Sprintf("\n%s:%d:%d\n", diagnostic.FilePath, diagnostic.Line, diagnostic.Column))
			output.WriteString(fmt.Sprintf("  [%s] %s\n", diagnostic.Source, diagnostic.Message))
		}
		output.WriteString("\n")
	}
	writeSection("ERRORS", errors)
	writeSection("WARNINGS", warnings)
	writeSection("OTHER ISSUES", others)

	return output.String(), nil
}

// GetFileDiagnosticsTool returns diagnostics for a specific file.
type GetFileDiagnosticsTool struct {
	analyzer *analyzer.Analyzer
}

// GetDefinition returns the tool definition for the AI provider.
func (tool *GetFileDiagnosticsTool) GetDefinition() ai_provider.Tool {
	return ai_provider.Tool{
		Type: "function",
		Function: ai_provider.ToolFunction{
			Name:        "get_file_diagnostics",
			Description: "Get diagnostics (errors, warnings, issues) extracted by the most recent check_code_status run for a specific file.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{
						"type":        "string",
						"description": "Path to the file, relative to project root",
					},
				},
				"required": []string{"file_path"},
			},
		},
	}
}

// Execute runs the tool.
func (tool *GetFileDiagnosticsTool) Execute(arguments map[string]any) (string, error) {
	filePath, getError := getStringArg(arguments, "file_path")
	if getError != nil {
		return "", getError
	}

	diagnostics := tool.analyzer.GetFileDiagnostics(filePath)
	if len(diagnostics) == 0 {
		return fmt.Sprintf("No issues recorded for %s (run check_code_status first if you have not).\n", filePath), nil
	}

	sort.Slice(diagnostics, func(i, j int) bool {
		return diagnostics[i].Line < diagnostics[j].Line
	})

	var output strings.Builder
	output.WriteString(fmt.Sprintf("=== Diagnostics for %s ===\n\n", filePath))
	output.WriteString(fmt.Sprintf("Total issues: %d\n\n", len(diagnostics)))

	for _, diagnostic := range diagnostics {
		severitySymbol := "-"
		switch diagnostic.Severity {
		case "error":
			severitySymbol = "x"
		case "warning":
			severitySymbol = "!"
		}
		output.WriteString(fmt.Sprintf("%s Line %d, Column %d [%s]\n",
			severitySymbol, diagnostic.Line, diagnostic.Column, diagnostic.Severity))
		output.WriteString(fmt.Sprintf("  Source: %s\n", diagnostic.Source))
		output.WriteString(fmt.Sprintf("  %s\n\n", diagnostic.Message))
	}

	return output.String(), nil
}
