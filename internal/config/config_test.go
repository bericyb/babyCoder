package config

import (
	"os"
	"testing"
)

func TestDefaultConfiguration(t *testing.T) {
	config := DefaultConfiguration()

	if config == nil {
		t.Fatal("DefaultConfiguration returned nil")
	}

	// Test AI Provider defaults
	if config.AIProvider.Type != "lmstudio" {
		t.Errorf("Expected AI provider type 'lmstudio', got %s", config.AIProvider.Type)
	}

	if config.AIProvider.Endpoint == "" {
		t.Error("AI provider endpoint should not be empty")
	}

	if config.AIProvider.Temperature <= 0 {
		t.Error("AI provider temperature should be positive")
	}

	// Test Agent defaults
	if config.Agent.MaxIterations <= 0 {
		t.Error("Agent max iterations should be positive")
	}
}

func TestLoadConfiguration(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := tempDir + "/.babycoder.json"

	configJSON := `{
		"ai_provider": {
			"type": "lmstudio",
			"endpoint": "http://localhost:1234/v1",
			"model": "test-model",
			"temperature": 0.5,
			"api_key": "test-key"
		},
		"agent": {
			"max_iterations": 5
		}
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Load configuration
	config, err := LoadConfiguration(configPath)
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Verify AI Provider
	if config.AIProvider.Type != "lmstudio" {
		t.Errorf("Expected type 'lmstudio', got %s", config.AIProvider.Type)
	}

	if config.AIProvider.Model != "test-model" {
		t.Errorf("Expected model 'test-model', got %s", config.AIProvider.Model)
	}

	if config.AIProvider.Temperature != 0.5 {
		t.Errorf("Expected temperature 0.5, got %f", config.AIProvider.Temperature)
	}

	// Verify Agent
	if config.Agent.MaxIterations != 5 {
		t.Errorf("Expected max iterations 5, got %d", config.Agent.MaxIterations)
	}
}

func TestLoadOrDefaultConfiguration(t *testing.T) {
	// Test with non-existent config (should return defaults)
	tempDir := t.TempDir()

	config, err := LoadOrDefaultConfiguration(tempDir)
	if err != nil {
		t.Fatalf("Failed to load or default configuration: %v", err)
	}

	if config == nil {
		t.Fatal("Configuration is nil")
	}

	// Should have default values
	if config.AIProvider.Type != "lmstudio" {
		t.Error("Expected default AI provider type")
	}
}

func TestLoadOrDefaultConfigurationWithExistingFile(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := tempDir + "/.babycoder.json"

	configJSON := `{
		"ai_provider": {
			"type": "lmstudio",
			"endpoint": "http://localhost:1234/v1",
			"model": "custom-model",
			"temperature": 0.7,
			"api_key": ""
		},
		"agent": {
			"max_iterations": 15
		}
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Load configuration
	config, err := LoadOrDefaultConfiguration(tempDir)
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Should have loaded values from file
	if config.AIProvider.Model != "custom-model" {
		t.Errorf("Expected custom-model, got %s", config.AIProvider.Model)
	}

	if config.Agent.MaxIterations != 15 {
		t.Errorf("Expected 15 max iterations, got %d", config.Agent.MaxIterations)
	}
}

func TestSaveConfiguration(t *testing.T) {
	tempDir := t.TempDir()
	configPath := tempDir + "/.babycoder.json"

	// Create a configuration
	config := DefaultConfiguration()
	config.AIProvider.Model = "saved-model"
	config.Agent.MaxIterations = 20

	// Save configuration
	if err := SaveConfiguration(config, configPath); err != nil {
		t.Fatalf("Failed to save configuration: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Configuration file was not created")
	}

	// Load and verify
	loaded, err := LoadConfiguration(configPath)
	if err != nil {
		t.Fatalf("Failed to load saved configuration: %v", err)
	}

	if loaded.AIProvider.Model != "saved-model" {
		t.Errorf("Expected saved-model, got %s", loaded.AIProvider.Model)
	}

	if loaded.Agent.MaxIterations != 20 {
		t.Errorf("Expected 20 max iterations, got %d", loaded.Agent.MaxIterations)
	}
}

func TestLoadConfigurationInvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	configPath := tempDir + "/.babycoder.json"

	// Write invalid JSON
	invalidJSON := `{"ai_provider": {invalid json}`
	if err := os.WriteFile(configPath, []byte(invalidJSON), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Attempt to load
	_, err := LoadConfiguration(configPath)
	if err == nil {
		t.Fatal("Expected error when loading invalid JSON, got nil")
	}
}

func TestLoadConfigurationNonExistentFile(t *testing.T) {
	_, err := LoadConfiguration("/nonexistent/path/.babycoder.json")
	if err == nil {
		t.Fatal("Expected error when loading non-existent file, got nil")
	}
}
