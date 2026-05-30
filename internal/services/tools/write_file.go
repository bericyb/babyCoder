package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/exar/babycoder/internal/services/ai_provider"
	"github.com/exar/babycoder/internal/services/analyzer"
	"github.com/exar/babycoder/internal/services/testrunner"
)

// WriteFileTool writes content to a file
type WriteFileTool struct {
	projectRoot string
	analyzer    *analyzer.Analyzer
	testRunner  *testrunner.TestRunner
}

// Execute writes content to a file
func (tool *WriteFileTool) Execute(arguments map[string]interface{}) (string, error) {
	filePath, err := getStringArg(arguments, "file_path")
	if err != nil {
		return "", err
	}

	content, err := getStringArg(arguments, "content")
	if err != nil {
		return "", err
	}

	createDirs := getBoolArg(arguments, "create_directories", true)

	registry := &ToolRegistry{projectRoot: tool.projectRoot}
	resolvedPath, err := registry.resolvePath(filePath)
	if err != nil {
		return "", err
	}

	// Create parent directories if requested
	if createDirs {
		dir := filepath.Dir(resolvedPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create directories: %w", err)
		}
	}

	// Write file
	if err := os.WriteFile(resolvedPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Trigger background analysis and checks if this is a Go file
	if strings.HasSuffix(filePath, ".go") {
		if tool.analyzer != nil {
			tool.analyzer.AnalyzeAsync()
		}
		if tool.testRunner != nil {
			tool.testRunner.MarkDirty() // Mark that tests need to run
		}
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), filePath), nil
}

// GetDefinition returns the tool definition for the AI provider
func (tool *WriteFileTool) GetDefinition() ai_provider.Tool {
	return ai_provider.Tool{
		Type: "function",
		Function: ai_provider.ToolFunction{
			Name:        "write_file",
			Description: "Write content to a file. Creates the file if it doesn't exist, overwrites if it does.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file to write (relative to project root or absolute)",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Content to write to the file",
					},
					"create_directories": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether to create parent directories if they don't exist (default: true)",
					},
				},
				"required": []string{"file_path", "content"},
			},
		},
	}
}
