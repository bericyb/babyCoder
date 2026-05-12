package storage

import (
	"testing"
	"time"
)

func TestNewDatabase(t *testing.T) {
	// Create temporary database
	tempDir := t.TempDir()
	dbPath := tempDir + "/test.db"

	database, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	if database == nil {
		t.Fatal("Database is nil")
	}
}

func TestCreateAndGetSession(t *testing.T) {
	tempDir := t.TempDir()
	database, err := NewDatabase(tempDir + "/test.db")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	// Create a session
	session := &Session{
		ID:          "test-session-id",
		ProjectRoot: "/test/project",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Title:       "Test Session",
		Status:      "active",
	}

	err = database.CreateSession(session)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Retrieve the session
	retrieved, err := database.GetSession("test-session-id")
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Retrieved session is nil")
	}

	if retrieved.ID != session.ID {
		t.Errorf("Expected ID %s, got %s", session.ID, retrieved.ID)
	}

	if retrieved.Title != session.Title {
		t.Errorf("Expected Title %s, got %s", session.Title, retrieved.Title)
	}

	if retrieved.Status != session.Status {
		t.Errorf("Expected Status %s, got %s", session.Status, retrieved.Status)
	}
}

func TestUpdateSession(t *testing.T) {
	tempDir := t.TempDir()
	database, err := NewDatabase(tempDir + "/test.db")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	// Create a session
	session := &Session{
		ID:          "test-session-id",
		ProjectRoot: "/test/project",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Title:       "Original Title",
		Status:      "active",
	}

	database.CreateSession(session)

	// Update the session
	session.Title = "Updated Title"
	session.Status = "completed"
	session.UpdatedAt = time.Now()

	err = database.UpdateSession(session)
	if err != nil {
		t.Fatalf("Failed to update session: %v", err)
	}

	// Retrieve and verify
	retrieved, err := database.GetSession("test-session-id")
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if retrieved.Title != "Updated Title" {
		t.Errorf("Expected updated title, got %s", retrieved.Title)
	}

	if retrieved.Status != "completed" {
		t.Errorf("Expected updated status, got %s", retrieved.Status)
	}
}

func TestSaveAndGetMessages(t *testing.T) {
	tempDir := t.TempDir()
	database, err := NewDatabase(tempDir + "/test.db")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	// Create session first
	session := &Session{
		ID:          "test-session-id",
		ProjectRoot: "/test/project",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Title:       "Test Session",
		Status:      "active",
	}
	database.CreateSession(session)

	// Save messages
	messages := []*Message{
		{
			SessionID:    "test-session-id",
			Role:         "system",
			Content:      "You are a helpful assistant",
			MessageIndex: 0,
			CreatedAt:    time.Now(),
		},
		{
			SessionID:    "test-session-id",
			Role:         "user",
			Content:      "Hello!",
			MessageIndex: 1,
			CreatedAt:    time.Now(),
		},
		{
			SessionID:    "test-session-id",
			Role:         "assistant",
			Content:      "Hi! How can I help?",
			MessageIndex: 2,
			CreatedAt:    time.Now(),
		},
	}

	for _, msg := range messages {
		if err := database.SaveMessage(msg); err != nil {
			t.Fatalf("Failed to save message: %v", err)
		}
	}

	// Retrieve messages
	retrieved, err := database.GetMessages("test-session-id")
	if err != nil {
		t.Fatalf("Failed to get messages: %v", err)
	}

	if len(retrieved) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(retrieved))
	}

	// Verify message order
	if retrieved[0].Role != "system" {
		t.Errorf("Expected first message role 'system', got %s", retrieved[0].Role)
	}

	if retrieved[1].Content != "Hello!" {
		t.Errorf("Expected second message content 'Hello!', got %s", retrieved[1].Content)
	}

	if retrieved[2].Role != "assistant" {
		t.Errorf("Expected third message role 'assistant', got %s", retrieved[2].Role)
	}
}

func TestSaveAndGetToolExecutions(t *testing.T) {
	tempDir := t.TempDir()
	database, err := NewDatabase(tempDir + "/test.db")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	// Create session first
	session := &Session{
		ID:          "test-session-id",
		ProjectRoot: "/test/project",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Title:       "Test Session",
		Status:      "active",
	}
	database.CreateSession(session)

	// Create a message first (required for foreign key)
	message := &Message{
		SessionID:    "test-session-id",
		Role:         "user",
		Content:      "Test message",
		MessageIndex: 0,
		CreatedAt:    time.Now(),
	}
	if err := database.SaveMessage(message); err != nil {
		t.Fatalf("Failed to save message: %v", err)
	}

	// Save tool execution with valid message_id
	execution := &ToolExecution{
		SessionID:   "test-session-id",
		MessageID:   message.ID, // Use the actual message ID
		ToolName:    "extract_function",
		Arguments:   `{"file": "test.go", "name": "TestFunc"}`,
		Result:      "func TestFunc() { ... }",
		FilePath:    "test.go",
		Success:     true,
		ExecutionMs: 150,
		CreatedAt:   time.Now(),
	}

	if err := database.SaveToolExecution(execution); err != nil {
		t.Fatalf("Failed to save tool execution: %v", err)
	}

	// Retrieve tool executions
	executions, err := database.GetToolExecutions("test-session-id")
	if err != nil {
		t.Fatalf("Failed to get tool executions: %v", err)
	}

	if len(executions) != 1 {
		t.Fatalf("Expected 1 tool execution, got %d", len(executions))
	}

	exec := executions[0]
	if exec.ToolName != "extract_function" {
		t.Errorf("Expected tool name 'extract_function', got %s", exec.ToolName)
	}

	if !exec.Success {
		t.Error("Expected success to be true")
	}

	if exec.ExecutionMs != 150 {
		t.Errorf("Expected execution time 150ms, got %dms", exec.ExecutionMs)
	}
}

func TestListSessions(t *testing.T) {
	tempDir := t.TempDir()
	database, err := NewDatabase(tempDir + "/test.db")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	// Create multiple sessions
	for i := 0; i < 5; i++ {
		session := &Session{
			ID:          time.Now().Format("20060102150405") + string(rune('0'+i)),
			ProjectRoot: "/test/project",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Title:       "Test Session",
			Status:      "active",
		}
		database.CreateSession(session)
		time.Sleep(time.Millisecond) // Ensure different timestamps
	}

	// List sessions
	sessions, err := database.ListSessions("/test/project", 10)
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}

	if len(sessions) != 5 {
		t.Fatalf("Expected 5 sessions, got %d", len(sessions))
	}
}

func TestDeleteSession(t *testing.T) {
	tempDir := t.TempDir()
	database, err := NewDatabase(tempDir + "/test.db")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	// Create session with messages
	session := &Session{
		ID:          "test-session-id",
		ProjectRoot: "/test/project",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Title:       "Test Session",
		Status:      "active",
	}
	database.CreateSession(session)

	message := &Message{
		SessionID:    "test-session-id",
		Role:         "user",
		Content:      "Test message",
		MessageIndex: 0,
		CreatedAt:    time.Now(),
	}
	database.SaveMessage(message)

	// Delete session
	if err := database.DeleteSession("test-session-id"); err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	// Verify session is deleted
	retrieved, err := database.GetSession("test-session-id")
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if retrieved != nil {
		t.Error("Session was not deleted")
	}

	// Verify messages are also deleted (cascade)
	messages, err := database.GetMessages("test-session-id")
	if err != nil {
		t.Fatalf("Failed to get messages: %v", err)
	}

	if len(messages) != 0 {
		t.Errorf("Expected 0 messages after session deletion, got %d", len(messages))
	}
}

func TestGetSessionStats(t *testing.T) {
	tempDir := t.TempDir()
	database, err := NewDatabase(tempDir + "/test.db")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	// Create session
	session := &Session{
		ID:          "test-session-id",
		ProjectRoot: "/test/project",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Title:       "Test Session",
		Status:      "active",
	}
	database.CreateSession(session)

	// Add messages with token counts
	for i := 0; i < 3; i++ {
		message := &Message{
			SessionID:      "test-session-id",
			Role:           "user",
			Content:        "Test message",
			MessageIndex:   i,
			CreatedAt:      time.Now(),
			TokensPrompt:   10,
			TokensResponse: 20,
		}
		database.SaveMessage(message)
	}

	// Add tool execution
	execution := &ToolExecution{
		SessionID:   "test-session-id",
		MessageID:   1,
		ToolName:    "test_tool",
		Arguments:   "{}",
		Result:      "result",
		FilePath:    "test.go",
		Success:     true,
		ExecutionMs: 100,
		CreatedAt:   time.Now(),
	}
	database.SaveToolExecution(execution)

	// Get stats
	stats, err := database.GetSessionStats("test-session-id")
	if err != nil {
		t.Fatalf("Failed to get session stats: %v", err)
	}

	if stats["message_count"] != 3 {
		t.Errorf("Expected 3 messages, got %v", stats["message_count"])
	}

	if stats["tool_execution_count"] != 1 {
		t.Errorf("Expected 1 tool execution, got %v", stats["tool_execution_count"])
	}

	if stats["total_prompt_tokens"] != 30 {
		t.Errorf("Expected 30 prompt tokens, got %v", stats["total_prompt_tokens"])
	}

	if stats["total_response_tokens"] != 60 {
		t.Errorf("Expected 60 response tokens, got %v", stats["total_response_tokens"])
	}

	if stats["total_tokens"] != 90 {
		t.Errorf("Expected 90 total tokens, got %v", stats["total_tokens"])
	}
}
