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
			SessionID: "test-session-id",
			Role:      "system",
			Content:   "You are a helpful assistant",
			CreatedAt: time.Now(),
		},
		{
			SessionID: "test-session-id",
			Role:      "user",
			Content:   "Hello!",
			CreatedAt: time.Now(),
		},
		{
			SessionID: "test-session-id",
			Role:      "assistant",
			Content:   "Hi! How can I help?",
			CreatedAt: time.Now(),
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

func TestSaveToolExecution(t *testing.T) {
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
		SessionID: "test-session-id",
		Role:      "user",
		Content:   "Test message",
		CreatedAt: time.Now(),
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

	if execution.ID == 0 {
		t.Error("Expected execution ID to be set")
	}

	if execution.ToolName != "extract_function" {
		t.Errorf("Expected tool name 'extract_function', got %s", execution.ToolName)
	}

	if !execution.Success {
		t.Error("Expected success to be true")
	}

	if execution.ExecutionMs != 150 {
		t.Errorf("Expected execution time 150ms, got %dms", execution.ExecutionMs)
	}
}
