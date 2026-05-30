package storage

import (
	"database/sql"
	"fmt"
)

// SaveMessage saves a message to the database
func (database *Database) SaveMessage(message *Message) error {
	query := `
		INSERT INTO messages (session_id, role, content, tool_calls, tool_call_id, created_at, tokens_prompt, tokens_response)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := database.connection.Exec(
		query,
		message.SessionID,
		message.Role,
		message.Content,
		message.ToolCalls,
		message.ToolCallID,
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
		SELECT id, session_id, role, content, tool_calls, tool_call_id, created_at, tokens_prompt, tokens_response
		FROM messages
		WHERE session_id = ?
		ORDER BY id ASC
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
