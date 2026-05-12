package tools

import (
	"fmt"
	"os"
	"strings"

	"github.com/exar/babycoder/internal/services/ai_provider"
	"github.com/exar/babycoder/internal/services/analyzer"
)

// ReadFileTool reads the contents of a file
type ReadFileTool struct {
	projectRoot string
	analyzer    *analyzer.Analyzer
}

// Execute reads a file and returns its contents
func (tool *ReadFileTool) Execute(arguments map[string]interface{}) (string, error) {
	filePath, err := getStringArg(arguments, "file_path")
	if err != nil {
		return "", err
	}

	includeDocumentation := getBoolArg(arguments, "include_documentation", false)

	registry := &ToolRegistry{projectRoot: tool.projectRoot}
	resolvedPath, err := registry.resolvePath(filePath)
	if err != nil {
		return "", err
	}

	// Check if file exists
	if !fileExists(resolvedPath) {
		return "", fmt.Errorf("file does not exist: %s", filePath)
	}

	// Read file
	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	result := string(content)

	// If documentation requested and it's a Go file, prepend package outline
	if includeDocumentation && strings.HasSuffix(filePath, ".go") && tool.analyzer != nil {
		packageInfo := tool.analyzer.GetPackageInfo(filePath)
		if packageInfo != nil {
			var docHeader strings.Builder
			docHeader.WriteString("=== DOCUMENTATION SUMMARY ===\n\n")
			docHeader.WriteString(fmt.Sprintf("Package: %s\n", packageInfo.PackageName))
			
			if len(packageInfo.Functions) > 0 {
				docHeader.WriteString(fmt.Sprintf("\nFunctions: %d\n", len(packageInfo.Functions)))
				for _, fn := range packageInfo.Functions {
					if fn.Receiver != "" {
						docHeader.WriteString(fmt.Sprintf("  • (%s) %s - Line %d\n", fn.Receiver, fn.Name, fn.Line))
					} else {
						docHeader.WriteString(fmt.Sprintf("  • %s - Line %d\n", fn.Name, fn.Line))
					}
				}
			}

			if len(packageInfo.Structs) > 0 {
				docHeader.WriteString(fmt.Sprintf("\nStructs: %d\n", len(packageInfo.Structs)))
				for _, st := range packageInfo.Structs {
					docHeader.WriteString(fmt.Sprintf("  • %s (%d fields) - Line %d\n", st.Name, len(st.Fields), st.Line))
				}
			}

			if len(packageInfo.Interfaces) > 0 {
				docHeader.WriteString(fmt.Sprintf("\nInterfaces: %d\n", len(packageInfo.Interfaces)))
				for _, iface := range packageInfo.Interfaces {
					docHeader.WriteString(fmt.Sprintf("  • %s (%d methods) - Line %d\n", iface.Name, len(iface.Methods), iface.Line))
				}
			}

			docHeader.WriteString("\n=== FILE CONTENTS ===\n\n")
			result = docHeader.String() + result
		}
	}

	return result, nil
}

// GetDefinition returns the tool definition for the AI provider
func (tool *ReadFileTool) GetDefinition() ai_provider.Tool {
	return ai_provider.Tool{
		Type: "function",
		Function: ai_provider.ToolFunction{
			Name:        "read_file",
			Description: "Read the contents of a file. For Go files, optionally include a documentation summary showing the package structure.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file to read (relative to project root or absolute)",
					},
					"include_documentation": map[string]interface{}{
						"type":        "boolean",
						"description": "For Go files, prepend a documentation summary with functions, structs, and interfaces (default: false)",
					},
				},
				"required": []string{"file_path"},
			},
		},
	}
}
