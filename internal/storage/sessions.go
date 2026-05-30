package storage

import (
	"database/sql"
	"fmt"
	"time"
)

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
