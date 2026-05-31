package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileTool(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "Hello, World!\nThis is a test file."
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := &ReadFileTool{projectRoot: tempDir}

	// Test reading file with relative path
	args := map[string]any{
		"file_path": "test.txt",
	}

	result, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "1: Hello, World!\n2: This is a test file."
	if result != expected {
		t.Errorf("Expected content %q, got %q", expected, result)
	}
}

func TestAddLineNumbers(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "single line without newline",
			input:    "hello",
			expected: "1: hello",
		},
		{
			name:     "single line with trailing newline",
			input:    "hello\n",
			expected: "1: hello\n",
		},
		{
			name:     "multiple lines without trailing newline",
			input:    "foo\nbar\nbaz",
			expected: "1: foo\n2: bar\n3: baz",
		},
		{
			name:     "multiple lines with trailing newline",
			input:    "foo\nbar\nbaz\n",
			expected: "1: foo\n2: bar\n3: baz\n",
		},
		{
			name:     "preserves blank lines in the middle",
			input:    "foo\n\nbar",
			expected: "1: foo\n2: \n3: bar",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := addLineNumbers(testCase.input)
			if result != testCase.expected {
				t.Errorf("addLineNumbers(%q) = %q, expected %q", testCase.input, result, testCase.expected)
			}
		})
	}
}

func TestReadFileToolNonExistent(t *testing.T) {
	tempDir := t.TempDir()
	tool := &ReadFileTool{projectRoot: tempDir}

	args := map[string]any{
		"file_path": "nonexistent.txt",
	}

	_, err := tool.Execute(args)
	if err == nil {
		t.Fatal("Expected error for nonexistent file, got nil")
	}
}

func TestReadFileToolPathTraversal(t *testing.T) {
	tempDir := t.TempDir()
	tool := &ReadFileTool{projectRoot: tempDir}

	// Try to escape project root
	args := map[string]any{
		"file_path": "../../../etc/passwd",
	}

	_, err := tool.Execute(args)
	if err == nil {
		t.Fatal("Expected error for path traversal, got nil")
	}

	if !strings.Contains(err.Error(), "outside project root") {
		t.Errorf("Expected 'outside project root' error, got: %v", err)
	}
}

func TestWriteFileTool(t *testing.T) {
	tempDir := t.TempDir()
	tool := &WriteFileTool{projectRoot: tempDir}

	testContent := "New file content\nLine 2"

	args := map[string]any{
		"file_path": "output.txt",
		"content":   testContent,
	}

	result, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	if !strings.Contains(result, "Successfully wrote") {
		t.Errorf("Unexpected result: %s", result)
	}

	// Verify file was written
	written, err := os.ReadFile(filepath.Join(tempDir, "output.txt"))
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	if string(written) != testContent {
		t.Errorf("Expected content %q, got %q", testContent, string(written))
	}
}

func TestWriteFileToolCreateDirectories(t *testing.T) {
	tempDir := t.TempDir()
	tool := &WriteFileTool{projectRoot: tempDir}

	args := map[string]any{
		"file_path":          "subdir/nested/file.txt",
		"content":            "test",
		"create_directories": true,
	}

	_, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("Failed to write file with nested directories: %v", err)
	}

	// Verify file exists
	filePath := filepath.Join(tempDir, "subdir", "nested", "file.txt")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("File was not created with nested directories")
	}
}

func TestListFilesTool(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	files := []string{"file1.go", "file2.go", "file3.txt", "test.md"}
	for _, file := range files {
		path := filepath.Join(tempDir, file)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	tool := &ListFilesTool{projectRoot: tempDir}

	// Test listing all files
	args := map[string]any{
		"directory": ".",
	}

	result, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("Failed to list files: %v", err)
	}

	if !strings.Contains(result, "Found 4 files") {
		t.Errorf("Expected 4 files, got: %s", result)
	}
}

func TestListFilesToolWithPattern(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	files := []string{"file1.go", "file2.go", "file3.txt"}
	for _, file := range files {
		path := filepath.Join(tempDir, file)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	tool := &ListFilesTool{projectRoot: tempDir}

	// Test with glob pattern
	args := map[string]any{
		"directory": ".",
		"pattern":   "*.go",
	}

	result, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("Failed to list files: %v", err)
	}

	if !strings.Contains(result, "Found 2 files") {
		t.Errorf("Expected 2 .go files, got: %s", result)
	}

	if !strings.Contains(result, "file1.go") || !strings.Contains(result, "file2.go") {
		t.Errorf("Expected file1.go and file2.go in results, got: %s", result)
	}

	if strings.Contains(result, "file3.txt") {
		t.Errorf("Did not expect file3.txt in results, got: %s", result)
	}
}

func TestListFilesToolRecursive(t *testing.T) {
	tempDir := t.TempDir()

	// Create nested structure
	os.MkdirAll(filepath.Join(tempDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tempDir, "root.go"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tempDir, "subdir", "nested.go"), []byte("test"), 0644)

	tool := &ListFilesTool{projectRoot: tempDir}

	args := map[string]any{
		"directory": ".",
		"pattern":   "*.go",
		"recursive": true,
	}

	result, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("Failed to list files recursively: %v", err)
	}

	if !strings.Contains(result, "Found 2 files") {
		t.Errorf("Expected 2 files in recursive search, got: %s", result)
	}

	if !strings.Contains(result, "root.go") || !strings.Contains(result, "nested.go") {
		t.Errorf("Expected both root.go and nested.go, got: %s", result)
	}
}

func TestLineEditFileTool(t *testing.T) {
	tempDir := t.TempDir()

	// Create test file with multiple lines
	testFile := filepath.Join(tempDir, "test.go")
	content := `package main

func main() {
	println("old")
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := &LineEditFileTool{projectRoot: tempDir}

	// Edit line 4
	args := map[string]any{
		"file_path":   "test.go",
		"start_line":  4,
		"end_line":    4,
		"new_content": "\tprintln(\"new\")",
	}

	result, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("Failed to edit file: %v", err)
	}

	if !strings.Contains(result, "replaced 1 lines") {
		t.Errorf("Unexpected result: %s", result)
	}

	// Verify edit
	edited, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read edited file: %v", err)
	}

	if !strings.Contains(string(edited), "println(\"new\")") {
		t.Errorf("Expected edited content to contain 'println(\"new\")', got: %s", string(edited))
	}

	if strings.Contains(string(edited), "println(\"old\")") {
		t.Errorf("Expected old content to be replaced, but it's still there: %s", string(edited))
	}
}

func TestLineEditFileToolMultipleLines(t *testing.T) {
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.txt")
	content := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := &LineEditFileTool{projectRoot: tempDir}

	// Replace lines 2-4 with new content
	args := map[string]any{
		"file_path":   "test.txt",
		"start_line":  2,
		"end_line":    4,
		"new_content": "New Line A\nNew Line B",
	}

	_, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("Failed to edit file: %v", err)
	}

	// Verify edit
	edited, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read edited file: %v", err)
	}

	expected := "Line 1\nNew Line A\nNew Line B\nLine 5"
	if string(edited) != expected {
		t.Errorf("Expected:\n%s\n\nGot:\n%s", expected, string(edited))
	}
}

func TestFindAndReplaceEditFileTool(t *testing.T) {
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.go")
	content := `package main

func greet() {
	println("Hello")
}

func farewell() {
	println("Hello")
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := &FindAndReplaceEditFileTool{projectRoot: tempDir}

	// Replace first occurrence only
	args := map[string]any{
		"file_path":    "test.go",
		"find_text":    "println(\"Hello\")",
		"replace_text": "println(\"Goodbye\")",
		"replace_all":  false,
	}

	result, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("Failed to find and replace: %v", err)
	}

	if !strings.Contains(result, "1 occurrence") {
		t.Errorf("Expected 1 occurrence replaced, got: %s", result)
	}

	// Verify only first was replaced
	edited, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read edited file: %v", err)
	}

	editedStr := string(edited)
	if strings.Count(editedStr, "println(\"Goodbye\")") != 1 {
		t.Errorf("Expected exactly 1 'Goodbye', got: %s", editedStr)
	}

	if strings.Count(editedStr, "println(\"Hello\")") != 1 {
		t.Errorf("Expected exactly 1 'Hello' remaining, got: %s", editedStr)
	}
}

func TestFindAndReplaceEditFileToolReplaceAll(t *testing.T) {
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.txt")
	content := "foo bar foo baz foo"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := &FindAndReplaceEditFileTool{projectRoot: tempDir}

	args := map[string]any{
		"file_path":    "test.txt",
		"find_text":    "foo",
		"replace_text": "qux",
		"replace_all":  true,
	}

	result, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("Failed to find and replace all: %v", err)
	}

	if !strings.Contains(result, "3 occurrence") {
		t.Errorf("Expected 3 occurrences replaced, got: %s", result)
	}

	// Verify all were replaced
	edited, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read edited file: %v", err)
	}

	if strings.Contains(string(edited), "foo") {
		t.Errorf("Expected no 'foo' remaining, got: %s", string(edited))
	}

	if strings.Count(string(edited), "qux") != 3 {
		t.Errorf("Expected 3 'qux', got: %s", string(edited))
	}
}

func TestFindAndReplaceEditFileToolNotFound(t *testing.T) {
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.txt")
	content := "Hello World"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := &FindAndReplaceEditFileTool{projectRoot: tempDir}

	args := map[string]any{
		"file_path":    "test.txt",
		"find_text":    "Nonexistent",
		"replace_text": "Replacement",
	}

	_, err := tool.Execute(args)
	if err == nil {
		t.Fatal("Expected error for text not found, got nil")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}
