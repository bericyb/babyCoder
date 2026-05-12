package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetProjectStructureTool(t *testing.T) {
	tempDir := t.TempDir()

	// Create a simple Go project structure
	structure := map[string]string{
		"main.go": `package main

func main() {
	println("hello")
}
`,
		"internal/utils/helper.go": `package utils

func Helper() string {
	return "help"
}
`,
		"internal/models/user.go": `package models

type User struct {
	Name string
	Age  int
}

func NewUser(name string) *User {
	return &User{Name: name}
}
`,
	}

	// Create files
	for filePath, content := range structure {
		fullPath := filepath.Join(tempDir, filePath)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filePath, err)
		}
	}

	tool := &GetProjectStructureTool{ProjectRoot: tempDir}

	args := map[string]interface{}{
		"include_imports": false,
		"include_exports": true,
		"max_depth":       10,
	}

	result, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("Failed to execute tool: %v", err)
	}

	// Verify output contains expected information
	if !strings.Contains(result, "Go Project Structure") {
		t.Error("Expected 'Go Project Structure' in output")
	}

	if !strings.Contains(result, "Total Packages:") {
		t.Error("Expected 'Total Packages:' in output")
	}

	// Should find 3 packages: main, utils, models
	if !strings.Contains(result, "main") {
		t.Error("Expected to find 'main' package")
	}

	if !strings.Contains(result, "utils") {
		t.Error("Expected to find 'utils' package")
	}

	if !strings.Contains(result, "models") {
		t.Error("Expected to find 'models' package")
	}

	// Check for exported symbols
	if !strings.Contains(result, "User") {
		t.Error("Expected to find 'User' type in models package")
	}

	if !strings.Contains(result, "Helper") {
		t.Error("Expected to find 'Helper' function in utils package")
	}
}

func TestGetProjectStructureToolEmpty(t *testing.T) {
	tempDir := t.TempDir()

	tool := &GetProjectStructureTool{ProjectRoot: tempDir}

	args := map[string]interface{}{}

	result, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("Failed to execute tool: %v", err)
	}

	if !strings.Contains(result, "No Go packages found") {
		t.Errorf("Expected 'No Go packages found' message, got: %s", result)
	}
}
