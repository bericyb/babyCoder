package storage

import (
	"database/sql"
	"fmt"
)

// MigrateDatabase applies any pending migrations
func (database *Database) MigrateDatabase() error {
	// Check if migrations table exists
	var tableName string
	err := database.connection.QueryRow(`
		SELECT name FROM sqlite_master 
		WHERE type='table' AND name='migrations'
	`).Scan(&tableName)

	if err == sql.ErrNoRows {
		// Create migrations table
		_, err := database.connection.Exec(`
			CREATE TABLE migrations (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				version INTEGER NOT NULL UNIQUE,
				description TEXT NOT NULL,
				applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)
		`)
		if err != nil {
			return fmt.Errorf("failed to create migrations table: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check migrations table: %w", err)
	}

	// Define migrations
	migrations := []struct {
		version     int
		description string
		apply       func() error
	}{
		{
			version:     1,
			description: "Add file_path column to tool_executions",
			apply: func() error {
				// Check if we need to add file_path column to tool_executions
				query := `SELECT COUNT(*) as count FROM pragma_table_info('tool_executions') WHERE name='file_path'`
				
				var count int
				err := database.connection.QueryRow(query).Scan(&count)
				if err != nil {
					return fmt.Errorf("failed to check for file_path column: %w", err)
				}

				// If column doesn't exist, add it
				if count == 0 {
					_, err := database.connection.Exec(`ALTER TABLE tool_executions ADD COLUMN file_path TEXT`)
					if err != nil {
						return fmt.Errorf("failed to add file_path column: %w", err)
					}
				}

				return nil
			},
		},
		{
			version:     2,
			description: "Add parent_session_id and session metadata for sub-agents",
			apply: func() error {
				// Check if parent_session_id column exists
				query := `SELECT COUNT(*) as count FROM pragma_table_info('sessions') WHERE name='parent_session_id'`
				
				var count int
				err := database.connection.QueryRow(query).Scan(&count)
				if err != nil {
					return fmt.Errorf("failed to check for parent_session_id column: %w", err)
				}

				// If columns don't exist, add them
				if count == 0 {
					_, err := database.connection.Exec(`
						ALTER TABLE sessions ADD COLUMN parent_session_id TEXT REFERENCES sessions(id);
					`)
					if err != nil {
						return fmt.Errorf("failed to add parent_session_id column: %w", err)
					}

					_, err = database.connection.Exec(`
						ALTER TABLE sessions ADD COLUMN session_type TEXT DEFAULT 'primary';
					`)
					if err != nil {
						return fmt.Errorf("failed to add session_type column: %w", err)
					}

					_, err = database.connection.Exec(`
						ALTER TABLE sessions ADD COLUMN task_description TEXT;
					`)
					if err != nil {
						return fmt.Errorf("failed to add task_description column: %w", err)
					}

					_, err = database.connection.Exec(`
						CREATE INDEX IF NOT EXISTS idx_sessions_parent ON sessions(parent_session_id);
					`)
					if err != nil {
						return fmt.Errorf("failed to create parent session index: %w", err)
					}
				}

				return nil
			},
		},
	}

	// Apply migrations
	for _, migration := range migrations {
		// Check if migration already applied
		var count int
		err := database.connection.QueryRow(
			"SELECT COUNT(*) FROM migrations WHERE version = ?",
			migration.version,
		).Scan(&count)

		if err != nil {
			return fmt.Errorf("failed to check migration version %d: %w", migration.version, err)
		}

		if count > 0 {
			// Migration already applied
			continue
		}

		// Apply migration
		if err := migration.apply(); err != nil {
			return fmt.Errorf("failed to apply migration version %d: %w", migration.version, err)
		}

		// Record migration
		_, err = database.connection.Exec(
			"INSERT INTO migrations (version, description) VALUES (?, ?)",
			migration.version,
			migration.description,
		)
		if err != nil {
			return fmt.Errorf("failed to record migration version %d: %w", migration.version, err)
		}
	}

	return nil
}
