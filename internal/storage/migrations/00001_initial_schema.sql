-- +goose Up
-- +goose StatementBegin
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    project_root TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    parent_session_id TEXT,
    session_type TEXT NOT NULL DEFAULT 'primary',
    task_description TEXT NOT NULL DEFAULT ''
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    tool_calls TEXT,
    tool_call_id TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    tokens_prompt INTEGER NOT NULL DEFAULT 0,
    tokens_response INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE tool_executions (
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
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_messages_session ON messages(session_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_messages_created ON messages(session_id, created_at);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_tool_executions_session ON tool_executions(session_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_sessions_updated ON sessions(updated_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_sessions_project ON sessions(project_root);
-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_sessions_project;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_sessions_updated;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_tool_executions_session;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_messages_created;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_messages_session;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS tool_executions;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS messages;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS sessions;
-- +goose StatementEnd
