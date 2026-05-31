package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/exar/babycoder/internal/services/ai_provider"
)

// ListFilesTool lists files in a directory with optional glob patterns
type ListFilesTool struct {
	projectRoot string
}

// Execute lists files matching the pattern
func (tool *ListFilesTool) Execute(arguments map[string]any) (string, error) {
	// Get directory path (default to project root)
	dirPath := "."
	if dirArg, exists := arguments["directory"]; exists {
		if str, ok := dirArg.(string); ok {
			dirPath = str
		}
	}

	// Get optional pattern
	pattern := "*"
	if patternArg, exists := arguments["pattern"]; exists {
		if str, ok := patternArg.(string); ok {
			pattern = str
		}
	}

	// Get optional recursive flag
	recursive := getBoolArg(arguments, "recursive", false)

	registry := &ToolRegistry{projectRoot: tool.projectRoot}
	resolvedDir, err := registry.resolvePath(dirPath)
	if err != nil {
		return "", err
	}

	// Check if directory exists
	info, err := os.Stat(resolvedDir)
	if err != nil {
		return "", fmt.Errorf("directory does not exist: %s", dirPath)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", dirPath)
	}

	var files []string

	if recursive {
		// Recursive search
		err = filepath.WalkDir(resolvedDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Skip directories
			if d.IsDir() {
				return nil
			}

			// Get relative path
			relPath, err := filepath.Rel(tool.projectRoot, path)
			if err != nil {
				relPath = path
			}

			// Match pattern
			matched, err := filepath.Match(pattern, filepath.Base(path))
			if err != nil {
				return err
			}

			if matched {
				files = append(files, relPath)
			}

			return nil
		})

		if err != nil {
			return "", fmt.Errorf("failed to walk directory: %w", err)
		}
	} else {
		// Non-recursive search
		entries, err := os.ReadDir(resolvedDir)
		if err != nil {
			return "", fmt.Errorf("failed to read directory: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			// Match pattern
			matched, err := filepath.Match(pattern, entry.Name())
			if err != nil {
				return "", fmt.Errorf("invalid pattern: %w", err)
			}

			if matched {
				relPath := filepath.Join(dirPath, entry.Name())
				files = append(files, relPath)
			}
		}
	}

	if len(files) == 0 {
		return "No files found matching the pattern", nil
	}

	return fmt.Sprintf("Found %d files:\n%s", len(files), strings.Join(files, "\n")), nil
}

// GetDefinition returns the tool definition for the AI provider
func (tool *ListFilesTool) GetDefinition() ai_provider.Tool {
	return ai_provider.Tool{
		Type: "function",
		Function: ai_provider.ToolFunction{
			Name:        "list_files",
			Description: "List files in a directory with optional glob pattern matching. Can search recursively.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"directory": map[string]any{
						"type":        "string",
						"description": "Directory to search (relative to project root, default: '.')",
					},
					"pattern": map[string]any{
						"type":        "string",
						"description": "Glob pattern to match files (e.g., '*.py', '*.ts', 'test_*.txt', default: '*')",
					},
					"recursive": map[string]any{
						"type":        "boolean",
						"description": "Whether to search recursively in subdirectories (default: false)",
					},
				},
				"required": []string{},
			},
		},
	}
}
