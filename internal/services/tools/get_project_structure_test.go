package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetProjectStructureTool(t *testing.T) {
	tempDir := t.TempDir()

	// Create a mixed-language project structure to prove the tool is
	// language-agnostic.
	structure := map[string]string{
		"README.md":                   "# project\n",
		"src/main.py":                 "print('hello')\n",
		"src/utils/helper.js":         "module.exports = {}\n",
		"lib/models/user.rb":          "class User; end\n",
		"node_modules/dep/index.js":   "// should be skipped\n",
		".git/HEAD":                   "ref: refs/heads/main\n",
	}

	for filePath, content := range structure {
		fullPath := filepath.Join(tempDir, filePath)
		if mkdirError := os.MkdirAll(filepath.Dir(fullPath), 0755); mkdirError != nil {
			t.Fatalf("Failed to create directory for %s: %v", filePath, mkdirError)
		}
		if writeError := os.WriteFile(fullPath, []byte(content), 0644); writeError != nil {
			t.Fatalf("Failed to create test file %s: %v", filePath, writeError)
		}
	}

	tool := &GetProjectStructureTool{ProjectRoot: tempDir}
	result, executeError := tool.Execute(map[string]any{})
	if executeError != nil {
		t.Fatalf("Execute failed: %v", executeError)
	}

	// Files that should appear.
	expectedSubstrings := []string{"README.md", "main.py", "helper.js", "user.rb", "src/", "lib/"}
	for _, expected := range expectedSubstrings {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected output to contain %q\n---\n%s", expected, result)
		}
	}

	// Skipped directories must not appear.
	skippedSubstrings := []string{"node_modules", ".git"}
	for _, skipped := range skippedSubstrings {
		if strings.Contains(result, skipped) {
			t.Errorf("Expected output to skip %q\n---\n%s", skipped, result)
		}
	}
}

func TestGetProjectStructureToolEmpty(t *testing.T) {
	tempDir := t.TempDir()
	tool := &GetProjectStructureTool{ProjectRoot: tempDir}

	result, executeError := tool.Execute(map[string]any{})
	if executeError != nil {
		t.Fatalf("Execute failed: %v", executeError)
	}
	if !strings.Contains(result, "empty") {
		t.Errorf("Expected empty-project message, got:\n%s", result)
	}
}
