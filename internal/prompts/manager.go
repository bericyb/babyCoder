package prompts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PromptType identifies different prompt categories
type PromptType string

const (
	// MainAgent is the primary agent's system prompt
	MainAgent PromptType = "main_agent"
	
	// SubAgent is the research sub-agent's system prompt
	SubAgent PromptType = "sub_agent"
)

// PromptManager handles loading and managing prompts
type PromptManager struct {
	projectRoot string
	overrides   map[PromptType]string
}

// NewPromptManager creates a new prompt manager
func NewPromptManager(projectRoot string) *PromptManager {
	return &PromptManager{
		projectRoot: projectRoot,
		overrides:   make(map[PromptType]string),
	}
}

// SetOverride sets a custom prompt override
func (pm *PromptManager) SetOverride(promptType PromptType, content string) {
	pm.overrides[promptType] = content
}

// GetPrompt retrieves a prompt, checking overrides first, then defaults
func (pm *PromptManager) GetPrompt(promptType PromptType) string {
	// Check for override
	if override, exists := pm.overrides[promptType]; exists {
		// Check if it's a file path
		if strings.HasPrefix(override, "file://") {
			filePath := strings.TrimPrefix(override, "file://")
			if !filepath.IsAbs(filePath) {
				filePath = filepath.Join(pm.projectRoot, filePath)
			}
			
			content, err := os.ReadFile(filePath)
			if err == nil {
				return string(content)
			}
			// Fall through to default if file read fails
		} else {
			// Direct content override
			return override
		}
	}
	
	// Return default prompt
	return pm.getDefaultPrompt(promptType)
}

// RenderPrompt renders a prompt with variable substitution
func (pm *PromptManager) RenderPrompt(promptType PromptType, vars map[string]string) string {
	prompt := pm.GetPrompt(promptType)
	
	for key, value := range vars {
		placeholder := fmt.Sprintf("{{%s}}", key)
		prompt = strings.ReplaceAll(prompt, placeholder, value)
	}
	
	return prompt
}

// LoadFromConfig loads prompt overrides from configuration
func (pm *PromptManager) LoadFromConfig(config map[string]interface{}) error {
	if config == nil {
		return nil
	}
	
	// Load main agent prompt
	if mainAgent, ok := config["main_agent"].(string); ok && mainAgent != "" {
		pm.SetOverride(MainAgent, mainAgent)
	}
	
	// Load sub-agent prompt
	if subAgent, ok := config["sub_agent"].(string); ok && subAgent != "" {
		pm.SetOverride(SubAgent, subAgent)
	}
	
	return nil
}

// getDefaultPrompt returns the built-in default prompt
func (pm *PromptManager) getDefaultPrompt(promptType PromptType) string {
	switch promptType {
	case MainAgent:
		return DefaultMainAgentPrompt
	case SubAgent:
		return DefaultSubAgentPrompt
	default:
		return ""
	}
}
