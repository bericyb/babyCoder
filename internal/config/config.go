package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Configuration holds the application configuration
type Configuration struct {
	AIProvider AIProviderConfiguration `json:"ai_provider"`
	Agent      AgentConfiguration      `json:"agent"`
	Project    ProjectConfiguration    `json:"project"`
	Prompts    PromptsConfiguration    `json:"prompts,omitempty"` // Optional prompt customization
}

// AIProviderConfiguration holds AI provider settings
type AIProviderConfiguration struct {
	Type        string  `json:"type"`         // lmstudio, openai, anthropic, etc.
	Endpoint    string  `json:"endpoint"`     // API endpoint URL
	Model       string  `json:"model"`        // Model identifier
	Temperature float32 `json:"temperature"`  // Model temperature
	APIKey      string  `json:"api_key"`      // Optional API key
}

// AgentConfiguration holds agent behavior settings
type AgentConfiguration struct {
	MaxIterations int  `json:"max_iterations"` // Maximum number of agent loop iterations
	Verbose       bool `json:"verbose"`        // Enable verbose logging
	AutoCommit    bool `json:"auto_commit"`    // Automatically commit changes
}

// ProjectConfiguration holds project-specific settings
type ProjectConfiguration struct {
	Root            string   `json:"root"`             // Project root directory
	ExcludePatterns []string `json:"exclude_patterns"` // Patterns to exclude from analysis
}

// PromptsConfiguration holds prompt customization settings
type PromptsConfiguration struct {
	MainAgent string `json:"main_agent,omitempty"` // Custom main agent prompt (inline or file://path)
	SubAgent  string `json:"sub_agent,omitempty"`  // Custom sub-agent prompt (inline or file://path)
}

// DefaultConfiguration returns a configuration with sensible defaults
func DefaultConfiguration() *Configuration {
	return &Configuration{
		AIProvider: AIProviderConfiguration{
			Type:        "lmstudio",
			Endpoint:    "http://localhost:1234/v1",
			Model:       "local-model",
			Temperature: 0.2,
			APIKey:      "",
		},
		Agent: AgentConfiguration{
			MaxIterations: 10,
			Verbose:       false,
			AutoCommit:    false,
		},
		Project: ProjectConfiguration{
			Root: ".",
			ExcludePatterns: []string{
				"vendor/",
				"**/*_test.go",
				"**/*.pb.go",
			},
		},
		Prompts: PromptsConfiguration{},
	}
}

// LoadConfiguration loads configuration from a file
func LoadConfiguration(configPath string) (*Configuration, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file: %w", err)
	}

	var configuration Configuration
	if err := json.Unmarshal(data, &configuration); err != nil {
		return nil, fmt.Errorf("failed to parse configuration file: %w", err)
	}

	return &configuration, nil
}

// LoadOrDefaultConfiguration attempts to load configuration from file, falls back to defaults
func LoadOrDefaultConfiguration(projectRoot string) (*Configuration, error) {
	configPath := filepath.Join(projectRoot, ".babycoder.json")
	
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Configuration file does not exist, use defaults
		config := DefaultConfiguration()
		config.Project.Root = projectRoot
		return config, nil
	}

	config, err := LoadConfiguration(configPath)
	if err != nil {
		return nil, err
	}

	// Override project root with provided path
	config.Project.Root = projectRoot
	return config, nil
}

// SaveConfiguration saves configuration to a file
func SaveConfiguration(config *Configuration, configPath string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}

	return nil
}
