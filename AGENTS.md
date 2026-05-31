# babyCoder — Contributor Guide for Agents

This file captures project-specific conventions and operational rules that any contributor — human or AI agent — must follow when modifying this codebase. It is intentionally narrower than the README: the README explains *what* babyCoder is, this file explains *how to work inside it*.

Read this in addition to `README.md` and `docs/` before making non-trivial changes.

---

## 1. Architectural Rules

### Layered structure

- `main.go` and the `internal/.../handlers`-style entry points act as the **orchestrator layer**: they parse input, trap errors, and coordinate calls into services. Keep them thin.
- `internal/services/*` contains **pure business logic**. Service code must not know about HTTP, CLI argument parsing, or any other transport concern.
- `internal/storage/*` is the only place that talks to SQLite. Other packages must go through it.

### Single source of truth

SQLite (`.babycoder/babycoder.db`) is the definitive store for all session, message, and tool-execution data. Do not introduce in-memory caches that shadow database state unless there is a measured performance reason and an explicit invalidation strategy.

---

## 2. Database & Migrations

This is the rule contributors get wrong most often, so it is called out first.

### Use goose. Always.

Schema changes are managed exclusively through [goose](https://github.com/pressly/goose) migration files in `internal/storage/migrations/`. The full workflow is documented in the "Database Migrations" section of `README.md`. The short version:

- One SQL file per migration, sequentially numbered: `NNNNN_short_description.sql`.
- Each file has a `-- +goose Up` and a `-- +goose Down` section.
- Wrap individual statements in `-- +goose StatementBegin` / `-- +goose StatementEnd`.
- Migrations are embedded into the binary via `//go:embed migrations/*.sql` in `internal/storage/migrations.go`; no runtime file-system access is required.
- `goose_db_version` is goose's bookkeeping table — never touch it directly.

### Do not

- **Do not** add `ALTER TABLE ... ADD COLUMN` (or any other DDL) directly inside Go code in `internal/storage/`. Put it in a new migration file.
- **Do not** swallow SQLite errors by string-matching their text (e.g. matching `"duplicate column name"` to fake idempotency). This pattern previously caused real bugs and was removed deliberately.
- **Do not** leave a `sql.Rows` cursor open across a subsequent write to the same table. SQLite will report "database is locked". If you `break` out of a `rows.Next()` loop early, call `rows.Close()` explicitly before issuing any write statement — `defer rows.Close()` is function-scoped and is **not** sufficient.

### Connection handling

- Always enable foreign keys (`PRAGMA foreign_keys = ON`) on every connection. The existing `NewDatabase` in `internal/storage/database.go` does this; preserve it.
- On any error during `NewDatabase`, close the connection before returning.

---

## 3. Go Style

These extend the general Go idioms; they are the points this project cares about specifically.

- **Naming**: plain, descriptive nouns. **Abbreviations are forbidden** — write `database` not `db`, `connection` not `conn`, `configuration` not `cfg`. The only exceptions are widely-recognized initialisms already used in the standard library (`URL`, `ID`, `HTTP`). Existing code follows this; new code must too.
- **Error handling**: check immediately after the call. Wrap with `fmt.Errorf("…: %w", err)` and include enough context to identify *where* the failure happened (function, operation, identifier).
- **User-facing vs developer-facing messages**: keep them separate. Errors surfaced to the CLI user must be clear and actionable. Detailed diagnostics (stack traces, raw driver errors, IDs) belong in the file logger under `.babycoder/logs/`, not on stdout.
- **Logging**: use the `internal/logging` package. Use distinct levels (`INFO`, `WARN`, `ERROR`, `DEBUG`). Include context variables — session ID, file path, tool name, etc.
- **Concurrency**: prefer channels and goroutines for inter-component communication; reach for `sync.Mutex` only when protecting in-process shared state within a single component.

---

## 4. Testing

- Every new feature requires tests. Both unit-level (isolated logic) and integration-level (handler → service → SQLite) coverage are expected.
- Storage tests run against a real SQLite file in `t.TempDir()`. Follow the patterns in `internal/storage/database_test.go`.
- Run `go test ./...` before considering work complete. CI parity is enforced informally — if your change doesn't pass locally, it isn't done.
- When a bug is fixed, add a regression test that fails without the fix.

---

## 5. Dependencies

- The Go toolchain version is pinned in `go.mod` (`go 1.25.x`). If `go get` requests a newer toolchain, that is acceptable when justified by a needed dependency; mention the bump in the change summary.
- Prefer well-maintained libraries with small dependency footprints. Audit transitive deps before adding anything.
- Current external runtime deps of note:
  - `github.com/mattn/go-sqlite3` — SQLite driver (cgo).
  - `github.com/pressly/goose/v3` — migration runner.
  - `github.com/google/uuid` — session IDs.

---

## 6. Workflow Expectations for AI Agents

When operating on this repository as an AI coding agent:

1. **Investigate before editing.** When the user asks you to "look into" or "investigate" something, present findings and a plan first. Do not modify files until the user confirms direction.
2. **Stay in scope.** Fix the bug the user asked about. Do not opportunistically refactor adjacent code, replace helper functions, or "clean up" things that were not part of the request. If you notice something worth changing, mention it — do not change it.
3. **Keep changes minimal and reviewable.** Prefer the smallest edit that correctly solves the problem.
4. **Verify.** Run `go build ./...`, `go vet ./...`, and the relevant `go test` invocations before declaring work complete.
5. **Summarize honestly.** State exactly what changed, what tests were run, and any caveats (e.g. "delete existing dev `.db` files before the next run"). Do not hide trade-offs.

---

## 7. Where to Look First

- Architecture overview: `README.md`
- Database schema and storage API: `docs/STORAGE.md`, `docs/SESSION_TRACKING.md`
- Migrations: `internal/storage/migrations/` and the "Database Migrations" section of `README.md`
- Interactive CLI behavior: `docs/INTERACTIVE_MODE.md`
- Logging and test layout: `docs/LOGGING_AND_TESTS.md`
