package storage

import (
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

// migrationsFileSystem holds the embedded SQL migration files.
// Files live in ./migrations and follow goose's naming convention:
//
//	NNNNN_short_description.sql
//
// Each file contains "-- +goose Up" and "-- +goose Down" sections.
//
//go:embed migrations/*.sql
var migrationsFileSystem embed.FS

// MigrateDatabase applies all pending schema migrations to the underlying
// SQLite database using goose. Migrations are embedded in the binary so the
// application has no external file-system dependency at runtime.
//
// goose tracks which migrations have already been applied in a
// "goose_db_version" table that it manages itself, so this function is
// idempotent and safe to invoke on every startup.
func (database *Database) MigrateDatabase() error {
	goose.SetBaseFS(migrationsFileSystem)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("failed to configure migration dialect: %w", err)
	}

	if err := goose.Up(database.connection, "migrations"); err != nil {
		return fmt.Errorf("failed to apply pending migrations: %w", err)
	}

	return nil
}
