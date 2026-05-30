package storage

import "fmt"

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
	CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(session_id, created_at);
	CREATE INDEX IF NOT EXISTS idx_tool_executions_session ON tool_executions(session_id);
	CREATE INDEX IF NOT EXISTS idx_sessions_updated ON sessions(updated_at DESC);
	CREATE INDEX IF NOT EXISTS idx_sessions_project ON sessions(project_root);
	`

	_, err := database.connection.Exec(schema)
	return err
}

// MigrateDatabase applies any pending migrations
func (database *Database) MigrateDatabase() error {
	// Add parent_session_id, session_type, and task_description columns if they don't exist
	migrations := []string{
		`ALTER TABLE sessions ADD COLUMN parent_session_id TEXT`,
		`ALTER TABLE sessions ADD COLUMN session_type TEXT NOT NULL DEFAULT 'primary'`,
		`ALTER TABLE sessions ADD COLUMN task_description TEXT DEFAULT ''`,
	}

	for _, migration := range migrations {
		// Try to execute migration - if it fails because column exists, continue
		_, err := database.connection.Exec(migration)
		if err != nil && !isDuplicateColumnError(err) {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	// Drop message_index column and its index (SQLite requires table recreation)
	// Check if column exists first
	var columnExists bool
	rows, err := database.connection.Query(`PRAGMA table_info(messages)`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cid int
			var name, typ string
			var notnull, pk int
			var dfltValue interface{}
			if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err == nil {
				if name == "message_index" {
					columnExists = true
					break
				}
			}
		}
	}

	if columnExists {
		// SQLite doesn't support DROP COLUMN directly, so we recreate the table
		recreateTable := `
			-- Create new messages table without message_index
			CREATE TABLE messages_new (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				session_id TEXT NOT NULL,
				role TEXT NOT NULL,
				content TEXT NOT NULL,
				tool_calls TEXT,
				tool_call_id TEXT,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				tokens_prompt INTEGER DEFAULT 0,
				tokens_response INTEGER DEFAULT 0,
				FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
			);

			-- Copy data from old table
			INSERT INTO messages_new (id, session_id, role, content, tool_calls, tool_call_id, created_at, tokens_prompt, tokens_response)
			SELECT id, session_id, role, content, tool_calls, tool_call_id, created_at, tokens_prompt, tokens_response
			FROM messages;

			-- Drop old table
			DROP TABLE messages;

			-- Rename new table
			ALTER TABLE messages_new RENAME TO messages;

			-- Recreate indexes (without message_index)
			CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);
			CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(session_id, created_at);
		`

		_, err := database.connection.Exec(recreateTable)
		if err != nil {
			return fmt.Errorf("failed to migrate messages table: %w", err)
		}
	}

	return nil
}

// isDuplicateColumnError checks if the error is due to duplicate column
func isDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	// SQLite returns this error when column already exists
	return contains(err.Error(), "duplicate column name")
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
