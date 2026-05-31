package tools

import (
	"context"
	"testing"

	"github.com/exar/babycoder/internal/services/agent"
	"github.com/exar/babycoder/internal/services/ai_provider"
	"github.com/exar/babycoder/internal/storage"
)

// MockAgent implements AgentInterface for testing
type MockAgent struct {
	systemMessages []string
	userMessages   []string
	iterations     int
	messages       []ai_provider.Message
	sessionID      string
}

func (m *MockAgent) SetSessionID(id string) {
	m.sessionID = id
}

func (m *MockAgent) GetSessionID() string {
	return m.sessionID
}

func (m *MockAgent) AddSystemMessage(content string) {
	m.systemMessages = append(m.systemMessages, content)
	m.messages = append(m.messages, ai_provider.Message{
		Role:    "system",
		Content: content,
	})
}

func (m *MockAgent) AddUserMessage(content string) {
	m.userMessages = append(m.userMessages, content)
	m.messages = append(m.messages, ai_provider.Message{
		Role:    "user",
		Content: content,
	})
}

func (m *MockAgent) RunIteration(ctx context.Context, executor agent.ToolExecutor) (string, error) {
	m.iterations++
	
	// Simulate agent completing task and providing executive summary
	response := `Based on my investigation using the available tools, here are my findings:

## EXECUTIVE SUMMARY

- The notification system uses a pub/sub architecture with Redis as the message broker
- Notifications are processed asynchronously by worker processes
- User preferences are stored in PostgreSQL and cached in Redis
- The system supports email, SMS, and push notifications
- Rate limiting is implemented to prevent notification spam`

	m.messages = append(m.messages, ai_provider.Message{
		Role:    "assistant",
		Content: response,
	})

	return response, nil
}

func (m *MockAgent) HasPendingToolCalls() bool {
	return false
}

func (m *MockAgent) GetMessages() []ai_provider.Message {
	return m.messages
}

func TestSubAgentToolExecution(t *testing.T) {
	// Create temp database
	database, err := storage.NewDatabase(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	// Create parent session
	parentSession := &storage.Session{
		ID:              "parent-123",
		ProjectRoot:     "/test/project",
		Title:           "Test Parent Session",
		Status:          "active",
		ParentSessionID: nil,
		SessionType:     "primary",
		TaskDescription: "",
	}
	if err := database.CreateSession(parentSession); err != nil {
		t.Fatalf("Failed to create parent session: %v", err)
	}

	// Create mock agent factory
	mockAgent := &MockAgent{}
	agentFactory := func(sessionID string) (AgentInterface, agent.ToolExecutor, error) {
		// Store session ID in mock
		mockAgent.SetSessionID(sessionID)
		
		// Verify sub-session was created
		session, err := database.GetSession(sessionID)
		if err != nil {
			t.Errorf("Failed to get sub-session: %v", err)
		}
		if session == nil {
			t.Error("Sub-session was not created")
		} else {
			if *session.ParentSessionID != "parent-123" {
				t.Errorf("Expected parent_session_id='parent-123', got '%s'", *session.ParentSessionID)
			}
			if session.SessionType != "subagent" {
				t.Errorf("Expected session_type='subagent', got '%s'", session.SessionType)
			}
		}

		executor := func(toolName string, arguments map[string]any) (string, error) {
			return "mock result", nil
		}

		return mockAgent, executor, nil
	}

	// Create SubAgentTool
	tool := &SubAgentTool{
		ProjectRoot:   "/test/project",
		ParentSession: "parent-123",
		Database:      database,
		AgentFactory:  agentFactory,
	}

	// Execute the tool
	result, err := tool.Execute(map[string]any{
		"task":           "Find out how the notification system works",
		"max_iterations": float64(5),
	})

	if err != nil {
		t.Fatalf("SubAgentTool execution failed: %v", err)
	}

	// Verify result contains key information
	if result == "" {
		t.Error("Expected non-empty result")
	}

	// Check that summary contains expected sections
	if len(result) < 100 {
		t.Errorf("Result seems too short: %d chars", len(result))
	}

	// Verify mock agent was called
	if mockAgent.iterations == 0 {
		t.Error("Expected agent to run at least one iteration")
	}

	if len(mockAgent.systemMessages) == 0 {
		t.Error("Expected system message to be added to sub-agent")
	}

	if len(mockAgent.userMessages) == 0 {
		t.Error("Expected user message to be added to sub-agent")
	}

	// Check if task description was captured
	if mockAgent.userMessages[0] != "Find out how the notification system works" {
		t.Errorf("Expected task in user message, got: %s", mockAgent.userMessages[0])
	}

	// Verify sub-session was created and marked completed
	// Get the session ID from the mock agent
	if mockAgent.GetSessionID() == "" {
		t.Fatal("Expected sub-session ID to be set")
	}
	
	session, err := database.GetSession(mockAgent.GetSessionID())
	if err != nil {
		t.Fatalf("Failed to get sub-session: %v", err)
	}

	if session == nil {
		t.Fatal("Expected sub-session to exist")
	}

	if session.Status != "completed" {
		t.Errorf("Expected sub-session status='completed', got '%s'", session.Status)
	}
	
	if session.TaskDescription != "Find out how the notification system works" {
		t.Errorf("Expected task_description to be set, got: %s", session.TaskDescription)
	}
	
	if session.ParentSessionID == nil || *session.ParentSessionID != "parent-123" {
		t.Error("Expected parent_session_id to be 'parent-123'")
	}
}

func TestSubAgentToolDefinition(t *testing.T) {
	tool := &SubAgentTool{}
	definition := tool.GetDefinition()

	if definition.Function.Name != "run_subagent" {
		t.Errorf("Expected tool name 'run_subagent', got '%s'", definition.Function.Name)
	}

	if definition.Function.Description == "" {
		t.Error("Expected non-empty description")
	}

	// Verify parameters
	params := definition.Function.Parameters
	
	properties, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("Expected properties in parameters")
	}

	if _, exists := properties["task"]; !exists {
		t.Error("Expected 'task' parameter")
	}

	if _, exists := properties["max_iterations"]; !exists {
		t.Error("Expected 'max_iterations' parameter")
	}

	if _, exists := properties["timeout_seconds"]; !exists {
		t.Error("Expected 'timeout_seconds' parameter")
	}
}
