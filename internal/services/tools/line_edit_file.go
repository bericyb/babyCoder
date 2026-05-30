package tools

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/exar/babycoder/internal/services/ai_provider"
	"github.com/exar/babycoder/internal/services/analyzer"
	"github.com/exar/babycoder/internal/services/testrunner"
)

// LineEditFileTool edits specific lines in a file
type LineEditFileTool struct {
	projectRoot string
	analyzer    *analyzer.Analyzer
	testRunner  *testrunner.TestRunner
}

// Execute performs line-based editing on a file
func (tool *LineEditFileTool) Execute(arguments map[string]interface{}) (string, error) {
	filePath, err := getStringArg(arguments, "file_path")
	if err != nil {
		return "", err
	}

	startLine, err := getIntArg(arguments, "start_line")
	if err != nil {
		return "", err
	}

	endLine, err := getIntArg(arguments, "end_line")
	if err != nil {
		return "", err
	}

	newContent, err := getStringArg(arguments, "new_content")
	if err != nil {
		return "", err
	}

	// Validate line numbers
	if startLine < 1 {
		return "", fmt.Errorf("start_line must be >= 1")
	}
	if endLine < startLine {
		return "", fmt.Errorf("end_line must be >= start_line")
	}

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
	file, err := os.Open(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Read all lines
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Validate line numbers against file length
	if startLine > len(lines) {
		return "", fmt.Errorf("start_line %d exceeds file length %d", startLine, len(lines))
	}
	if endLine > len(lines) {
		return "", fmt.Errorf("end_line %d exceeds file length %d", endLine, len(lines))
	}

	// Build new content
	var result []string
	
	// Lines before the edit
	result = append(result, lines[:startLine-1]...)
	
	// New content (split by newlines if it contains multiple lines)
	newLines := strings.Split(newContent, "\n")
	result = append(result, newLines...)
	
	// Lines after the edit
	if endLine < len(lines) {
		result = append(result, lines[endLine:]...)
	}

	// Write back to file
	content := strings.Join(result, "\n")
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

	linesReplaced := endLine - startLine + 1
	linesAdded := len(newLines)
	
	return fmt.Sprintf("Successfully edited %s: replaced %d lines (lines %d-%d) with %d lines", 
		filePath, linesReplaced, startLine, endLine, linesAdded), nil
}

// GetDefinition returns the tool definition for the AI provider
func (tool *LineEditFileTool) GetDefinition() ai_provider.Tool {
	return ai_provider.Tool{
		Type: "function",
		Function: ai_provider.ToolFunction{
			Name:        "line_edit_file",
			Description: "Edit specific lines in a file. Replaces lines from start_line to end_line (inclusive, 1-indexed) with new_content.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file to edit",
					},
					"start_line": map[string]interface{}{
						"type":        "number",
						"description": "Starting line number (1-indexed, inclusive)",
					},
					"end_line": map[string]interface{}{
						"type":        "number",
						"description": "Ending line number (1-indexed, inclusive)",
					},
					"new_content": map[string]interface{}{
						"type":        "string",
						"description": "New content to replace the specified lines. Can contain newlines for multi-line replacements.",
					},
				},
				"required": []string{"file_path", "start_line", "end_line", "new_content"},
			},
		},
	}
}
