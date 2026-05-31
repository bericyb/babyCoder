package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Database represents the SQLite database connection
type Database struct {
	connection *sql.DB
}

// Session represents a conversation session
type Session struct {
	ID          string    `json:"id"`
	ProjectRoot string    `json:"project_root"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Title       string    `json:"title"`
	Status      string    `json:"status"` // active, completed, failed
}

// Message represents a single message in the conversation
type Message struct {
	ID             int64     `json:"id"`
	SessionID      string    `json:"session_id"`
	Role           string    `json:"role"`            // system, user, assistant, tool
	Content        string    `json:"content"`
	ToolCalls      string    `json:"tool_calls"`      // JSON string of tool calls
	ToolCallID     string    `json:"tool_call_id"`
	CreatedAt      time.Time `json:"created_at"`
	TokensPrompt   int       `json:"tokens_prompt"`   // Token count for this message
	TokensResponse int       `json:"tokens_response"` // Token count for response
}

// ToolExecution represents a tool execution record
type ToolExecution struct {
	ID          int64     `json:"id"`
	SessionID   string    `json:"session_id"`
	MessageID   int64     `json:"message_id"`
	ToolName    string    `json:"tool_name"`
	Arguments   string    `json:"arguments"`   // JSON string
	Result      string    `json:"result"`
	FilePath    string    `json:"file_path"`   // File affected by this tool (if applicable)
	Success     bool      `json:"success"`
	ErrorMsg    string    `json:"error_msg"`
	ExecutionMs int64     `json:"execution_ms"` // Execution time in milliseconds
	CreatedAt   time.Time `json:"created_at"`
}

// NewDatabase creates a new database connection and initializes schema.
// The parent directory of databasePath is created if it does not yet exist,
// so callers do not need to ensure the directory exists beforehand.
func NewDatabase(databasePath string) (*Database, error) {
	if err := os.MkdirAll(filepath.Dir(databasePath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	connection, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign key constraints (required for CASCADE DELETE)
	if _, err := connection.Exec("PRAGMA foreign_keys = ON"); err != nil {
		connection.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	database := &Database{connection: connection}

	// Apply any pending schema migrations. goose itself creates and maintains
	// the bookkeeping table on first run, so no separate schema-initialization
	// step is required here.
	if err := database.MigrateDatabase(); err != nil {
		connection.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return database, nil
}

// Close closes the database connection
func (database *Database) Close() error {
	return database.connection.Close()
}

// Helper function to marshal tool calls to JSON string
func MarshalToolCalls(toolCalls any) string {
	if toolCalls == nil {
		return ""
	}
	data, err := json.Marshal(toolCalls)
	if err != nil {
		return ""
	}
	return string(data)
}

// Helper function to unmarshal tool calls from JSON string
func UnmarshalToolCalls(data string, target any) error {
	if data == "" {
		return nil
	}
	return json.Unmarshal([]byte(data), target)
}
