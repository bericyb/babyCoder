# Tools Service - Complete Agent Toolkit

The tools service provides file manipulation, code analysis, test execution, automated documentation, and bash command execution to the AI agent. All analysis and testing happens passively in the background, similar to an IDE.

## Tool Categories

1. **File Operations** - Direct file manipulation (read, write, edit, list)
2. **Code Analysis** - Passive background analysis and active queries (language-agnostic)
3. **Test Execution** - Automatic test running with pass/fail tracking
4. **Documentation** - Automatic doc updates via LLM when code changes
5. **Bash Execution** - Synchronous command execution with output capture

## Security

All tools enforce **project root containment** - files can only be accessed within the configured project root directory. Path traversal attempts (e.g., `../../../etc/passwd`) are blocked. Bash commands run with the same user permissions as babyCoder.

---

## Part 1: File Operation Tools

### 1. read_file

Read the contents of a file in the project.

**Parameters:**
- `file_path` (string, required): Path to file (relative to project root or absolute within project)

**Returns:** File contents as a string

**Example:**
```json
{
  "file_path": "main.go"
}
```

**Use Cases:**
- Examine existing code before making changes
- Read configuration files
- Inspect any text file inside the project root

---

### 2. write_file

Write content to a file, creating it if it doesn't exist, overwriting if it does.

**Passive Triggers:**
- Code analysis re-runs
- Tests re-run (debounced 2s)
- Documentation hash check (auto-updates if stale)

**Parameters:**
- `file_path` (string, required): Path to file
- `content` (string, required): Content to write
- `create_directories` (boolean, optional): Create parent directories if needed (default: true)

**Returns:** Success message with bytes written

**Example:**
```json
{
  "file_path": "pkg/utils/helper.go",
  "content": "package utils\\n\\nfunc Helper() {}\\n",
  "create_directories": true
}
```

**Use Cases:**
- Create new files
- Replace entire file contents
- Generate code files

---

### 3. list_files

List files in a directory with optional glob pattern matching and recursive search.

**Parameters:**
- `directory` (string, optional): Directory to search (default: ".")
- `pattern` (string, optional): Glob pattern (default: "*")
- `recursive` (boolean, optional): Search subdirectories (default: false)

**Returns:** List of matching file paths

**Example:**
```json
{
  "directory": "internal/services",
  "pattern": "*.go",
  "recursive": true
}
```

**Glob Patterns:**
- `*.py` - All Python files
- `test_*.py` - Files starting with "test_"
- `*_test.go` - Files ending with "_test.go"

**Use Cases:**
- Find all files of a certain type
- Discover project structure
- Locate test files

---

### 4. line_edit_file

Edit specific lines in a file by line number (1-indexed, inclusive).

**Passive Triggers:**
- Code analysis re-runs
- Tests re-run (debounced 2s)
- Documentation hash check (auto-updates if stale)

**Parameters:**
- `file_path` (string, required): Path to file
- `start_line` (number, required): Starting line number (1-indexed)
- `end_line` (number, required): Ending line number (1-indexed)
- `new_content` (string, required): Replacement content (can contain newlines)

**Returns:** Success message with lines replaced count

**Example:**
```json
{
  "file_path": "main.go",
  "start_line": 10,
  "end_line": 12,
  "new_content": "\\t// Updated comment\\n\\tfmt.Println(\\"Hello, World!\\")"
}
```

**Behavior:**
- Lines `start_line` through `end_line` (inclusive) are replaced
- `new_content` can be multi-line (split on `\\n`)
- Line numbers are 1-indexed (first line is 1, not 0)

**Use Cases:**
- Precise edits when line numbers are known
- Replace function bodies
- Update specific code blocks

**⚠️ Warning:** Line numbers change after file modifications. Use with caution in multi-step edits.

---

### 5. find_and_replace_edit_file

Find exact text matches and replace them in a file.

**Passive Triggers:**
- Code analysis re-runs
- Tests re-run (debounced 2s)
- Documentation hash check (auto-updates if stale)

**Parameters:**
- `file_path` (string, required): Path to file
- `find_text` (string, required): Exact text to find (must match exactly, including whitespace)
- `replace_text` (string, required): Replacement text
- `replace_all` (boolean, optional): Replace all occurrences vs. first only (default: false)

**Returns:** Success message with occurrence count

**Example:**
```json
{
  "file_path": "config.go",
  "find_text": "localhost:8080",
  "replace_text": "localhost:3000",
  "replace_all": true
}
```

**Behavior:**
- `find_text` must match **exactly** (case-sensitive, whitespace-sensitive)
- If `replace_all` is false, only the first occurrence is replaced
- Returns error if `find_text` is not found

**Use Cases:**
- Rename variables or functions
- Update configuration values
- Change import paths
- Batch replacements

---

## Part 2: Code Analysis Tools

babyCoder includes **passive background analysis** similar to an IDE's language server. The analyzer runs automatically after file edits and provides active tools for querying code status.

### How It Works

1. **Passive Analysis (Background):**
   - Runs automatically on startup
   - Re-runs asynchronously after any file is modified
   - Executes a user-supplied `build_command` shell command (for example
     `cargo check`, `npm run build`, `tsc --noEmit`,
     `pytest --collect-only`, or `go build ./...`)
   - Captures stdout, stderr, and the exit code
   - Uses the AI provider to summarize the raw output into a strict JSON
     pass/fail report with per-file diagnostics
   - Results stored in memory for instant querying

2. **Active Tools (On-Demand):**
   - Agent can query current code status anytime
   - Fresh analysis runs if needed
   - Get diagnostics for specific files

---

### 6. check_code_status

Get a summary of all errors, warnings, and issues in the project.

**Parameters:**
- `include_warnings` (boolean, optional): Include warning-level diagnostics (default: true)
- `max_diagnostics` (integer, optional): Max diagnostics per severity level (default: 20)

**Returns:** Formatted report with errors and warnings

**Example:**
```json
{
  "include_warnings": true,
  "max_diagnostics": 10
}
```

**Output Format:**
```
=== Project Code Status ===

Summary: 2 error(s), 1 warning(s), 0 other issue(s)

=== ERRORS ===

internal/services/agent/agent.go:45:12
  [build] undefined: nonExistentFunc

main.go:102:5
  [build] expected ')', found 'EOF'

=== WARNINGS ===

internal/storage/database.go:234:2
  [build] this value of err is never used
```

Each diagnostic carries `file_path`, `line`, `column`, `severity`,
`message`, and `source` fields, as extracted by the AI provider from
the raw build output.

**Use Cases:**
- Check if project builds before committing
- See all issues at a glance after making changes
- Verify fixes resolved errors

---

### 7. get_file_diagnostics

Get detailed diagnostics for a specific file.

**Parameters:**
- `file_path` (string, required): Relative path to the file

**Returns:** All diagnostics for that file with line/column info

**Example:**
```json
{
  "file_path": "internal/services/agent/agent.go"
}
```

**Output Format:**
```
=== Diagnostics for internal/services/agent/agent.go ===

Total issues: 3

✗ Line 45, Column 12 [error]
  Source: build
  undefined: nonExistentFunc

⚠ Line 67, Column 2 [warning]
  Source: build
  this value of err is never used

✗ Line 89, Column 15 [error]
  Source: build
  expected '}', found 'EOF'
```

**Use Cases:**
- Focus on issues in a specific file you're editing
- See all problems before fixing them
- Understand parse errors with line numbers

---

### 8. get_project_structure

Walk the project file tree language-agnostically and display its
directory and file structure. This tool no longer parses ASTs or knows
anything about packages — it operates purely on the filesystem.

Noise directories are skipped automatically: `.git`, `node_modules`,
`vendor`, `target`, `dist`, `build`, `__pycache__`, common virtual
environment folders (`.venv`, `venv`, `env`), and IDE folders
(`.idea`, `.vscode`).

**Parameters:**
- `max_depth` (number, optional): Maximum directory depth to traverse (default: 5)
- `include_hidden` (boolean, optional): Include dotfiles and dot-directories (default: false)
- `max_entries` (number, optional): Maximum total entries to report before truncating (default: 500)

**Returns:** A formatted tree of directories and files

**Example:**
```json
{
  "max_depth": 5,
  "include_hidden": false,
  "max_entries": 500
}
```

**Use Cases:**
- Get a bird's eye view of the project layout
- Quickly locate where files live
- Onboard to an unfamiliar codebase

---

## Part 3: Test Execution Tools (Passive System)

babyCoder includes **automatic test execution** similar to an IDE's test runner. Tests run passively in the background after code changes.

### How It Works

1. **Passive Execution (Background):**
   - Runs automatically on startup
   - **Triggered after agent completes all tool calls** (not time-based)
   - File edits mark tests as "dirty" via `MarkDirty()`
   - Tests run once when agent finishes its turn
   - Executes a user-supplied `test_command` shell command (for example
     `pytest`, `npm test`, `cargo test`, `mvn test`, or `go test ./...`)
   - Captures stdout, stderr, and the exit code
   - Uses the AI provider to extract a pass/fail summary plus the
     per-test failure details from the raw output
   - Results cached in memory for instant querying

2. **Active Tools (On-Demand):**
   - Agent can check test status anytime
   - Get detailed failure information
   - Force immediate test run (bypass dirty flag)

---

### 9. get_test_status

Get a quick summary of test results including pass/fail counts and timing.

**Parameters:** None

**Returns:** Formatted test status summary

**Example:**
```json
{}
```

**Output (all passing):**
```
=== Test Status ===

✓ ALL TESTS PASSING

Total:    34 tests
Passed:   34 (100.0%)
Failed:   0
Duration: 2.14s
Last run: 2024-01-15 10:30:00
```

**Output (with failures):**
```
=== Test Status ===

✗ 2 TEST(S) FAILING

Total:    34 tests
Passed:   32 (94.1%)
Failed:   2
Duration: 2.18s
Last run: 2024-01-15 10:30:15

ℹ Use 'get_failing_tests' to see failure details.
```

**Use Cases:**
- Quick health check after making changes
- Verify all tests pass before committing
- See test timing and coverage

---

### 10. get_failing_tests

Get detailed information about all currently failing tests with error messages.

**Parameters:**
- `package_filter` (string, optional): Filter to specific package

**Returns:** Detailed failure information grouped by package

**Example:**
```json
{
  "package_filter": "github.com/exar/babycoder/internal/services/agent"
}
```

**Output:**
```
=== Failing Tests (2) ===

Package: github.com/exar/babycoder/internal/services/agent
------------------------------------------------------------

✗ TestAgentRun (0.05s)
  agent_test.go:45: Expected nil error, got: undefined function
  agent_test.go:46: Stack trace:
    Run() at agent.go:67
    TestAgentRun() at agent_test.go:45
  
✗ TestRegisterTool (0.02s)
  agent_test.go:78: Tool not registered
  agent_test.go:79: Expected tool count: 5, got: 4
```

**Use Cases:**
- Debug test failures
- Understand what needs to be fixed
- Focus on specific package issues

---

### 11. run_tests

Force immediate test execution, bypassing the dirty flag check.

**Parameters:**
- `package_filter` (string, optional): Run tests only for specific package (e.g., `./internal/services/agent`)

**Returns:** Test run summary

**Example:**
```json
{
  "package_filter": "./internal/services/tools"
}
```

**Output:**
```
=== Test Run Complete ===

✓ ALL TESTS PASSED

Total:    13 tests
Passed:   13
Failed:   0
Duration: 0.85s
```

**Use Cases:**
- Get immediate test feedback after a fix
- Verify a specific package before moving on
- Run tests manually without waiting for agent completion

---

## Part 4: Automatic Documentation System

babyCoder includes a **fully automatic documentation system** that uses LLM workers to keep Go documentation fresh when code changes.

### How It Works (Completely Passive)

1. **Hash-Based Tracking:**
   - SHA256 hash of function signature vs doc comment
   - Stored in SQLite for persistence across sessions
   - Automatic staleness detection

2. **Async LLM Workers (2 workers):**
   - When signature changes but doc doesn't → queue update job
   - Worker calls local LLM to generate fresh documentation
   - Auto-applies new doc to file
   - Re-hashes to mark fresh

3. **Agent Experience:**
   - Agent edits code → docs update automatically in background (2-5 seconds)
   - Agent never explicitly manages documentation
   - Documentation is always accurate and up-to-date

### Architecture Flow

```
Agent edits function signature
    ↓
Hash changes: abc123 → def456
    ↓
Stale detected → Queue LLM job (background)
    ↓
LLM worker generates new doc
    ↓
Auto-apply to file → Re-hash → Fresh!
```

### No Active Tools Needed

The documentation system is **fully passive** - there are no tools to call. The agent simply edits code, and documentation stays current automatically through the LLM workers.

### What Gets Tracked

- All exported functions (including methods)
- All exported structs
- All exported interfaces
- Function signature changes
- Parameter additions/removals
- Return type changes

### Database Tables

**`documentation_hashes`:**
- Tracks signature hash vs doc hash per symbol
- Flags stale docs when signature changes
- Indexed for fast lookups

**`documentation_updates`:**
- Queue of pending/processing/completed jobs
- Full audit trail of doc changes
- Error tracking for failed LLM calls

### Example Workflow

```
1. Agent: write_file("agent.go", [modified Run function signature])
2. [Background, 0.1s]: Hash check detects signature change
3. [Background, 0.2s]: Queue LLM job for Run function
4. [Background, 3s]: LLM generates new documentation
5. [Background, 3.1s]: Auto-apply doc to agent.go
6. [Background, 3.2s]: Re-hash → mark fresh
7. Agent: [continues working, never knew docs were updated]
```

### Benefits for Local Models

- **No rate limits:** Hammer the local LLM constantly
- **No cost:** CPU cycles are free
- **Always fresh:** Impossible for docs to drift
- **Zero overhead:** Agent never thinks about documentation

---

## Passive Analysis Behavior

### When Passive Systems Run

1. **On Startup:**
   - Code analysis runs in background
   - Tests run in background
   - Message: "Analyzing project and running tests..."

2. **After File Edits (for .go files):**
   - All three systems triggered: analysis, tests, doc tracking
   - Code analysis: immediate (async)
   - Tests: marked as "dirty", **run when agent completes**
   - Doc tracking: immediate hash check → queue LLM jobs
   - Analysis and docs run asynchronously (don't block agent)
   - Tests run synchronously after agent's final response

3. **On Demand:**
   - `check_code_status`: runs analysis synchronously if stale
   - `get_test_status`: returns cached results (or waits if running)
   - `run_tests`: forces immediate test execution
   - Documentation: fully automatic, no manual trigger

### What Gets Analyzed/Tested

**Code Analysis:**
- Runs the user-supplied `build_command` (for example `cargo check`,
  `npm run build`, `tsc --noEmit`, or `go build ./...`)
- Captures stdout, stderr, and the exit code
- Uses the AI provider to extract structured pass/fail diagnostics
  (file, line, column, severity, message, source) from the raw output

**Test Execution:**
- Runs the user-supplied `test_command` (for example `pytest`,
  `npm test`, `cargo test`, `mvn test`, or `go test ./...`)
- Triggered when agent completes all tool calls
- Runs whenever the dirty flag is set (any file edit sets it; the
  command from the last invocation is reused on the next run)
- AI provider extracts pass/fail counts and per-test failure details
  from the captured output
- Captures failure output and timing

**Documentation Tracking:**
- Hashes all exported function signatures
- Compares to doc comment hashes
- Queues LLM jobs for stale docs
- Auto-applies generated documentation

### Performance

- **In-Memory:** Analysis results and test results cached in RAM
- **SQLite:** Documentation hashes persisted for cross-session tracking
- **Fast:** Go compiles quickly, even for large projects
- **Async:** File edits don't block waiting for background systems
- **Smart Trigger:** Tests run exactly once per agent turn (not per file edit)
- **Batched:** Multiple file edits in one turn = one test run
- **Concurrent:** Multiple file edits trigger single analysis (no duplicate work)

---

## Testing

### File Operations

All file operation tools have comprehensive test coverage:

```bash
go test ./internal/services/tools -v
```

**Test Coverage:**
- ✓ Read file (basic, non-existent, path traversal)
- ✓ Write file (basic, nested directories)
- ✓ List files (basic, pattern matching, recursive)
- ✓ Line edit (single line, multiple lines)
- ✓ Find and replace (first match, all matches, not found)

**Total: 13 tests, all passing**

### Code Analysis

The analyzer service can be tested independently:

```bash
go test ./internal/services/analyzer -v
```

Integration tests for code analysis tools are included in the tools test suite.

### Test Runner

The test runner service can be tested independently:

```bash
go test ./internal/services/testrunner -v
```

### Documentation Tracker

The doc tracker service can be tested independently:

```bash
go test ./internal/services/doctracker -v
```

---

## Complete Tool List

**File Operations (5 tools):**
1. `read_file` - Read file contents (with optional doc summary)
2. `write_file` - Write/create files (triggers passive systems)
3. `list_files` - List files with patterns
4. `line_edit_file` - Edit specific lines (triggers passive systems)
5. `find_and_replace_edit_file` - Find and replace text (triggers passive systems)

**Code Analysis (3 tools):**
6. `check_code_status` - Project-wide error/warning summary
7. `get_file_diagnostics` - Per-file detailed diagnostics
8. `get_project_structure` - Walk the project file tree

**Test Execution (3 tools):**
9. `get_test_status` - Quick pass/fail summary
10. `get_failing_tests` - Detailed failure information
11. `run_tests` - Force immediate test run

**Total: 10 tools available to the agent**

**Passive Systems (no tools needed):**
- Automatic documentation updates via LLM workers
- Background code analysis after edits
- Background test execution after edits

---

## Passive System Integration

### Complete Workflow Example

```
Agent: "Add an 'input' parameter to the Run function"

[Agent calls write_file or line_edit_file]

[Immediate, background]:
  • Code analyzer starts (0.5s)
  • Doc tracker: "Run signature changed" → queue LLM job
  • Test runner: Mark as dirty (needs run)

[Agent continues with tool calls...]
[Agent finishes all tool calls]

[Now - synchronous]:
  • Tests execute: (your configured test command, e.g. `pytest`, `npm test`) (1-3s)
  • Results parsed and cached

[Background - async]:
  • LLM worker picks up doc job (3s)
  • Generates new documentation for Run function
  • Auto-applies to agent.go
  • Re-hashes → marks fresh (5s total)

Agent: "Check if everything looks good"
[check_code_status] → ✓ No errors
[get_test_status] → ✓ All 34 tests passing (uses cached results)
[Docs are fresh!]
```

**The agent never explicitly:**
- Ran tests (happened automatically after tool calls)
- Generated documentation (LLM workers in background)
- Checked for staleness (hash tracking is automatic)

**Everything happened at the perfect time:**
- Tests: After all edits, before showing results
- Docs: In background, ready for next read
- Analysis: Immediately for instant feedback

---

## Security Features

### 1. Path Containment

All file paths are resolved and verified to be within the project root.

### 2. Validation

- File paths are required and validated
- Line numbers are validated against file length
- Find text must exist before replacement

### 3. Safe Defaults

- `create_directories: true` - Prevents errors
- `replace_all: false` - Conservative default
- `recursive: false` - Prevents accidental large traversals

---

## Part 5: Bash Execution Tools

### 12. bash_execute

Execute a bash command synchronously and return its output. Use for quick commands that complete in seconds (max 30s timeout, configurable up to 300s).

**NOT suitable for:**
- Long-running processes (web servers, daemons) - these need background process management
- Interactive commands that wait for input
- Commands that require TTY allocation

**Parameters:**
- `command` (string, required): The bash command to execute (e.g., 'ls -la', 'pytest', 'npm test', 'curl http://localhost:8080')
- `working_dir` (string, optional): Working directory (relative to project root or absolute). Defaults to project root.
- `timeout_seconds` (number, optional): Timeout in seconds (default: 30, max: 300)

**Returns:** Command output (stdout + stderr), or error message with partial output

**Example (simple):**
```json
{
  "command": "ls -la"
}
```

**Example (with directory and timeout):**
```json
{
  "command": "pytest -v tests/services",
  "working_dir": ".",
  "timeout_seconds": 60
}
```

**Example (check server):**
```json
{
  "command": "curl -s http://localhost:8080/health",
  "timeout_seconds": 5
}
```

**Use Cases:**
- Run tests with custom flags
- Check build status (e.g. `cargo check`, `npm run build`, `tsc --noEmit`)
- Query APIs with curl
- Run git commands (`git status`, `git diff`)
- Execute custom scripts
- Check dependencies (e.g. `npm install`, `pip check`, `cargo check`)
- Run linters/formatters (e.g. `prettier`, `ruff`, `eslint`, `gofmt`)
- System information (`ps aux | grep go`, `df -h`)

**Output Format:**
```
Command succeeded:
<stdout output>
--- STDERR ---
<stderr output if any>
```

Or on error:
```
Command failed:
<stdout output>
--- STDERR ---
<stderr output>
Error: <error message>
```

**Timeout Behavior:**
- Command is killed after timeout
- Partial output is returned
- Error indicates timeout occurred

**Security:**
- Runs with same user permissions as babyCoder
- No automatic privilege escalation
- Working directory must be within project root (if not absolute)
- Standard shell injection risks apply - agent should be careful with user input

---

### 13. process_start

Launch a long-running background process (web server, daemon, watcher). The process runs independently and you can poll its output later with process_logs.

**Parameters:**
- `process_id` (string, required): Unique identifier for this process (e.g., 'webserver', 'api', 'worker')
- `name` (string, required): Human-readable name for the process
- `command` (string, required): The command to execute (e.g., 'go', 'node', './myapp', 'python3')
- `args` (array of strings, optional): Command arguments (e.g., ['run', 'main.go'], ['server.js'])
- `working_dir` (string, optional): Working directory (relative to project root or absolute). Defaults to project root.

**Returns:** Success message with PID, process ID, and usage instructions

**Example (Go web server):**
```json
{
  "process_id": "api",
  "name": "API Server",
  "command": "go",
  "args": ["run", "cmd/api/main.go"],
  "working_dir": "."
}
```

**Example (Node.js app):**
```json
{
  "process_id": "webapp",
  "name": "Web Application",
  "command": "node",
  "args": ["server.js"],
  "working_dir": "./frontend"
}
```

**Use Cases:**
- Start a web server for testing
- Launch a background worker/daemon
- Run a file watcher
- Start a development server
- Launch a database or message queue

**Output Capture:**
- Last 1000 lines buffered automatically
- Both stdout and stderr captured
- Timestamps added to each line
- Use `process_logs` to retrieve output

---

### 14. process_status

Check the status of a background process or list all managed processes. Shows PID, status, uptime, and restart count.

**Parameters:**
- `process_id` (string, optional): Process ID to check. If omitted, lists all managed processes.

**Returns:** Process status details or list of all processes

**Example (single process):**
```json
{
  "process_id": "api"
}
```

**Example (list all):**
```json
{}
```

**Output (single process):**
```
Process: api
  Name: API Server
  Status: running
  PID: 12345
  Command: go
  Working Dir: /path/to/project
  Start Time: 2026-05-11T14:30:00Z
  Uptime: 5m30s
  Restart Count: 0
```

**Output (all processes):**
```
Managed Processes (2 total):

  [api] API Server
    Status: running | PID: 12345 | Uptime: 5m30s
    Command: go

  [worker] Background Worker
    Status: running | PID: 12346 | Uptime: 3m15s
    Command: python3
```

**Use Cases:**
- Check if a server is still running
- Get PID for debugging
- See how long a process has been running
- List all background processes

---

### 15. process_logs

Retrieve recent output (stdout/stderr) from a background process. Useful for checking web server responses, error messages, etc.

**Parameters:**
- `process_id` (string, required): The process ID to retrieve logs from
- `lines` (number, optional): Number of recent lines to retrieve (default: 50, max: 1000)

**Returns:** Recent log lines with timestamps

**Example:**
```json
{
  "process_id": "api",
  "lines": 100
}
```

**Output:**
```
Recent logs from api (last 100 lines):

[14:30:05][stdout] Starting server on :8080
[14:30:05][stdout] Database connected
[14:30:06][stdout] Listening on http://localhost:8080
[14:30:10][stdout] GET /health 200 2ms
[14:30:15][stdout] GET /api/users 200 15ms
[14:30:20][stderr] WARN: Slow query detected (230ms)
```

**Use Cases:**
- Check web server startup messages
- Debug errors from background processes
- Monitor application logs
- Verify server is responding correctly
- Track API requests

---

### 16. process_stop

Stop a running background process. The process will be killed immediately.

**Parameters:**
- `process_id` (string, required): The process ID to stop

**Returns:** Success message

**Example:**
```json
{
  "process_id": "api"
}
```

**Use Cases:**
- Stop a server before making changes
- Clean up background processes
- Free up resources
- Stop a misbehaving process

**Note:** The process is killed (SIGKILL), not gracefully stopped. For graceful shutdown, implement signal handling in your application.

---

### 17. process_restart

Restart a background process. The process will be stopped and started again with the same configuration.

**Parameters:**
- `process_id` (string, required): The process ID to restart

**Returns:** Success message with new PID and restart count

**Example:**
```json
{
  "process_id": "api"
}
```

**Output:**
```
✓ Process api restarted successfully
  New PID: 12347
  Restart Count: 1
  Status: running
```

**Use Cases:**
- Apply code changes to a running server
- Recover from a crashed process
- Reset application state
- Apply configuration changes

**Note:** Restart count is tracked and persisted in the database.

---

## Complete Workflow Examples

### Example 1: Web Server Development with Hot Reload

Use a language-appropriate watcher under `process_start` (e.g. `air` for
Go, `nodemon` for Node.js, `cargo watch` for Rust, `watchexec` for
anything).

```
1. Start the watcher as a managed process:
   process_start(
     process_id='api',
     name='API Hot Reload',
     command='watchexec',
     args=['-r', '-e', 'py', '--', 'python', 'app.py']
   )

2. Make code changes
   (the watcher rebuilds and restarts automatically)

3. Check logs:
   process_logs(process_id='api', lines=50)

4. Test endpoints:
   bash_execute(command='curl http://localhost:8080/health')

5. When done:
   process_stop(process_id='api')
```

### Example 2: Background Worker

```
1. Start worker:
   process_start(
     process_id='worker',
     name='Data Processor',
     command='go',
     args=['run', 'cmd/worker/main.go']
   )

2. Check it's running:
   process_status(process_id='worker')

3. Monitor logs:
   process_logs(process_id='worker', lines=100)

4. Make code changes, then restart:
   process_restart(process_id='worker')

5. Stop when done:
   process_stop(process_id='worker')
```

### Example 3: Multi-Service Architecture

```
1. Start API server:
   process_start(
     process_id='api',
     name='API Server',
     command='go',
     args=['run', 'cmd/api/main.go']
   )

2. Start frontend:
   process_start(
     process_id='frontend',
     name='Frontend Dev Server',
     command='npm',
     args=['run', 'dev'],
     working_dir='./frontend'
   )

3. Check all services:
   process_status()

4. Monitor API logs:
   process_logs(process_id='api')

5. Test communication:
   bash_execute(command='curl http://localhost:3000')
```

---

## Tool Count Summary

- **File Operations:** 5 tools (read_file, write_file, list_files, line_edit_file, find_and_replace_edit_file)
- **Code Analysis:** 3 tools (check_code_status, get_file_diagnostics, get_project_structure)
- **Test Execution:** 3 tools (get_test_status, get_failing_tests, run_tests)
- **Bash/Process Management:** 6 tools (bash_execute, process_start, process_status, process_logs, process_stop, process_restart)
- **Total Active Tools:** 17
- **Passive Systems:** 3 (code analyzer, test runner, doc tracker)

---
