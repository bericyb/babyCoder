package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/exar/babycoder/internal/services/ai_provider"
)

// GetProjectStructureTool walks the project directory and returns a
// language-agnostic file tree. It does no AST parsing and has no language
// preference.
type GetProjectStructureTool struct {
	ProjectRoot string // Made public for testing.
}

// defaultSkipDirectories names directories that almost never contain useful
// source code and are safe to skip across most ecosystems.
var defaultSkipDirectories = map[string]bool{
	".git":         true,
	".hg":          true,
	".svn":         true,
	"node_modules": true,
	"vendor":       true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
	"env":          true,
	"target":       true,
	"dist":         true,
	"build":        true,
	"out":          true,
	".next":        true,
	".nuxt":        true,
	".cache":       true,
	".idea":        true,
	".vscode":      true,
	"coverage":     true,
	".gradle":      true,
	".mvn":         true,
	"bin":          true,
	"obj":          true,
}

// treeEntry represents a single file or directory in the walk.
type treeEntry struct {
	relativePath string
	isDirectory  bool
	sizeBytes    int64
	depth        int
}

// Execute walks the project and returns a formatted file tree.
func (tool *GetProjectStructureTool) Execute(arguments map[string]any) (string, error) {
	maximumDepth := getIntArgDefault(arguments, "max_depth", 5)
	includeHidden := getBoolArg(arguments, "include_hidden", false)
	maximumEntries := getIntArgDefault(arguments, "max_entries", 500)

	entries, walkError := tool.walk(maximumDepth, includeHidden, maximumEntries)
	if walkError != nil {
		return "", fmt.Errorf("failed to walk project: %w", walkError)
	}

	if len(entries) == 0 {
		return "Project directory is empty (or all entries were skipped).", nil
	}

	totalFiles := 0
	totalDirectories := 0
	var totalSizeBytes int64
	for _, entry := range entries {
		if entry.isDirectory {
			totalDirectories++
		} else {
			totalFiles++
			totalSizeBytes += entry.sizeBytes
		}
	}

	var output strings.Builder
	output.WriteString("# Project Structure\n\n")
	output.WriteString(fmt.Sprintf("**Project Root:** %s\n", tool.ProjectRoot))
	output.WriteString(fmt.Sprintf("**Files:** %d  |  **Directories:** %d  |  **Total Size:** %s\n",
		totalFiles, totalDirectories, formatByteSize(totalSizeBytes)))
	if len(entries) >= maximumEntries {
		output.WriteString(fmt.Sprintf("\n_Note: output capped at %d entries; increase `max_entries` to see more._\n", maximumEntries))
	}
	output.WriteString("\n```\n")
	for _, entry := range entries {
		indent := strings.Repeat("  ", entry.depth)
		name := filepath.Base(entry.relativePath)
		if entry.isDirectory {
			output.WriteString(fmt.Sprintf("%s%s/\n", indent, name))
		} else {
			output.WriteString(fmt.Sprintf("%s%s  (%s)\n", indent, name, formatByteSize(entry.sizeBytes)))
		}
	}
	output.WriteString("```\n")

	return output.String(), nil
}

// walk traverses the project directory, applying skip rules and depth/entry
// caps. Entries are returned in a stable directory-first, alphabetical order
// suitable for tree-style rendering.
func (tool *GetProjectStructureTool) walk(maximumDepth int, includeHidden bool, maximumEntries int) ([]treeEntry, error) {
	var entries []treeEntry

	var walkDirectory func(absolutePath string, depth int) error
	walkDirectory = func(absolutePath string, depth int) error {
		if len(entries) >= maximumEntries {
			return nil
		}
		if depth > maximumDepth {
			return nil
		}

		directoryEntries, readError := os.ReadDir(absolutePath)
		if readError != nil {
			return nil // Permission denied or unreadable — skip silently.
		}

		// Sort: directories first, then files; both alphabetical.
		sort.Slice(directoryEntries, func(i, j int) bool {
			leftIsDirectory := directoryEntries[i].IsDir()
			rightIsDirectory := directoryEntries[j].IsDir()
			if leftIsDirectory != rightIsDirectory {
				return leftIsDirectory
			}
			return directoryEntries[i].Name() < directoryEntries[j].Name()
		})

		for _, directoryEntry := range directoryEntries {
			if len(entries) >= maximumEntries {
				return nil
			}

			name := directoryEntry.Name()
			if !includeHidden && strings.HasPrefix(name, ".") {
				continue
			}
			if directoryEntry.IsDir() && defaultSkipDirectories[name] {
				continue
			}

			childAbsolutePath := filepath.Join(absolutePath, name)
			relativePath, relativeError := filepath.Rel(tool.ProjectRoot, childAbsolutePath)
			if relativeError != nil {
				relativePath = childAbsolutePath
			}

			if directoryEntry.IsDir() {
				entries = append(entries, treeEntry{
					relativePath: relativePath,
					isDirectory:  true,
					depth:        depth,
				})
				if walkChildrenError := walkDirectory(childAbsolutePath, depth+1); walkChildrenError != nil {
					return walkChildrenError
				}
			} else {
				fileInformation, statError := directoryEntry.Info()
				sizeBytes := int64(0)
				if statError == nil {
					sizeBytes = fileInformation.Size()
				}
				entries = append(entries, treeEntry{
					relativePath: relativePath,
					isDirectory:  false,
					sizeBytes:    sizeBytes,
					depth:        depth,
				})
			}
		}
		return nil
	}

	if walkError := walkDirectory(tool.ProjectRoot, 0); walkError != nil {
		return nil, walkError
	}
	return entries, nil
}

// formatByteSize returns a human-readable size string.
func formatByteSize(sizeBytes int64) string {
	const unit = 1024
	if sizeBytes < unit {
		return fmt.Sprintf("%d B", sizeBytes)
	}
	divisor := int64(unit)
	exponentLabel := "KMGTPE"
	exponentIndex := 0
	for value := sizeBytes / unit; value >= unit; value /= unit {
		divisor *= unit
		exponentIndex++
		if exponentIndex >= len(exponentLabel)-1 {
			break
		}
	}
	return fmt.Sprintf("%.1f %ciB", float64(sizeBytes)/float64(divisor), exponentLabel[exponentIndex])
}

// GetDefinition returns the tool definition.
func (tool *GetProjectStructureTool) GetDefinition() ai_provider.Tool {
	return ai_provider.Tool{
		Type: "function",
		Function: ai_provider.ToolFunction{
			Name:        "get_project_structure",
			Description: "Display the project's file and directory tree. Skips common noise directories (node_modules, .git, vendor, target, dist, build, __pycache__, virtual envs, IDE folders, etc.). Language-agnostic.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"max_depth": map[string]any{
						"type":        "number",
						"description": "Maximum directory depth to descend (default: 5)",
					},
					"include_hidden": map[string]any{
						"type":        "boolean",
						"description": "Include dotfiles and dotted directories (default: false)",
					},
					"max_entries": map[string]any{
						"type":        "number",
						"description": "Maximum number of entries to return (default: 500)",
					},
				},
			},
		},
	}
}
