package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	rulesFileName = "rules.md"
	rulesHeader   = "# Rules\n\n"
)

// RulesManager handles project-specific rules
type RulesManager struct {
	projectRoot string
	rulesPath   string
}

// NewRulesManager creates a new rules manager
func NewRulesManager(projectRoot string) *RulesManager {
	return &RulesManager{
		projectRoot: projectRoot,
		rulesPath:   filepath.Join(projectRoot, rulesFileName),
	}
}

// GetRulesPath returns the path to the rules file
func (rm *RulesManager) GetRulesPath() string {
	return rm.rulesPath
}

// RulesExist checks if a rules.md file exists
func (rm *RulesManager) RulesExist() bool {
	_, err := os.Stat(rm.rulesPath)
	return err == nil
}

// LoadRules reads all rules from rules.md
func (rm *RulesManager) LoadRules() ([]string, error) {
	if !rm.RulesExist() {
		return []string{}, nil
	}

	content, err := os.ReadFile(rm.rulesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read rules file: %w", err)
	}

	return rm.parseRules(string(content)), nil
}

// parseRules extracts rule items from markdown content
func (rm *RulesManager) parseRules(content string) []string {
	lines := strings.Split(content, "\n")
	var rules []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Look for markdown list items (- or *)
		if strings.HasPrefix(line, "- ") {
			rule := strings.TrimPrefix(line, "- ")
			rule = strings.TrimSpace(rule)
			if rule != "" {
				rules = append(rules, rule)
			}
		} else if strings.HasPrefix(line, "* ") {
			rule := strings.TrimPrefix(line, "* ")
			rule = strings.TrimSpace(rule)
			if rule != "" {
				rules = append(rules, rule)
			}
		}
	}

	return rules
}

// AddRule appends a new rule to rules.md
func (rm *RulesManager) AddRule(rule string) error {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return fmt.Errorf("rule cannot be empty")
	}

	// Initialize file if it doesn't exist
	if !rm.RulesExist() {
		if err := rm.initializeRulesFile(); err != nil {
			return err
		}
	}

	// Append rule
	file, err := os.OpenFile(rm.rulesPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open rules file: %w", err)
	}
	defer file.Close()

	_, err = file.WriteString(fmt.Sprintf("- %s\n", rule))
	if err != nil {
		return fmt.Errorf("failed to write rule: %w", err)
	}

	return nil
}

// initializeRulesFile creates a new rules.md with header
func (rm *RulesManager) initializeRulesFile() error {
	err := os.WriteFile(rm.rulesPath, []byte(rulesHeader), 0644)
	if err != nil {
		return fmt.Errorf("failed to create rules file: %w", err)
	}
	return nil
}

// GetRulesAsText returns rules formatted for system prompt injection
func (rm *RulesManager) GetRulesAsText() (string, error) {
	rules, err := rm.LoadRules()
	if err != nil {
		return "", err
	}

	if len(rules) == 0 {
		return "", nil
	}

	var builder strings.Builder
	builder.WriteString("Project Rules:\n")
	for _, rule := range rules {
		builder.WriteString(fmt.Sprintf("- %s\n", rule))
	}

	return builder.String(), nil
}

// ClearRules removes all rules (deletes the file)
func (rm *RulesManager) ClearRules() error {
	if !rm.RulesExist() {
		return nil
	}

	err := os.Remove(rm.rulesPath)
	if err != nil {
		return fmt.Errorf("failed to remove rules file: %w", err)
	}

	return nil
}
