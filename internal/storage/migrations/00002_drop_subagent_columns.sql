-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN task_description;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN session_type;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN parent_session_id;
-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN parent_session_id TEXT;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN session_type TEXT NOT NULL DEFAULT 'primary';
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN task_description TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd
