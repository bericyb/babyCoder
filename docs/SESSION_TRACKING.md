# Session Tracking - Complete Audit Trail

babyCoder tracks **every detail** of agent sessions for full reproducibility and debugging.

## What We Track

### 1. Sessions Table

Every conversation session:

| Field | Type | Description |
|-------|------|-------------|
| `id` | UUID | Unique session identifier |
| `project_root` | string | Project directory path |
| `created_at` | timestamp | When session started |
| `updated_at` | timestamp | Last activity time |
| `title` | string | Session description |
| `status` | string | active, completed, failed |

**Use Case:** Manage and organize agent conversations

---

### 2. Messages Table

Every message in the conversation:

| Field | Type | Description |
|-------|------|-------------|
| `id` | int64 | Auto-increment message ID |
| `session_id` | UUID | Parent session |
| `role` | string | system, user, assistant, tool |
| `content` | text | Message text content |
| `tool_calls` | JSON | LLM's tool call requests |
| `tool_call_id` | string | Response to specific tool call |
| `message_index` | int | Order in conversation |
| `created_at` | timestamp | When message was sent |
| `tokens_prompt` | int | Prompt token count |
| `tokens_response` | int | Response token count |

**Use Case:** Full conversation history with context

**Example:**
```
[1] system: You are a helpful coding assistant...
[2] user: Read the main.go file
[3] assistant: I'll read that file for you... (tool_calls: [{read_file...}])
[4] tool: package main\n\nimport... (tool_call_id: call_abc123)
[5] assistant: Here's what I found in main.go...
```

---

### 3. Tool Executions Table

Every tool call with full details:

| Field | Type | Description |
|-------|------|-------------|
| `id` | int64 | Auto-increment execution ID |
| `session_id` | UUID | Parent session |
| `message_id` | int64 | Assistant message that requested this |
| `tool_name` | string | Tool that was executed |
| `arguments` | JSON | Full parameters passed to tool |
| `result` | text | Tool's return value |
| `file_path` | string | File affected (if applicable) |
| `success` | boolean | Whether execution succeeded |
| `error_msg` | text | Error details (if failed) |
| `execution_ms` | int64 | Execution time in milliseconds |
| `created_at` | timestamp | When tool was executed |

**Use Case:** Audit trail of all file operations and tool usage

**Example:**
```json
{
  "id": 1,
  "tool_name": "read_file",
  "arguments": "{\"file_path\": \"main.go\"}",
  "result": "package main\n\nimport ...",
  "file_path": "main.go",
  "success": true,
  "execution_ms": 5
}
```

---

## Complete Tracking Flow

When a tool is called, we capture:

1. **Before Execution:**
   - Assistant's decision to call the tool (stored in `messages.tool_calls`)
   - Tool name and all parameters
   - Link to the assistant message that requested it

2. **During Execution:**
   - Start time
   - Which file is being accessed (if applicable)

3. **After Execution:**
   - End time (execution duration calculated)
   - Success/failure status
   - Full result or error message
   - Tool response message back to LLM

4. **Context Links:**
   - Tool execution → Assistant message (via `message_id`)
   - Tool execution → Session (via `session_id`)
   - Tool response message → Tool call (via `tool_call_id`)

---

## Session Detail View

When you run `./babyCoder sessions show <session-id>`, you see:

```
=== Session abc-123-def-456 ===

Title:        Interactive session
Status:       completed
Project Root: /Users/exar/Projects/babyCoder
Created:      2024-01-15 10:30:00
Updated:      2024-01-15 10:35:00

=== Statistics ===
Messages:        10
Tool Executions: 3
Prompt Tokens:   450
Response Tokens: 820
Total Tokens:    1270

=== Conversation ===

[1] system:
You are a helpful coding assistant...

[2] user:
Read the main.go file

[3] assistant:
I'll read that file for you.
  (Has tool calls)

[4] tool:
package main

import (
  "fmt"
...

[5] assistant:
Here's what I found in main.go...

=== Tool Executions ===

[1] ✓ read_file (5ms)
    File: main.go
    Arguments: {"file_path": "main.go"}
    Result: package main\n\nimport ...

[2] ✓ write_file (12ms)
    File: hello.go
    Arguments: {"file_path": "hello.go", "content": "package main..."}
    Result: Successfully wrote 85 bytes to hello.go

[3] ✓ find_and_replace_edit_file (8ms)
    File: config.go
    Arguments: {"file_path": "config.go", "find_text": "localhost:8080", ...}
    Result: Successfully replaced 1 occurrence(s) in config.go
```

---

## Reproducibility

With this tracking, you can:

1. **Replay sessions** - See exactly what the agent did, step by step
2. **Debug failures** - Trace where and why something went wrong
3. **Audit file changes** - Know what files were modified and when
4. **Measure performance** - See tool execution times
5. **Track token usage** - Monitor API costs per session
6. **Understand decisions** - See the full context for each tool call

---

## Database Schema

```sql
CREATE TABLE sessions (
  id TEXT PRIMARY KEY,
  project_root TEXT NOT NULL,
  created_at TIMESTAMP,
  updated_at TIMESTAMP,
  title TEXT,
  status TEXT
);

CREATE TABLE messages (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id TEXT NOT NULL,
  role TEXT NOT NULL,
  content TEXT,
  tool_calls TEXT,  -- JSON array of tool call requests
  tool_call_id TEXT,
  message_index INTEGER,
  created_at TIMESTAMP,
  tokens_prompt INTEGER,
  tokens_response INTEGER,
  FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE TABLE tool_executions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id TEXT NOT NULL,
  message_id INTEGER NOT NULL,  -- Links to assistant message
  tool_name TEXT NOT NULL,
  arguments TEXT NOT NULL,      -- JSON object of parameters
  result TEXT,
  file_path TEXT,               -- File affected
  success BOOLEAN,
  error_msg TEXT,
  execution_ms INTEGER,
  created_at TIMESTAMP,
  FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
  FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE
);
```

---

## Automatic Migration

Existing databases are automatically migrated to add the `file_path` column:

```go
// Runs on startup
database.MigrateDatabase()
```

No manual intervention required!

---

## Privacy & Security

- **Local-only:** All data stored in `.babycoder/babycoder.db` on your machine
- **No external tracking:** Nothing sent to external services
- **Full control:** Delete sessions anytime with `./babyCoder sessions delete <id>`
- **Project-scoped:** Each project has its own database

---

## Example: Tracing a Bug

Imagine the agent made an incorrect edit. You can trace exactly what happened:

```bash
# View the session
./babyCoder sessions show abc-123

# See the tool execution that caused the issue
[3] ✓ find_and_replace_edit_file (8ms)
    File: config.go
    Arguments: {"find_text": "localhost:8080", "replace_text": "localhost:3000"}

# The full arguments are stored, so you can:
# 1. See exactly what the LLM requested
# 2. Understand why it made that choice (see conversation context)
# 3. Reproduce the issue
# 4. Fix the tool or prompt
```

---

## Statistics Tracking

Per-session metrics automatically calculated:

- Total messages exchanged
- Tool execution count
- Token usage (prompt + response)
- Success/failure rates
- Average execution time

Access via `GetSessionStats()` or the CLI.

---

## Future Enhancements

With this tracking foundation, we can add:

- **Session replay** - Rerun a session step-by-step
- **Export sessions** - Save to JSON/markdown for sharing
- **Analytics dashboard** - Visualize tool usage patterns
- **Cost tracking** - Calculate API costs per session
- **Diff viewing** - Show before/after for file edits
- **Session templates** - Save and reuse common workflows

All the data is already captured!
