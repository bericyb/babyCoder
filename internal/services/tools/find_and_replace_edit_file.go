package tools

import (
	"fmt"
	"os"
	"strings"

	"github.com/exar/babycoder/internal/services/ai_provider"
	"github.com/exar/babycoder/internal/services/analyzer"
	"github.com/exar/babycoder/internal/services/testrunner"
)

// FindAndReplaceEditFileTool performs find and replace operations on a file
type FindAndReplaceEditFileTool struct {
	projectRoot string
	analyzer    *analyzer.Analyzer
	testRunner  *testrunner.TestRunner
}

// Execute performs find and replace on a file
func (tool *FindAndReplaceEditFileTool) Execute(arguments map[string]interface{}) (string, error) {
	filePath, err := getStringArg(arguments, "file_path")
	if err != nil {
		return "", err
	}

	findText, err := getStringArg(arguments, "find_text")
	if err != nil {
		return "", err
	}

	replaceText, err := getStringArg(arguments, "replace_text")
	if err != nil {
		return "", err
	}

	// Optional: replace all occurrences or just first
	replaceAll := getBoolArg(arguments, "replace_all", false)

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

	originalContent := string(content)

	// Check if find_text exists
	if !strings.Contains(originalContent, findText) {
		return "", fmt.Errorf("find_text not found in file: %s", findText)
	}

	// Perform replacement
	var newContent string
	var replacementCount int

	if replaceAll {
		// Replace all occurrences
		replacementCount = strings.Count(originalContent, findText)
		newContent = strings.ReplaceAll(originalContent, findText, replaceText)
	} else {
		// Replace only first occurrence
		replacementCount = 1
		newContent = strings.Replace(originalContent, findText, replaceText, 1)
	}

	// Check if anything actually changed
	if newContent == originalContent {
		return "", fmt.Errorf("replacement resulted in no changes")
	}

	// Write back to file
	if err := os.WriteFile(resolvedPath, []byte(newContent), 0644); err != nil {
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

	return fmt.Sprintf("Successfully replaced %d occurrence(s) in %s", replacementCount, filePath), nil
}

// GetDefinition returns the tool definition for the AI provider
func (tool *FindAndReplaceEditFileTool) GetDefinition() ai_provider.Tool {
	return ai_provider.Tool{
		Type: "function",
		Function: ai_provider.ToolFunction{
			Name:        "find_and_replace_edit_file",
			Description: "Find and replace text in a file. Can replace first occurrence or all occurrences. The find_text must match exactly (including whitespace).",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file to edit",
					},
					"find_text": map[string]interface{}{
						"type":        "string",
						"description": "Exact text to find (must match exactly including whitespace)",
					},
					"replace_text": map[string]interface{}{
						"type":        "string",
						"description": "Text to replace with",
					},
					"replace_all": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether to replace all occurrences (true) or just the first one (false, default)",
					},
				},
				"required": []string{"file_path", "find_text", "replace_text"},
			},
		},
	}
}
