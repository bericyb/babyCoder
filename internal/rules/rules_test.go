package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRulesManager(t *testing.T) {
	tempDir := t.TempDir()
	rm := NewRulesManager(tempDir)

	if rm.projectRoot != tempDir {
		t.Errorf("Expected project root %s, got %s", tempDir, rm.projectRoot)
	}

	expectedPath := filepath.Join(tempDir, "rules.md")
	if rm.GetRulesPath() != expectedPath {
		t.Errorf("Expected rules path %s, got %s", expectedPath, rm.GetRulesPath())
	}
}

func TestRulesExist(t *testing.T) {
	tempDir := t.TempDir()
	rm := NewRulesManager(tempDir)

	// Should not exist initially
	if rm.RulesExist() {
		t.Error("Rules file should not exist initially")
	}

	// Create the file
	err := rm.AddRule("Test rule")
	if err != nil {
		t.Fatalf("Failed to add rule: %v", err)
	}

	// Should exist now
	if !rm.RulesExist() {
		t.Error("Rules file should exist after adding rule")
	}
}

func TestAddRule(t *testing.T) {
	tempDir := t.TempDir()
	rm := NewRulesManager(tempDir)

	// Add first rule
	err := rm.AddRule("Always use error handling")
	if err != nil {
		t.Fatalf("Failed to add first rule: %v", err)
	}

	// Add second rule
	err = rm.AddRule("Write tests for all functions")
	if err != nil {
		t.Fatalf("Failed to add second rule: %v", err)
	}

	// Read file and verify content
	content, err := os.ReadFile(rm.GetRulesPath())
	if err != nil {
		t.Fatalf("Failed to read rules file: %v", err)
	}

	contentStr := string(content)

	// Check header
	if !strings.Contains(contentStr, "# Rules") {
		t.Error("Rules file missing header")
	}

	// Check both rules
	if !strings.Contains(contentStr, "- Always use error handling") {
		t.Error("First rule not found in file")
	}

	if !strings.Contains(contentStr, "- Write tests for all functions") {
		t.Error("Second rule not found in file")
	}
}

func TestAddEmptyRule(t *testing.T) {
	tempDir := t.TempDir()
	rm := NewRulesManager(tempDir)

	err := rm.AddRule("")
	if err == nil {
		t.Error("Expected error when adding empty rule")
	}

	err = rm.AddRule("   ")
	if err == nil {
		t.Error("Expected error when adding whitespace-only rule")
	}
}

func TestLoadRules(t *testing.T) {
	tempDir := t.TempDir()
	rm := NewRulesManager(tempDir)

	// Load from non-existent file
	rules, err := rm.LoadRules()
	if err != nil {
		t.Fatalf("LoadRules should not error on missing file: %v", err)
	}

	if len(rules) != 0 {
		t.Errorf("Expected 0 rules, got %d", len(rules))
	}

	// Add some rules
	rm.AddRule("Rule one")
	rm.AddRule("Rule two")
	rm.AddRule("Rule three")

	// Load rules
	rules, err = rm.LoadRules()
	if err != nil {
		t.Fatalf("Failed to load rules: %v", err)
	}

	if len(rules) != 3 {
		t.Errorf("Expected 3 rules, got %d", len(rules))
	}

	expectedRules := []string{"Rule one", "Rule two", "Rule three"}
	for i, expected := range expectedRules {
		if rules[i] != expected {
			t.Errorf("Rule %d: expected %q, got %q", i, expected, rules[i])
		}
	}
}

func TestParseRules(t *testing.T) {
	rm := NewRulesManager("")

	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "empty content",
			content:  "",
			expected: []string{},
		},
		{
			name:     "single rule with dash",
			content:  "# Rules\n\n- Use proper naming",
			expected: []string{"Use proper naming"},
		},
		{
			name:     "single rule with asterisk",
			content:  "# Rules\n\n* Use proper naming",
			expected: []string{"Use proper naming"},
		},
		{
			name: "multiple rules",
			content: `# Rules

- Rule one
- Rule two
- Rule three`,
			expected: []string{"Rule one", "Rule two", "Rule three"},
		},
		{
			name: "mixed bullets",
			content: `# Rules

- Rule one
* Rule two
- Rule three`,
			expected: []string{"Rule one", "Rule two", "Rule three"},
		},
		{
			name: "with extra whitespace",
			content: `# Rules

-   Rule with spaces   
*  Another rule  `,
			expected: []string{"Rule with spaces", "Another rule"},
		},
		{
			name: "ignores non-list lines",
			content: `# Rules

Some text
- Rule one
Not a rule
- Rule two`,
			expected: []string{"Rule one", "Rule two"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := rm.parseRules(tt.content)

			if len(rules) != len(tt.expected) {
				t.Errorf("Expected %d rules, got %d", len(tt.expected), len(rules))
				return
			}

			for i, expected := range tt.expected {
				if rules[i] != expected {
					t.Errorf("Rule %d: expected %q, got %q", i, expected, rules[i])
				}
			}
		})
	}
}

func TestGetRulesAsText(t *testing.T) {
	tempDir := t.TempDir()
	rm := NewRulesManager(tempDir)

	// No rules
	text, err := rm.GetRulesAsText()
	if err != nil {
		t.Fatalf("GetRulesAsText failed: %v", err)
	}

	if text != "" {
		t.Errorf("Expected empty text for no rules, got: %s", text)
	}

	// Add rules
	rm.AddRule("Use descriptive names")
	rm.AddRule("Always add tests")

	// Get text
	text, err = rm.GetRulesAsText()
	if err != nil {
		t.Fatalf("GetRulesAsText failed: %v", err)
	}

	if !strings.Contains(text, "Project Rules:") {
		t.Error("Missing 'Project Rules:' header")
	}

	if !strings.Contains(text, "- Use descriptive names") {
		t.Error("Missing first rule")
	}

	if !strings.Contains(text, "- Always add tests") {
		t.Error("Missing second rule")
	}
}

func TestClearRules(t *testing.T) {
	tempDir := t.TempDir()
	rm := NewRulesManager(tempDir)

	// Clear non-existent file (should not error)
	err := rm.ClearRules()
	if err != nil {
		t.Errorf("ClearRules on non-existent file should not error: %v", err)
	}

	// Add some rules
	rm.AddRule("Rule one")
	rm.AddRule("Rule two")

	if !rm.RulesExist() {
		t.Fatal("Rules file should exist")
	}

	// Clear rules
	err = rm.ClearRules()
	if err != nil {
		t.Fatalf("Failed to clear rules: %v", err)
	}

	// Verify file is gone
	if rm.RulesExist() {
		t.Error("Rules file should not exist after clearing")
	}
}
