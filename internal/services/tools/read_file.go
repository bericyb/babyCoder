package tools

import (
	"fmt"
	"os"
	"strings"

	"github.com/exar/babycoder/internal/services/ai_provider"
)

// ReadFileTool reads the contents of a file.
type ReadFileTool struct {
	projectRoot string
}

// Execute reads a file and returns its contents.
func (tool *ReadFileTool) Execute(arguments map[string]any) (string, error) {
	filePath, getError := getStringArg(arguments, "file_path")
	if getError != nil {
		return "", getError
	}

	registry := &ToolRegistry{projectRoot: tool.projectRoot}
	resolvedPath, resolveError := registry.resolvePath(filePath)
	if resolveError != nil {
		return "", resolveError
	}

	if !fileExists(resolvedPath) {
		return "", fmt.Errorf("file does not exist: %s", filePath)
	}

	content, readError := os.ReadFile(resolvedPath)
	if readError != nil {
		return "", fmt.Errorf("failed to read file: %w", readError)
	}

	return addLineNumbers(string(content)), nil
}

// addLineNumbers prefixes each line of the given content with its 1-indexed
// line number, formatted as "<line>: <content>". A trailing newline in the
// original content is preserved without producing an extra numbered empty
// line.
func addLineNumbers(content string) string {
	if content == "" {
		return ""
	}

	hasTrailingNewline := strings.HasSuffix(content, "\n")
	trimmed := content
	if hasTrailingNewline {
		trimmed = strings.TrimSuffix(content, "\n")
	}

	lines := strings.Split(trimmed, "\n")
	numberedLines := make([]string, len(lines))
	for index, line := range lines {
		numberedLines[index] = fmt.Sprintf("%d: %s", index+1, line)
	}

	result := strings.Join(numberedLines, "\n")
	if hasTrailingNewline {
		result += "\n"
	}
	return result
}

// GetDefinition returns the tool definition for the AI provider.
func (tool *ReadFileTool) GetDefinition() ai_provider.Tool {
	return ai_provider.Tool{
		Type: "function",
		Function: ai_provider.ToolFunction{
			Name:        "read_file",
			Description: "Read the contents of a file in the project. Each line is prefixed with its 1-indexed line number in the format '<line>: <content>'. The line number prefix is not part of the file content.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{
						"type":        "string",
						"description": "Path to the file to read (relative to project root or absolute)",
					},
				},
				"required": []string{"file_path"},
			},
		},
	}
}
