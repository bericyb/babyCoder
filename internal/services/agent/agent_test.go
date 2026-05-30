package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/exar/babycoder/internal/config"
	"github.com/exar/babycoder/internal/services/ai_provider"
)

// MockProvider implements ai_provider.Provider for testing
type MockProvider struct {
	responses []string
	callCount int
}

func (m *MockProvider) ChatCompletion(ctx context.Context, request ai_provider.ChatCompletionRequest) (*ai_provider.ChatCompletionResponse, error) {
	if m.callCount >= len(m.responses) {
		return &ai_provider.ChatCompletionResponse{
			Choices: []ai_provider.Choice{
				{
					Message: ai_provider.Message{
						Role:    "assistant",
						Content: "Default response",
					},
					FinishReason: "stop",
				},
			},
		}, nil
	}

	response := m.responses[m.callCount]
	m.callCount++

	return &ai_provider.ChatCompletionResponse{
		Choices: []ai_provider.Choice{
			{
				Message: ai_provider.Message{
					Role:    "assistant",
					Content: response,
				},
				FinishReason: "stop",
			},
		},
	}, nil
}

func (m *MockProvider) GetModel() string {
	return "mock-model"
}

func TestDreamTimerStartsAfterRun(t *testing.T) {
	// Create temp directory for testing
	tempDir := t.TempDir()
	dreamDir := filepath.Join(tempDir, ".babycoder")
	os.MkdirAll(dreamDir, 0755)

	provider := &MockProvider{
		responses: []string{"I've completed the task"},
	}

	cfg := &config.AgentConfiguration{
		MaxIterations: 10,
	}

	agent := NewAgent(provider, cfg, nil, tempDir)
	agent.AddUserMessage("test message")

	// Run agent
	err := agent.Run(context.Background(), func(toolName string, args map[string]interface{}) (string, error) {
		return "tool result", nil
	})

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Check that timer was set
	if agent.dreamTimer == nil {
		t.Error("Dream timer was not set after Run()")
	}
}

func TestDreamTimerCancelledOnNewRun(t *testing.T) {
	tempDir := t.TempDir()
	dreamDir := filepath.Join(tempDir, ".babycoder")
	os.MkdirAll(dreamDir, 0755)

	provider := &MockProvider{
		responses: []string{
			"Response 1",
			"Response 2",
		},
	}

	cfg := &config.AgentConfiguration{
		MaxIterations: 10,
	}

	agent := NewAgent(provider, cfg, nil, tempDir)
	agent.AddUserMessage("first message")

	// First run
	agent.Run(context.Background(), func(toolName string, args map[string]interface{}) (string, error) {
		return "tool result", nil
	})

	firstTimer := agent.dreamTimer

	// Second run should cancel first timer
	agent.AddUserMessage("second message")
	agent.Run(context.Background(), func(toolName string, args map[string]interface{}) (string, error) {
		return "tool result", nil
	})

	secondTimer := agent.dreamTimer

	if firstTimer == secondTimer {
		t.Error("Timer was not replaced on second Run()")
	}
}

func TestSummarizeSession(t *testing.T) {
	tempDir := t.TempDir()

	provider := &MockProvider{
		responses: []string{"This is a summary of the session."},
	}

	cfg := &config.AgentConfiguration{
		MaxIterations: 10,
	}

	agent := NewAgent(provider, cfg, nil, tempDir)

	// Add some messages
	agent.messages = []ai_provider.Message{
		{Role: "system", Content: "You are a coding agent"},
		{Role: "user", Content: "Add a function"},
		{Role: "assistant", Content: "I've added the function"},
		{Role: "user", Content: "Now test it"},
		{Role: "assistant", Content: "I've added tests"},
	}

	summary := agent.summarizeSession(context.Background())

	if summary == "" {
		t.Error("Summary should not be empty")
	}

	if summary != "This is a summary of the session." {
		t.Errorf("Expected specific summary, got: %s", summary)
	}
}

func TestSummarizeSessionWithLongMessages(t *testing.T) {
	tempDir := t.TempDir()

	provider := &MockProvider{
		responses: []string{"Summary"},
	}

	cfg := &config.AgentConfiguration{
		MaxIterations: 10,
	}

	agent := NewAgent(provider, cfg, nil, tempDir)

	// Add message with content longer than 200 chars
	longContent := strings.Repeat("x", 300)
	agent.messages = []ai_provider.Message{
		{Role: "user", Content: longContent},
	}

	summary := agent.summarizeSession(context.Background())

	if summary == "" {
		t.Error("Summary should not be empty")
	}

	// The long content should be truncated in the prompt, but we got a summary
	if provider.callCount != 1 {
		t.Error("Provider should have been called once")
	}
}

func TestDecideDreamUpdate(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name            string
		currentDream    string
		sessionSummary  string
		llmResponse     string
		expectUpdate    bool
	}{
		{
			name:           "No update needed",
			currentDream:   "Project is about X",
			sessionSummary: "Fixed a typo",
			llmResponse:    "NO_UPDATE",
			expectUpdate:   false,
		},
		{
			name:           "Update needed",
			currentDream:   "Project is about X",
			sessionSummary: "Added new feature Y",
			llmResponse:    "Project is about X and now includes feature Y",
			expectUpdate:   true,
		},
		{
			name:           "Empty current dream",
			currentDream:   "",
			sessionSummary: "Created initial project structure",
			llmResponse:    "This is a new project with basic structure",
			expectUpdate:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &MockProvider{
				responses: []string{tt.llmResponse},
			}

			cfg := &config.AgentConfiguration{
				MaxIterations: 10,
			}

			agent := NewAgent(provider, cfg, nil, tempDir)

			result := agent.decideDreamUpdate(context.Background(), tt.currentDream, tt.sessionSummary)

			if tt.expectUpdate {
				if result == "NO_UPDATE" || result == "" {
					t.Errorf("Expected update, got: %s", result)
				}
			} else {
				if result != "NO_UPDATE" {
					t.Errorf("Expected NO_UPDATE, got: %s", result)
				}
			}
		})
	}
}

func TestUpdateDreamWithInsufficientMessages(t *testing.T) {
	tempDir := t.TempDir()
	dreamDir := filepath.Join(tempDir, ".babycoder")
	os.MkdirAll(dreamDir, 0755)

	provider := &MockProvider{}

	cfg := &config.AgentConfiguration{
		MaxIterations: 10,
	}

	agent := NewAgent(provider, cfg, nil, tempDir)

	// Add only 2 messages (should skip update)
	agent.messages = []ai_provider.Message{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "Hello"},
	}

	agent.updateDream(context.Background())

	// Provider should not have been called
	if provider.callCount != 0 {
		t.Error("updateDream should skip when messages <= 2")
	}

	// Dream file should not exist
	dreamPath := filepath.Join(tempDir, ".babycoder", "dream.txt")
	if _, err := os.Stat(dreamPath); err == nil {
		t.Error("Dream file should not be created with insufficient messages")
	}
}

func TestUpdateDreamCreatesFile(t *testing.T) {
	tempDir := t.TempDir()
	dreamDir := filepath.Join(tempDir, ".babycoder")
	os.MkdirAll(dreamDir, 0755)

	provider := &MockProvider{
		responses: []string{
			"User added a new feature",
			"This project now includes a new feature",
		},
	}

	cfg := &config.AgentConfiguration{
		MaxIterations: 10,
	}

	agent := NewAgent(provider, cfg, nil, tempDir)

	// Add enough messages
	agent.messages = []ai_provider.Message{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "Add a feature"},
		{Role: "assistant", Content: "Done"},
	}

	agent.updateDream(context.Background())

	// Wait a bit for async operation (though updateDream is actually synchronous)
	time.Sleep(10 * time.Millisecond)

	// Check dream file was created
	dreamPath := filepath.Join(tempDir, ".babycoder", "dream.txt")
	content, err := os.ReadFile(dreamPath)
	if err != nil {
		t.Fatalf("Dream file should exist: %v", err)
	}

	if string(content) != "This project now includes a new feature" {
		t.Errorf("Unexpected dream content: %s", content)
	}
}

func TestUpdateDreamWithNoUpdate(t *testing.T) {
	tempDir := t.TempDir()
	dreamDir := filepath.Join(tempDir, ".babycoder")
	os.MkdirAll(dreamDir, 0755)

	// Create initial dream
	dreamPath := filepath.Join(dreamDir, "dream.txt")
	initialDream := "Initial project description"
	os.WriteFile(dreamPath, []byte(initialDream), 0644)

	provider := &MockProvider{
		responses: []string{
			"User fixed a typo",
			"NO_UPDATE",
		},
	}

	cfg := &config.AgentConfiguration{
		MaxIterations: 10,
	}

	agent := NewAgent(provider, cfg, nil, tempDir)

	// Add messages
	agent.messages = []ai_provider.Message{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "Fix typo"},
		{Role: "assistant", Content: "Fixed"},
	}

	agent.updateDream(context.Background())

	// Dream should remain unchanged
	content, err := os.ReadFile(dreamPath)
	if err != nil {
		t.Fatalf("Dream file should exist: %v", err)
	}

	if string(content) != initialDream {
		t.Error("Dream should not be modified when LLM returns NO_UPDATE")
	}
}

func TestUpdateDreamUpdatesExistingFile(t *testing.T) {
	tempDir := t.TempDir()
	dreamDir := filepath.Join(tempDir, ".babycoder")
	os.MkdirAll(dreamDir, 0755)

	// Create initial dream
	dreamPath := filepath.Join(dreamDir, "dream.txt")
	initialDream := "Initial project description"
	os.WriteFile(dreamPath, []byte(initialDream), 0644)

	provider := &MockProvider{
		responses: []string{
			"User added authentication",
			"Updated project description with authentication",
		},
	}

	cfg := &config.AgentConfiguration{
		MaxIterations: 10,
	}

	agent := NewAgent(provider, cfg, nil, tempDir)

	// Add messages
	agent.messages = []ai_provider.Message{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "Add auth"},
		{Role: "assistant", Content: "Added authentication"},
	}

	agent.updateDream(context.Background())

	// Dream should be updated
	content, err := os.ReadFile(dreamPath)
	if err != nil {
		t.Fatalf("Dream file should exist: %v", err)
	}

	expected := "Updated project description with authentication"
	if string(content) != expected {
		t.Errorf("Expected: %s\nGot: %s", expected, string(content))
	}
}

func TestNewAgentWithProjectRoot(t *testing.T) {
	provider := &MockProvider{}
	cfg := &config.AgentConfiguration{MaxIterations: 10}
	projectRoot := "/test/project"

	agent := NewAgent(provider, cfg, nil, projectRoot)

	if agent.projectRoot != projectRoot {
		t.Errorf("Expected projectRoot: %s, got: %s", projectRoot, agent.projectRoot)
	}
}

func TestUpdateDreamWithNoProjectRoot(t *testing.T) {
	provider := &MockProvider{
		responses: []string{"Summary", "Updated dream"},
	}

	cfg := &config.AgentConfiguration{
		MaxIterations: 10,
	}

	// Agent without project root
	agent := NewAgent(provider, cfg, nil, "")

	// Add messages
	agent.messages = []ai_provider.Message{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "Test"},
		{Role: "assistant", Content: "Response"},
	}

	// Should not panic, just return early
	agent.updateDream(context.Background())

	// Provider should not be called
	if provider.callCount != 0 {
		t.Error("updateDream should skip when projectRoot is empty")
	}
}
