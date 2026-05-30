package storage

import (
	"fmt"
)

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
