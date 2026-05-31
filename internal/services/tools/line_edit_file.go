package tools

import (
	"errors"
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
	hashTracker *FileHashTracker
}

// Execute performs line-based editing on a file
func (tool *LineEditFileTool) Execute(arguments map[string]any) (string, error) {
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

	// Read the file and verify its hash against the most recent read_file
	// in this session via the shared pipeline. The tracker returns the raw
	// bytes alongside the verification result so we do not read twice.
	fileBytes, verifyError := tool.hashTracker.VerifyOnDiskForEdit(resolvedPath)
	if verifyError != nil {
		if errors.Is(verifyError, ErrFileNotRead) || errors.Is(verifyError, ErrFileChangedSinceRead) {
			return "", fmt.Errorf(
				"file %q must be read before editing. Its current contents do not match what was last read in this session (either it was modified by a previous edit, or it changed on disk). Call read_file on it before retrying. (underlying: %v)",
				filePath, verifyError,
			)
		}
		return "", fmt.Errorf("failed to read file: %w", verifyError)
	}

	// Split into lines. We strip a single trailing newline before splitting
	// so the resulting slice matches what bufio.Scanner used to produce
	// (lines without their terminator, no spurious empty final element).
	fileContent := strings.TrimSuffix(string(fileBytes), "\n")

	var lines []string
	if fileContent != "" {
		lines = strings.Split(fileContent, "\n")
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

	// Trigger background analysis and test run after any file edit. Both
	// are no-ops if the agent has not yet supplied a build/test command.
	if tool.analyzer != nil {
		tool.analyzer.AnalyzeAsync()
	}
	if tool.testRunner != nil {
		tool.testRunner.MarkDirty()
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
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{
						"type":        "string",
						"description": "Path to the file to edit",
					},
					"start_line": map[string]any{
						"type":        "number",
						"description": "Starting line number (1-indexed, inclusive)",
					},
					"end_line": map[string]any{
						"type":        "number",
						"description": "Ending line number (1-indexed, inclusive)",
					},
					"new_content": map[string]any{
						"type":        "string",
						"description": "New content to replace the specified lines. Can contain newlines for multi-line replacements.",
					},
				},
				"required": []string{"file_path", "start_line", "end_line", "new_content"},
			},
		},
	}
}
