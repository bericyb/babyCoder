package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Database represents the SQLite database connection
type Database struct {
	connection *sql.DB
}

// Session represents a conversation session
type Session struct {
	ID              string    `json:"id"`
	ProjectRoot     string    `json:"project_root"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	Title           string    `json:"title"`
	Status          string    `json:"status"`           // active, completed, failed
	ParentSessionID *string   `json:"parent_session_id"` // NULL for primary sessions
	SessionType     string    `json:"session_type"`      // 'primary' or 'subagent'
	TaskDescription string    `json:"task_description"`  // What the sub-agent was asked to do
}

// Message represents a single message in the conversation
type Message struct {
	ID             int64     `json:"id"`
	SessionID      string    `json:"session_id"`
	Role           string    `json:"role"`            // system, user, assistant, tool
	Content        string    `json:"content"`
	ToolCalls      string    `json:"tool_calls"`      // JSON string of tool calls
	ToolCallID     string    `json:"tool_call_id"`
	MessageIndex   int       `json:"message_index"`   // Order within session
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

// DocumentationHash represents a tracked symbol's signature and doc hash
type DocumentationHash struct {
	ID            int64     `json:"id"`
	FilePath      string    `json:"file_path"`
	SymbolName    string    `json:"symbol_name"`
	SymbolType    string    `json:"symbol_type"` // func, struct, interface, const, var
	SignatureHash string    `json:"signature_hash"`
	DocHash       string    `json:"doc_hash"`
	IsStale       bool      `json:"is_stale"`
	LastChecked   time.Time `json:"last_checked"`
}

// DocumentationUpdate represents a pending or completed doc update job
type DocumentationUpdate struct {
	ID           int64     `json:"id"`
	FilePath     string    `json:"file_path"`
	SymbolName   string    `json:"symbol_name"`
	OldSignature string    `json:"old_signature"`
	NewSignature string    `json:"new_signature"`
	OldDoc       string    `json:"old_doc"`
	NewDoc       string    `json:"new_doc"`
	Status       string    `json:"status"` // pending, processing, completed, failed
	CreatedAt    time.Time `json:"created_at"`
	CompletedAt  time.Time `json:"completed_at,omitempty"`
	ErrorMsg     string    `json:"error_msg,omitempty"`
}

// NewDatabase creates a new database connection and initializes schema
func NewDatabase(databasePath string) (*Database, error) {
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

	if err := database.initializeSchema(); err != nil {
		connection.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Apply any pending migrations
	if err := database.MigrateDatabase(); err != nil {
		connection.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return database, nil
}

// initializeSchema creates the database tables if they don't exist
func (database *Database) initializeSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		project_root TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		title TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'active'
	);

	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		tool_calls TEXT,
		tool_call_id TEXT,
		message_index INTEGER NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		tokens_prompt INTEGER DEFAULT 0,
		tokens_response INTEGER DEFAULT 0,
		FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS tool_executions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		message_id INTEGER NOT NULL,
		tool_name TEXT NOT NULL,
		arguments TEXT NOT NULL,
		result TEXT NOT NULL,
		file_path TEXT,
		success BOOLEAN NOT NULL DEFAULT 1,
		error_msg TEXT,
		execution_ms INTEGER NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
		FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);
	CREATE INDEX IF NOT EXISTS idx_messages_order ON messages(session_id, message_index);
	CREATE INDEX IF NOT EXISTS idx_tool_executions_session ON tool_executions(session_id);
	CREATE INDEX IF NOT EXISTS idx_sessions_updated ON sessions(updated_at DESC);
	CREATE INDEX IF NOT EXISTS idx_sessions_project ON sessions(project_root);

	CREATE TABLE IF NOT EXISTS documentation_hashes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file_path TEXT NOT NULL,
		symbol_name TEXT NOT NULL,
		symbol_type TEXT NOT NULL,
		signature_hash TEXT NOT NULL,
		doc_hash TEXT NOT NULL,
		is_stale BOOLEAN NOT NULL DEFAULT 0,
		last_checked TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(file_path, symbol_name)
	);

	CREATE TABLE IF NOT EXISTS documentation_updates (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file_path TEXT NOT NULL,
		symbol_name TEXT NOT NULL,
		old_signature TEXT,
		new_signature TEXT,
		old_doc TEXT,
		new_doc TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		completed_at TIMESTAMP,
		error_msg TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_doc_hashes_file ON documentation_hashes(file_path);
	CREATE INDEX IF NOT EXISTS idx_doc_hashes_stale ON documentation_hashes(is_stale);
	CREATE INDEX IF NOT EXISTS idx_doc_updates_status ON documentation_updates(status);
	CREATE INDEX IF NOT EXISTS idx_doc_updates_created ON documentation_updates(created_at DESC);
	`

	_, err := database.connection.Exec(schema)
	return err
}

// Close closes the database connection
func (database *Database) Close() error {
	return database.connection.Close()
}

// CreateSession creates a new session
func (database *Database) CreateSession(session *Session) error {
	query := `
		INSERT INTO sessions (id, project_root, created_at, updated_at, title, status, parent_session_id, session_type, task_description)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := database.connection.Exec(
		query,
		session.ID,
		session.ProjectRoot,
		session.CreatedAt,
		session.UpdatedAt,
		session.Title,
		session.Status,
		session.ParentSessionID,
		session.SessionType,
		session.TaskDescription,
	)

	return err
}

// GetSession retrieves a session by ID
func (database *Database) GetSession(sessionID string) (*Session, error) {
	query := `
		SELECT id, project_root, created_at, updated_at, title, status, parent_session_id, session_type, task_description 
		FROM sessions 
		WHERE id = ?
	`

	var session Session
	err := database.connection.QueryRow(query, sessionID).Scan(
		&session.ID,
		&session.ProjectRoot,
		&session.CreatedAt,
		&session.UpdatedAt,
		&session.Title,
		&session.Status,
		&session.ParentSessionID,
		&session.SessionType,
		&session.TaskDescription,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return &session, nil
}

// UpdateSession updates an existing session
func (database *Database) UpdateSession(session *Session) error {
	query := `
		UPDATE sessions
		SET updated_at = ?, title = ?, status = ?
		WHERE id = ?
	`

	_, err := database.connection.Exec(
		query,
		session.UpdatedAt,
		session.Title,
		session.Status,
		session.ID,
	)

	return err
}

// UpdateSessionStatus updates only the status field of a session
func (database *Database) UpdateSessionStatus(sessionID string, status string) error {
	query := `
		UPDATE sessions
		SET status = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := database.connection.Exec(query, status, time.Now(), sessionID)
	return err
}

// ListSessions retrieves all sessions, optionally filtered by project root
func (database *Database) ListSessions(projectRoot string, limit int) ([]*Session, error) {
	var query string
	var args []interface{}

	if projectRoot != "" {
		query = `
			SELECT id, project_root, created_at, updated_at, title, status, parent_session_id, session_type, task_description
			FROM sessions
			WHERE project_root = ?
			ORDER BY updated_at DESC
			LIMIT ?
		`
		args = []interface{}{projectRoot, limit}
	} else {
		query = `
			SELECT id, project_root, created_at, updated_at, title, status, parent_session_id, session_type, task_description
			FROM sessions
			ORDER BY updated_at DESC
			LIMIT ?
		`
		args = []interface{}{limit}
	}

	rows, err := database.connection.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		var session Session
		if err := rows.Scan(
			&session.ID,
			&session.ProjectRoot,
			&session.CreatedAt,
			&session.UpdatedAt,
			&session.Title,
			&session.Status,
			&session.ParentSessionID,
			&session.SessionType,
			&session.TaskDescription,
		); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, &session)
	}

	return sessions, rows.Err()
}

// GetSubSessions retrieves all sub-agent sessions for a parent session
func (database *Database) GetSubSessions(parentSessionID string) ([]*Session, error) {
	query := `
		SELECT id, project_root, created_at, updated_at, title, status, parent_session_id, session_type, task_description
		FROM sessions
		WHERE parent_session_id = ?
		ORDER BY created_at ASC
	`

	rows, err := database.connection.Query(query, parentSessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sub-sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		var session Session
		if err := rows.Scan(
			&session.ID,
			&session.ProjectRoot,
			&session.CreatedAt,
			&session.UpdatedAt,
			&session.Title,
			&session.Status,
			&session.ParentSessionID,
			&session.SessionType,
			&session.TaskDescription,
		); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, &session)
	}

	return sessions, rows.Err()
}

// SaveMessage saves a message to the database
func (database *Database) SaveMessage(message *Message) error {
	query := `
		INSERT INTO messages (session_id, role, content, tool_calls, tool_call_id, message_index, created_at, tokens_prompt, tokens_response)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := database.connection.Exec(
		query,
		message.SessionID,
		message.Role,
		message.Content,
		message.ToolCalls,
		message.ToolCallID,
		message.MessageIndex,
		message.CreatedAt,
		message.TokensPrompt,
		message.TokensResponse,
	)

	if err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}

	// Get the inserted ID
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get message ID: %w", err)
	}

	message.ID = id
	return nil
}

// GetMessages retrieves all messages for a session
func (database *Database) GetMessages(sessionID string) ([]*Message, error) {
	query := `
		SELECT id, session_id, role, content, tool_calls, tool_call_id, message_index, created_at, tokens_prompt, tokens_response
		FROM messages
		WHERE session_id = ?
		ORDER BY message_index ASC
	`

	rows, err := database.connection.Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		var message Message
		var toolCalls, toolCallID sql.NullString

		if err := rows.Scan(
			&message.ID,
			&message.SessionID,
			&message.Role,
			&message.Content,
			&toolCalls,
			&toolCallID,
			&message.MessageIndex,
			&message.CreatedAt,
			&message.TokensPrompt,
			&message.TokensResponse,
		); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		if toolCalls.Valid {
			message.ToolCalls = toolCalls.String
		}
		if toolCallID.Valid {
			message.ToolCallID = toolCallID.String
		}

		messages = append(messages, &message)
	}

	return messages, rows.Err()
}

// SaveToolExecution saves a tool execution record
func (database *Database) SaveToolExecution(execution *ToolExecution) error {
	query := `
		INSERT INTO tool_executions (session_id, message_id, tool_name, arguments, result, file_path, success, error_msg, execution_ms, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := database.connection.Exec(
		query,
		execution.SessionID,
		execution.MessageID,
		execution.ToolName,
		execution.Arguments,
		execution.Result,
		execution.FilePath,
		execution.Success,
		execution.ErrorMsg,
		execution.ExecutionMs,
		execution.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save tool execution: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get tool execution ID: %w", err)
	}

	execution.ID = id
	return nil
}

// GetToolExecutions retrieves all tool executions for a session
func (database *Database) GetToolExecutions(sessionID string) ([]*ToolExecution, error) {
	query := `
		SELECT id, session_id, message_id, tool_name, arguments, result, file_path, success, error_msg, execution_ms, created_at
		FROM tool_executions
		WHERE session_id = ?
		ORDER BY created_at ASC
	`

	rows, err := database.connection.Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool executions: %w", err)
	}
	defer rows.Close()

	var executions []*ToolExecution
	for rows.Next() {
		var execution ToolExecution
		var errorMsg, filePath sql.NullString

		if err := rows.Scan(
			&execution.ID,
			&execution.SessionID,
			&execution.MessageID,
			&execution.ToolName,
			&execution.Arguments,
			&execution.Result,
			&filePath,
			&execution.Success,
			&errorMsg,
			&execution.ExecutionMs,
			&execution.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan tool execution: %w", err)
		}

		if errorMsg.Valid {
			execution.ErrorMsg = errorMsg.String
		}
		if filePath.Valid {
			execution.FilePath = filePath.String
		}

		executions = append(executions, &execution)
	}

	return executions, rows.Err()
}

// DeleteSession deletes a session and all associated messages and tool executions
func (database *Database) DeleteSession(sessionID string) error {
	query := `DELETE FROM sessions WHERE id = ?`
	_, err := database.connection.Exec(query, sessionID)
	return err
}

// GetSessionStats returns statistics for a session
func (database *Database) GetSessionStats(sessionID string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get message count
	var messageCount int
	err := database.connection.QueryRow(
		`SELECT COUNT(*) FROM messages WHERE session_id = ?`,
		sessionID,
	).Scan(&messageCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get message count: %w", err)
	}
	stats["message_count"] = messageCount

	// Get tool execution count
	var toolCount int
	err = database.connection.QueryRow(
		`SELECT COUNT(*) FROM tool_executions WHERE session_id = ?`,
		sessionID,
	).Scan(&toolCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool execution count: %w", err)
	}
	stats["tool_execution_count"] = toolCount

	// Get total tokens
	var totalPromptTokens, totalResponseTokens int
	err = database.connection.QueryRow(
		`SELECT COALESCE(SUM(tokens_prompt), 0), COALESCE(SUM(tokens_response), 0) FROM messages WHERE session_id = ?`,
		sessionID,
	).Scan(&totalPromptTokens, &totalResponseTokens)
	if err != nil {
		return nil, fmt.Errorf("failed to get token counts: %w", err)
	}
	stats["total_prompt_tokens"] = totalPromptTokens
	stats["total_response_tokens"] = totalResponseTokens
	stats["total_tokens"] = totalPromptTokens + totalResponseTokens

	return stats, nil
}

// Helper function to marshal tool calls to JSON string
func MarshalToolCalls(toolCalls interface{}) string {
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
func UnmarshalToolCalls(data string, target interface{}) error {
	if data == "" {
		return nil
	}
	return json.Unmarshal([]byte(data), target)
}

// SaveDocumentationHash saves or updates a documentation hash record
func (database *Database) SaveDocumentationHash(docHash *DocumentationHash) error {
	query := `
		INSERT INTO documentation_hashes (file_path, symbol_name, symbol_type, signature_hash, doc_hash, is_stale, last_checked)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(file_path, symbol_name) 
		DO UPDATE SET 
			symbol_type = excluded.symbol_type,
			signature_hash = excluded.signature_hash,
			doc_hash = excluded.doc_hash,
			is_stale = excluded.is_stale,
			last_checked = excluded.last_checked
	`

	_, err := database.connection.Exec(
		query,
		docHash.FilePath,
		docHash.SymbolName,
		docHash.SymbolType,
		docHash.SignatureHash,
		docHash.DocHash,
		docHash.IsStale,
		docHash.LastChecked,
	)

	return err
}

// GetDocumentationHashes retrieves all doc hashes for a file
func (database *Database) GetDocumentationHashes(filePath string) ([]*DocumentationHash, error) {
	query := `SELECT id, file_path, symbol_name, symbol_type, signature_hash, doc_hash, is_stale, last_checked 
	          FROM documentation_hashes WHERE file_path = ?`

	rows, err := database.connection.Query(query, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to query documentation hashes: %w", err)
	}
	defer rows.Close()

	var hashes []*DocumentationHash
	for rows.Next() {
		var hash DocumentationHash
		err := rows.Scan(
			&hash.ID,
			&hash.FilePath,
			&hash.SymbolName,
			&hash.SymbolType,
			&hash.SignatureHash,
			&hash.DocHash,
			&hash.IsStale,
			&hash.LastChecked,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan documentation hash: %w", err)
		}
		hashes = append(hashes, &hash)
	}

	return hashes, nil
}

// GetStaleDocumentationHashes retrieves all stale documentation records
func (database *Database) GetStaleDocumentationHashes() ([]*DocumentationHash, error) {
	query := `SELECT id, file_path, symbol_name, symbol_type, signature_hash, doc_hash, is_stale, last_checked 
	          FROM documentation_hashes WHERE is_stale = 1`

	rows, err := database.connection.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query stale hashes: %w", err)
	}
	defer rows.Close()

	var hashes []*DocumentationHash
	for rows.Next() {
		var hash DocumentationHash
		err := rows.Scan(
			&hash.ID,
			&hash.FilePath,
			&hash.SymbolName,
			&hash.SymbolType,
			&hash.SignatureHash,
			&hash.DocHash,
			&hash.IsStale,
			&hash.LastChecked,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan documentation hash: %w", err)
		}
		hashes = append(hashes, &hash)
	}

	return hashes, nil
}

// CreateDocumentationUpdate creates a new documentation update job
func (database *Database) CreateDocumentationUpdate(update *DocumentationUpdate) error {
	query := `
		INSERT INTO documentation_updates (file_path, symbol_name, old_signature, new_signature, old_doc, new_doc, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := database.connection.Exec(
		query,
		update.FilePath,
		update.SymbolName,
		update.OldSignature,
		update.NewSignature,
		update.OldDoc,
		update.NewDoc,
		update.Status,
		update.CreatedAt,
	)

	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err == nil {
		update.ID = id
	}

	return nil
}

// UpdateDocumentationUpdate updates an existing documentation update record
func (database *Database) UpdateDocumentationUpdate(update *DocumentationUpdate) error {
	query := `
		UPDATE documentation_updates
		SET new_doc = ?, status = ?, completed_at = ?, error_msg = ?
		WHERE id = ?
	`

	_, err := database.connection.Exec(
		query,
		update.NewDoc,
		update.Status,
		update.CompletedAt,
		update.ErrorMsg,
		update.ID,
	)

	return err
}

// GetPendingDocumentationUpdates retrieves all pending update jobs
func (database *Database) GetPendingDocumentationUpdates() ([]*DocumentationUpdate, error) {
	query := `SELECT id, file_path, symbol_name, old_signature, new_signature, old_doc, new_doc, status, created_at, completed_at, error_msg
	          FROM documentation_updates WHERE status = 'pending' ORDER BY created_at ASC`

	rows, err := database.connection.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending updates: %w", err)
	}
	defer rows.Close()

	var updates []*DocumentationUpdate
	for rows.Next() {
		var update DocumentationUpdate
		var completedAt sql.NullTime
		var errorMsg sql.NullString

		err := rows.Scan(
			&update.ID,
			&update.FilePath,
			&update.SymbolName,
			&update.OldSignature,
			&update.NewSignature,
			&update.OldDoc,
			&update.NewDoc,
			&update.Status,
			&update.CreatedAt,
			&completedAt,
			&errorMsg,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan documentation update: %w", err)
		}

		if completedAt.Valid {
			update.CompletedAt = completedAt.Time
		}
		if errorMsg.Valid {
			update.ErrorMsg = errorMsg.String
		}

		updates = append(updates, &update)
	}

	return updates, nil
}

// RecordManagedProcess records a new managed process in the database

