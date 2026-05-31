# Storage Service - Session and Chat History

The storage service provides persistent storage for agent sessions, conversation history, and tool execution tracking using SQLite.

## Database Schema

### Tables

#### sessions
Stores information about agent conversation sessions.

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT PRIMARY KEY | Unique session identifier (UUID) |
| project_root | TEXT | Project directory path |
| created_at | TIMESTAMP | Session creation time |
| updated_at | TIMESTAMP | Last activity time |
| title | TEXT | Session title (truncated prompt) |
| status | TEXT | Session status: active, completed, failed |

#### messages
Stores individual messages in conversations.

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER PRIMARY KEY | Auto-increment message ID |
| session_id | TEXT | Foreign key to sessions.id |
| role | TEXT | Message role: system, user, assistant, tool |
| content | TEXT | Message content |
| tool_calls | TEXT | JSON string of tool calls (if any) |
| tool_call_id | TEXT | Tool call ID for tool response messages |
| message_index | INTEGER | Order within session |
| created_at | TIMESTAMP | Message creation time |
| tokens_prompt | INTEGER | Token count for prompt |
| tokens_response | INTEGER | Token count for response |

#### tool_executions
Tracks tool execution details and performance.

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER PRIMARY KEY | Auto-increment execution ID |
| session_id | TEXT | Foreign key to sessions.id |
| message_id | INTEGER | Foreign key to messages.id |
| tool_name | TEXT | Name of executed tool |
| arguments | TEXT | JSON string of tool arguments |
| result | TEXT | Tool execution result |
| success | BOOLEAN | Whether execution succeeded |
| error_msg | TEXT | Error message (if failed) |
| execution_ms | INTEGER | Execution time in milliseconds |
| created_at | TIMESTAMP | Execution time |

### Indexes

- `idx_messages_session`: Fast message retrieval by session
- `idx_messages_order`: Ordered message retrieval
- `idx_tool_executions_session`: Fast tool execution lookup
- `idx_sessions_updated`: Recent sessions listing
- `idx_sessions_project`: Project-specific session filtering

## API

### Database Operations

```go
// Create new database connection
db, err := storage.NewDatabase("/path/to/.babycoder/babycoder.db")
defer db.Close()

// Create a session
session := &storage.Session{
    ID:          uuid.New().String(),
    ProjectRoot: "/path/to/project",
    CreatedAt:   time.Now(),
    UpdatedAt:   time.Now(),
    Title:       "Implement user authentication",
    Status:      "active",
}
err = db.CreateSession(session)

// Save a message
message := &storage.Message{
    SessionID:    sessionID,
    Role:         "user",
    Content:      "Add validation to CreateUser",
    MessageIndex: 0,
    CreatedAt:    time.Now(),
}
err = db.SaveMessage(message)

// Save tool execution
execution := &storage.ToolExecution{
    SessionID:   sessionID,
    MessageID:   messageID,
    ToolName:    "extract_function",
    Arguments:   `{"file": "user.go", "name": "CreateUser"}`,
    Result:      "func CreateUser(...) { ... }",
    Success:     true,
    ExecutionMs: 150,
    CreatedAt:   time.Now(),
}
err = db.SaveToolExecution(execution)
```

### Query Operations

```go
// Get a session
session, err := db.GetSession(sessionID)

// List recent sessions
sessions, err := db.ListSessions("/path/to/project", 20)

// Get all messages for a session
messages, err := db.GetMessages(sessionID)

// Get tool executions for a session
executions, err := db.GetToolExecutions(sessionID)

// Get session statistics
stats, err := db.GetSessionStats(sessionID)
// Returns: message_count, tool_execution_count, total_tokens, etc.

// Update session
session.Status = "completed"
session.UpdatedAt = time.Now()
err = db.UpdateSession(session)

// Delete session (cascades to messages and executions)
err = db.DeleteSession(sessionID)
```

## CLI Commands

### List Sessions

```bash
./babyCoder sessions list
```

Shows recent sessions with:
- Session ID
- Title
- Status
- Created/Updated timestamps
- Message count, tool executions, token usage

### Show Session Details

```bash
./babyCoder sessions show <session_id>
```

Displays complete session information:
- Metadata (title, status, timestamps)
- Statistics (messages, tools, tokens)
- Full conversation history
- Tool execution details with timing

### Delete Session

```bash
./babyCoder sessions delete <session_id>
```

Permanently removes a session and all associated data.

## Integration with Agent

The agent service automatically integrates with the storage layer:

```go
// Create database
db, err := storage.NewDatabase(".babycoder/babycoder.db")

// Create agent with database
agent := agent.NewAgent(provider, config, db)

// Set session ID
agent.SetSessionID(sessionID)

// All messages and tool executions are automatically saved
agent.AddUserMessage("Hello")
agent.Run(ctx, toolExecutor)  // Saves all interactions

// Load previous session
agent.LoadSession(sessionID)  // Restores message history
```

## Storage Location

By default, application state is kept inside the `.babycoder/` directory at the project root:

```
/path/to/project/
└── .babycoder/
    ├── babycoder.json   # Configuration
    └── babycoder.db     # SQLite database
```

## Benefits

1. **Session Persistence**: Resume conversations across invocations
2. **Audit Trail**: Complete history of all agent actions
3. **Performance Tracking**: Tool execution timing and success rates
4. **Token Accounting**: Track API usage and costs
5. **Debugging**: Review past interactions to improve agent behavior
6. **Analytics**: Aggregate statistics across sessions

## Example Workflow

```bash
# Initialize project
./babyCoder init

# Run agent (creates session)
./babyCoder agent "Add input validation to user registration"
# Output: Session ID: abc-123-def-456

# View session details
./babyCoder sessions show abc-123-def-456

# Continue work in new session
./babyCoder agent "Now add unit tests for validation"

# Review all sessions
./babyCoder sessions list

# Clean up old sessions
./babyCoder sessions delete abc-123-def-456
```

## Future Enhancements

- Session tagging and search
- Export sessions to markdown/JSON
- Session replay for debugging
- Cost tracking per session
- Multi-project session management
- Session templates and presets
