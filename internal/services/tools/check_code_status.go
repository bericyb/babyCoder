package tools

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/exar/babycoder/internal/services/ai_provider"
	"github.com/exar/babycoder/internal/services/analyzer"
)

// CheckCodeStatusTool returns current code analysis status
type CheckCodeStatusTool struct {
	analyzer *analyzer.Analyzer
}

// GetDefinition returns the tool definition for the AI provider
func (tool *CheckCodeStatusTool) GetDefinition() ai_provider.Tool {
	return ai_provider.Tool{
		Type: "function",
		Function: ai_provider.ToolFunction{
			Name:        "check_code_status",
			Description: "Check current Go project code status including compile errors, warnings, and other issues. Use this to understand what problems exist in the codebase before or after making changes.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"include_warnings": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether to include warnings from go vet (default: true)",
					},
					"max_diagnostics": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of diagnostics to return per severity level (default: 20)",
					},
				},
				"required": []string{},
			},
		},
	}
}

// Execute runs the tool
func (tool *CheckCodeStatusTool) Execute(arguments map[string]interface{}) (string, error) {
	// Parse arguments
	includeWarnings := true
	if includeWarningsArg, exists := arguments["include_warnings"]; exists {
		if boolValue, ok := includeWarningsArg.(bool); ok {
			includeWarnings = boolValue
		}
	}

	maxDiagnostics := 20
	if maxDiagnosticsArg, exists := arguments["max_diagnostics"]; exists {
		if floatValue, ok := maxDiagnosticsArg.(float64); ok {
			maxDiagnostics = int(floatValue)
		}
	}

	// Trigger analysis if not currently running
	// This ensures fresh results
	status := tool.analyzer.GetStatus()
	if isAnalyzing, ok := status["is_analyzing"].(bool); !isAnalyzing || !ok {
		// Run synchronously to get immediate results
		if err := tool.analyzer.Analyze(); err != nil {
			return "", fmt.Errorf("failed to analyze project: %w", err)
		}
	}

	// Get all diagnostics
	allDiagnostics := tool.analyzer.GetDiagnostics()

	// Separate by severity
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

	// Sort by file path and line number
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

	// Limit results
	if len(errors) > maxDiagnostics {
		errors = errors[:maxDiagnostics]
	}
	if len(warnings) > maxDiagnostics {
		warnings = warnings[:maxDiagnostics]
	}
	if len(others) > maxDiagnostics {
		others = others[:maxDiagnostics]
	}

	// Build response
	var result strings.Builder
	result.WriteString("=== Go Project Code Status ===\n\n")

	if len(errors) == 0 && len(warnings) == 0 && len(others) == 0 {
		result.WriteString("✓ No issues found. Project compiles successfully.\n")
		return result.String(), nil
	}

	// Summary
	result.WriteString(fmt.Sprintf("Summary: %d error(s), %d warning(s), %d other issue(s)\n\n",
		len(errors), len(warnings), len(others)))

	// Errors section
	if len(errors) > 0 {
		result.WriteString("=== ERRORS ===\n")
		for _, diagnostic := range errors {
			result.WriteString(fmt.Sprintf("\n%s:%d:%d\n", diagnostic.FilePath, diagnostic.Line, diagnostic.Column))
			result.WriteString(fmt.Sprintf("  [%s] %s\n", diagnostic.Source, diagnostic.Message))
		}
		result.WriteString("\n")
	}

	// Warnings section
	if len(warnings) > 0 {
		result.WriteString("=== WARNINGS ===\n")
		for _, diagnostic := range warnings {
			result.WriteString(fmt.Sprintf("\n%s:%d:%d\n", diagnostic.FilePath, diagnostic.Line, diagnostic.Column))
			result.WriteString(fmt.Sprintf("  [%s] %s\n", diagnostic.Source, diagnostic.Message))
		}
		result.WriteString("\n")
	}

	// Other issues
	if len(others) > 0 {
		result.WriteString("=== OTHER ISSUES ===\n")
		for _, diagnostic := range others {
			result.WriteString(fmt.Sprintf("\n%s:%d:%d\n", diagnostic.FilePath, diagnostic.Line, diagnostic.Column))
			result.WriteString(fmt.Sprintf("  [%s] %s\n", diagnostic.Source, diagnostic.Message))
		}
		result.WriteString("\n")
	}

	return result.String(), nil
}

// GetFileDiagnosticsTool returns diagnostics for a specific file
type GetFileDiagnosticsTool struct {
	analyzer *analyzer.Analyzer
}

// GetDefinition returns the tool definition for the AI provider
func (tool *GetFileDiagnosticsTool) GetDefinition() ai_provider.Tool {
	return ai_provider.Tool{
		Type: "function",
		Function: ai_provider.ToolFunction{
			Name:        "get_file_diagnostics",
			Description: "Get detailed diagnostics (errors, warnings, issues) for a specific Go file. Use this to see all problems in a particular file you're working on.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "Relative path to the Go file (e.g., 'internal/services/agent/agent.go')",
					},
				},
				"required": []string{"file_path"},
			},
		},
	}
}

// Execute runs the tool
func (tool *GetFileDiagnosticsTool) Execute(arguments map[string]interface{}) (string, error) {
	// Get file path
	filePath, exists := arguments["file_path"]
	if !exists {
		return "", fmt.Errorf("file_path is required")
	}

	filePathStr, ok := filePath.(string)
	if !ok {
		return "", fmt.Errorf("file_path must be a string")
	}

	// Get diagnostics for this file
	diagnostics := tool.analyzer.GetFileDiagnostics(filePathStr)

	if len(diagnostics) == 0 {
		return fmt.Sprintf("✓ No issues found in %s\n", filePathStr), nil
	}

	// Sort by line number
	sort.Slice(diagnostics, func(i, j int) bool {
		return diagnostics[i].Line < diagnostics[j].Line
	})

	// Build response
	var result strings.Builder
	result.WriteString(fmt.Sprintf("=== Diagnostics for %s ===\n\n", filePathStr))
	result.WriteString(fmt.Sprintf("Total issues: %d\n\n", len(diagnostics)))

	for _, diagnostic := range diagnostics {
		severitySymbol := "•"
		if diagnostic.Severity == "error" {
			severitySymbol = "✗"
		} else if diagnostic.Severity == "warning" {
			severitySymbol = "⚠"
		}

		result.WriteString(fmt.Sprintf("%s Line %d, Column %d [%s]\n",
			severitySymbol, diagnostic.Line, diagnostic.Column, diagnostic.Severity))
		result.WriteString(fmt.Sprintf("  Source: %s\n", diagnostic.Source))
		result.WriteString(fmt.Sprintf("  %s\n\n", diagnostic.Message))
	}

	return result.String(), nil
}

// GetPackageOutlineTool returns package structure information
type GetPackageOutlineTool struct {
	analyzer *analyzer.Analyzer
}

// GetDefinition returns the tool definition for the AI provider
func (tool *GetPackageOutlineTool) GetDefinition() ai_provider.Tool {
	return ai_provider.Tool{
		Type: "function",
		Function: ai_provider.ToolFunction{
			Name:        "get_package_outline",
			Description: "Get the structure outline of a Go file or package including functions, methods, structs, interfaces, and imports. Use this to understand the code organization.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "Relative path to the Go file (e.g., 'internal/services/agent/agent.go')",
					},
				},
				"required": []string{"file_path"},
			},
		},
	}
}

// Execute runs the tool
func (tool *GetPackageOutlineTool) Execute(arguments map[string]interface{}) (string, error) {
	// Get file path
	filePath, exists := arguments["file_path"]
	if !exists {
		return "", fmt.Errorf("file_path is required")
	}

	filePathStr, ok := filePath.(string)
	if !ok {
		return "", fmt.Errorf("file_path must be a string")
	}

	// Get package info
	packageInfo := tool.analyzer.GetPackageInfo(filePathStr)
	if packageInfo == nil {
		return "", fmt.Errorf("no package information found for %s (file may not have been analyzed yet)", filePathStr)
	}

	// Build response
	var result strings.Builder
	result.WriteString(fmt.Sprintf("=== Package Outline: %s ===\n\n", packageInfo.PackageName))
	result.WriteString(fmt.Sprintf("File: %s\n\n", packageInfo.FilePath))

	// Imports
	if len(packageInfo.Imports) > 0 {
		result.WriteString("--- IMPORTS ---\n")
		for _, importPath := range packageInfo.Imports {
			result.WriteString(fmt.Sprintf("  %s\n", importPath))
		}
		result.WriteString("\n")
	}

	// Structs
	if len(packageInfo.Structs) > 0 {
		result.WriteString("--- STRUCTS ---\n")
		for _, structDef := range packageInfo.Structs {
			result.WriteString(fmt.Sprintf("\ntype %s struct (Line %d)\n", structDef.Name, structDef.Line))
			for _, field := range structDef.Fields {
				result.WriteString(fmt.Sprintf("  %s\n", field))
			}
		}
		result.WriteString("\n")
	}

	// Interfaces
	if len(packageInfo.Interfaces) > 0 {
		result.WriteString("--- INTERFACES ---\n")
		for _, interfaceDef := range packageInfo.Interfaces {
			result.WriteString(fmt.Sprintf("\ntype %s interface (Line %d)\n", interfaceDef.Name, interfaceDef.Line))
			for _, method := range interfaceDef.Methods {
				result.WriteString(fmt.Sprintf("  %s\n", method))
			}
		}
		result.WriteString("\n")
	}

	// Functions and Methods
	if len(packageInfo.Functions) > 0 {
		result.WriteString("--- FUNCTIONS & METHODS ---\n")
		for _, funcSig := range packageInfo.Functions {
			if funcSig.Receiver != "" {
				// Method
				result.WriteString(fmt.Sprintf("\nfunc (%s) %s(%s)", funcSig.Receiver, funcSig.Name, funcSig.Parameters))
			} else {
				// Function
				result.WriteString(fmt.Sprintf("\nfunc %s(%s)", funcSig.Name, funcSig.Parameters))
			}

			if funcSig.Results != "" {
				result.WriteString(fmt.Sprintf(" (%s)", funcSig.Results))
			}

			result.WriteString(fmt.Sprintf(" // Line %d\n", funcSig.Line))
		}
		result.WriteString("\n")
	}

	// Convert to JSON for structured output option
	jsonData, err := json.MarshalIndent(packageInfo, "", "  ")
	if err == nil {
		result.WriteString("\n--- JSON REPRESENTATION ---\n")
		result.WriteString(string(jsonData))
		result.WriteString("\n")
	}

	return result.String(), nil
}
