# babyCoder

A specialized AI coding agent designed for multi-language development, optimized for smaller local language models through precision tooling and enhanced code locality.

## Quick Start

```bash
# Initialize in your project
./babyCoder init

# Start interactive mode (default)
./babyCoder

# Or view existing sessions
./babyCoder sessions list
```

## Overview

babyCoder addresses the fundamental challenge of enabling smaller, resource-efficient language models to write and edit code in any language with the same precision and reliability as larger frontier models. By providing purpose-built tools with fine-grained code locality and deep integration with a language's static analysis ecosystem, babyCoder makes professional-grade development accessible to local AI models.
**Primary Interface**: Interactive CLI - Just run `./babyCoder` to start chatting with the agent  
**Secondary Interface**: Optional HTTP API server for external agent integration

## Core Philosophy

Traditional AI coding agents struggle with precise code modifications because they operate at the file level, requiring models to understand entire files and regenerate large code blocks. babyCoder solves this by providing tools that match how developers actually think about and manipulate code: at the function, struct, method, and logical block level.

## Key Features

### 1. Hashline Editing System

Unlike traditional line-number-based editing (which breaks when code changes) or full-content matching (which requires large context windows), babyCoder implements a **hashline addressing system**:

- Each logical code block (function, struct, method, interface) is assigned a stable content-based hash
- Edits reference code by hash rather than line numbers, remaining valid across file changes
- Minimal context requirements: models only need to see the specific block being modified
- Enables precise, surgical edits without understanding the entire file structure

**Example:**
```
Instead of: "Edit lines 145-167 in user_service.go"
babyCoder: "Edit function hash abc123 to add validation"
```

### 2. Function and Struct Level Locality

babyCoder provides tools that operate at the natural boundaries of programming languages:

- **Extract Function Definition**: Retrieve a single function with its signature, documentation, and body
- **Extract Struct Definition**: Get a struct with all its fields, tags, and comments
- **Extract Method Set**: Retrieve all methods associated with a type
- **Extract Interface Definition**: Get interface contracts with all method signatures

These tools give models exactly the context they need, nothing more, nothing less.

### 3. Deep LSP Integration

babyCoder leverages the Golang Language Server Protocol (gopls) for intelligent code awareness:

- **Type Information**: Query the type of any expression or variable
- **Symbol Navigation**: Jump to definitions, find implementations, locate references
- **Code Actions**: Get contextually relevant suggestions for improvements
- **Diagnostics**: Real-time error and warning detection before compilation
- **Hover Information**: Retrieve documentation and type signatures on demand

### 4. Static Analysis Tooling

Integration with Golang's powerful static analysis tools:

- **go vet**: Detect suspicious constructs and common mistakes
- **staticcheck**: Advanced linting with hundreds of checks
- **Type Checker**: Validate type correctness without full compilation
- **Import Analysis**: Verify and organize imports automatically
- **Dead Code Detection**: Identify unused functions, variables, and types

### 5. Tool Calling Robustness

Small models often struggle with complex tool schemas. babyCoder addresses this through:

- **Simplified Tool Signatures**: Each tool does one thing well
- **Validation Layers**: Input validation with clear error messages
- **Guided Workflows**: Tools suggest next steps based on current context
- **Forgiving Parsing**: Graceful handling of malformed requests
- **Learning Feedback**: Error messages teach the model better tool usage

## Architecture

### Service Separation

babyCoder follows a structured monolith approach as an interactive CLI-first application:

```
babyCoder/
├── main.go                           # Interactive CLI entry point
├── internal/
│   ├── config/
│   │   └── config.go                # Configuration management (JSON-based)
│   ├── logging/
│   │   └── logger.go                # File-based logging service
│   ├── services/
│   │   ├── tools/                   # All code manipulation tools (planned)
│   │   │   ├── extraction.go       # Code block extraction
│   │   │   ├── editing.go          # Hashline-based code modification
│   │   │   ├── analysis.go         # Static analysis and LSP integration
│   │   │   ├── validation.go       # Syntax and type checking
│   │   │   └── hashline.go         # Content-based addressing system
│   │   ├── ai_provider/            # LLM integration (LMStudio, OpenAI spec)
│   │   │   ├── provider.go         # Provider implementation
│   │   │   └── types.go            # OpenAI-compatible types
│   │   └── agent/                  # Agent orchestration and tool calling
│   │       └── agent.go            # Agent loop with message persistence
│   └── storage/                    # SQLite persistence
│       └── database.go             # Sessions, messages, tool executions
└── .babycoder/
    ├── logs/                       # Timestamped log files (auto-cleanup)
    ├── babycoder.db                # SQLite database
    └── babycoder.json              # Project configuration
```

### Current Implementation Status

**✅ Completed (Phase 1):**
- Interactive CLI with STDIN/STDOUT
- Configuration system (JSON-based)
- AI provider service (LMStudio/OpenAI spec)
- Agent loop with tool calling support
- Session and chat history persistence (SQLite)
- File-based logging (no CLI pollution)
- Comprehensive test coverage (21 tests)

**🚧 In Progress:**
- Tools service architecture
- Hashline addressing system
- LSP integration

**📋 Planned:**
- Code extraction tools
- Code editing tools
- Static analysis integration

### Data Flow

**Interactive Mode (Primary)**
1. **User Input**: Types message at `You:` prompt
2. **Agent Service**: Processes input, maintains conversation context
3. **AI Provider**: Sends messages to LLM (LMStudio by default)
4. **Tool Execution**: If LLM requests tools, executes and returns results
5. **Storage**: Persists all messages and tool executions to SQLite
6. **Logging**: All internal logs written to `.babycoder/logs/`
7. **Response**: Clean output displayed to user, ready for next input

**Session Management:**
- Each interactive session gets a unique UUID
- All conversations persisted to SQLite
- Sessions can be listed, viewed, and deleted via `sessions` command
- Automatic tracking of messages, tool calls, and token usage

## Technology Stack

### Backend
- **Language**: [Target Language] (Self-hosted/Tooling specific)
- **Database**: SQLite with foreign key constraints (single-file, zero-config)
- **Logging**: File-based with automatic rotation and cleanup
- **LSP Client**: Integration with Language Server Protocol (LSP) client (Language-agnostic)
- **Parser**: `go/ast`, `go/parser`, `go/types` (planned)

### AI Provider Integration
- **Primary**: LMStudio (OpenAI-compatible API)
- **Local Models**: Ollama, llama.cpp, LocalAI (planned)
- **API Providers**: OpenAI, Anthropic (planned)
- **Protocol**: OpenAI chat completions with tool calling

### Storage & Persistence
- **Database**: SQLite with sessions, messages, and tool_executions tables
- **Migrations**: [goose](https://github.com/pressly/goose) with embedded SQL files (see [Database Migrations](#database-migrations))
- **Logging**: Timestamped files in `.babycoder/logs/` with 7-day retention
- **Configuration**: JSON-based (`.babycoder/babycoder.json`)
- **Session Management**: UUID-based sessions with full audit trail

## Tool Catalog

### Code Extraction Tools

- `extract_function`: Get a single function by name or hash
- `extract_struct`: Get a struct definition with all fields
- `extract_method_set`: Get all methods for a given type
- `extract_interface`: Get an interface definition
- `list_symbols`: Get all top-level symbols in a file or package

### Code Editing Tools

- `edit_function_by_hash`: Modify a function using its content hash
- `edit_struct_by_hash`: Modify a struct definition
- `insert_function`: Add a new function to a file
- `delete_function_by_hash`: Remove a function
- `rename_symbol`: Rename across all references (powered by LSP)

### Analysis Tools

- `get_type_at_position`: Query the type of an expression
- `find_references`: Locate all uses of a symbol
- `get_diagnostics`: Retrieve errors and warnings for a file
- `suggest_imports`: Get import suggestions for undefined symbols
- `detect_dead_code`: Find unused functions and variables

### Validation Tools

- `validate_syntax`: Check if code parses correctly
- `validate_types`: Run type checker on a file or package
- `run_vet`: Execute `go vet` and return issues
- `run_staticcheck`: Run staticcheck linter

## Design Principles

### For AI Agents
1. **Minimal Context**: Tools provide only what's needed for the task
2. **Stable Addressing**: Hashes don't change unless content changes
3. **Clear Errors**: Every error teaches the model how to correct itself
4. **Incremental Feedback**: Validate before committing changes
5. **Unified Interface**: All tools accessible through single service API

### For Human Developers
1. **CLI First**: Primary interaction through familiar command-line interface
2. **Full Transparency**: Verbose mode shows exactly what the agent is doing
3. **Rollback Capability**: Every change is versioned and reversible
4. **Override Controls**: Humans can intervene at any step via interactive mode
5. **Audit Logging**: Complete history of all agent actions stored in SQLite

## Usage

### Interactive Mode (Default)

Simply run babyCoder with no arguments to start an interactive session:

```bash
./babyCoder

╔════════════════════════════════════════════════════════════╗
║                   babyCoder Interactive                    ║
╚════════════════════════════════════════════════════════════╝

Model:    local-model
Endpoint: http://127.0.0.1:1234/v1

Type your message and press Enter to send.
Type /exit to quit.
Type /new to start a new session.

You: Hello! Can you help me with Golang?
```

**Interactive Commands:**
- `/exit` - Save session and quit
- `/new` - Start a fresh session
- `/help` - Show available commands

### Session Management

```bash
# List recent sessions
./babyCoder sessions list

# View session details with full conversation
./babyCoder sessions show <session-id>

# Delete a session
./babyCoder sessions delete <session-id>
```

### Project Initialization

```bash
# Create .babycoder/babycoder.json configuration file
./babyCoder init
```

### Configuration

Edit `.babycoder/babycoder.json` in your project root:

```json
{
  "ai_provider": {
    "type": "lmstudio",
    "endpoint": "http://127.0.0.1:1234/v1",
    "model": "local-model",
    "temperature": 0.2,
    "api_key": ""
  },
  "agent": {
    "max_iterations": 100,
    "verbose": false,
    "auto_commit": false
  },
  "project": {
    "root": "/path/to/project",
    "exclude_patterns": [
      "vendor/",
      "**/*_test.go",
      "**/*.pb.go"
    ]
  }
}
```

### Logging

All logs are automatically written to `.babycoder/logs/` directory:

```
.babycoder/
└── logs/
    ├── babycoder_2024-01-15_14-30-00.log
    ├── babycoder_2024-01-15_15-45-12.log
    └── ...
```

Logs older than 7 days are automatically cleaned up. The CLI output remains clean and user-friendly.

## Testing

babyCoder includes comprehensive test coverage for all core services:

```bash
# Run all tests
go test ./...

# Run with verbose output
go test ./... -v

# Run with coverage
go test ./... -cover

# Test specific package
go test ./internal/storage -v
```

**Test Coverage:**
- ✓ Config Service: 7 tests
- ✓ Logging Service: 6 tests  
- ✓ Storage Service: 8 tests
- **Total: 21 tests, all passing**

All core functionality is codified in tests to ensure reliability and maintainability.

## Database Migrations

babyCoder uses [goose](https://github.com/pressly/goose) (`github.com/pressly/goose/v3`) to manage SQLite schema migrations. Migration files live in `internal/storage/migrations/` and are embedded into the compiled binary via `//go:embed`, so the application has no external file-system dependency at runtime.

Goose maintains its own `goose_db_version` table inside the SQLite database to record which migrations have been applied. `Database.MigrateDatabase()` is invoked on every startup (from `NewDatabase` in `internal/storage/database.go`) and is idempotent — already-applied migrations are skipped automatically.

### Adding a new migration

1. Create a new file in `internal/storage/migrations/` following the sequential naming convention:

   ```
   00002_short_description_of_change.sql
   ```

   The numeric prefix must be strictly greater than the highest existing migration's prefix. Use a short, descriptive snake_case name (no abbreviations — see the project conventions in `AGENTS.md`).

2. Populate the file with `Up` and `Down` sections. Wrap each individual SQL statement in `StatementBegin` / `StatementEnd` markers — goose otherwise splits on semicolons, which breaks any statement containing an embedded semicolon (e.g. trigger bodies):

   ```sql
   -- +goose Up
   -- +goose StatementBegin
   ALTER TABLE sessions ADD COLUMN archived_at TIMESTAMP;
   -- +goose StatementEnd


   -- +goose Down
   -- +goose StatementBegin
   ALTER TABLE sessions DROP COLUMN archived_at;
   -- +goose StatementEnd
   ```

3. The `Down` section should reverse the `Up` section as faithfully as SQLite allows. Order `DROP` statements so that dependent objects (indexes, foreign-key children) are dropped before the objects they depend on.

4. Rebuild — `go:embed` will pick the new file up automatically. No Go code changes are required.

5. Run the test suite (`go test ./internal/storage/...`) to confirm the migration applies cleanly against a fresh database.

### Why goose, and what we no longer do

The earlier hand-rolled migration logic combined unconditional `ALTER TABLE ... ADD COLUMN` statements with string-matching on `"duplicate column name"` errors to fake idempotency. It also performed ad-hoc `PRAGMA table_info` introspection to decide whether to rebuild the `messages` table. Goose replaces both patterns with a single, versioned, recorded migration history. Do **not** reintroduce string-matched error handling or unconditional `ALTER` statements; add a new versioned migration file instead.

| Feature | Traditional Agents | babyCoder |
|---------|-------------------|-----------|
| Code Addressing | Line numbers | Content hashes |
| Edit Granularity | File/multi-line | Function/struct level |
| Context Size | Entire file | Single code block |
| Type Awareness | Limited/none | Full LSP integration |
| Validation | Post-edit | Pre-edit + post-edit |
| Model Requirements | Large (70B+) | Small (7B-13B) |

## Why This Matters

**Cost Efficiency**: Running 7B models locally is 10-100x cheaper than API calls to frontier models.

**Privacy**: Code never leaves your infrastructure.

**Speed**: Local inference with optimized context means faster iteration.

**Reliability**: Purpose-built tools reduce hallucination and improve success rates.

**Accessibility**: Enables Golang development assistance on consumer hardware.

## Roadmap

### ✅ Phase 1: Foundation (Complete)
- Interactive CLI with STDIN/STDOUT
- Configuration management (JSON)
- AI provider service (LMStudio)
- Agent loop with tool calling
- Session persistence (SQLite)
- File-based logging
- Comprehensive test coverage

### 🚧 Phase 2: Core Tooling (Current)
- Hashline addressing system
- Code extraction tools (functions, structs, interfaces)
- LSP integration for type checking
- Basic code editing tools

### 📋 Phase 3: Advanced Editing
- Multi-function refactoring
- Import management
- Code generation templates
- Intelligent error recovery

### 📋 Phase 4: Agent Optimization
- Tool usage learning
- Context optimization
- Batched operations
- Predictive caching

### 📋 Phase 5: Ecosystem Integration
- IDE plugins (VSCode, Neovim)
- CI/CD integration
- Team collaboration features
- Model fine-tuning datasets

## Project Structure

```
babyCoder/
├── main.go                           # Interactive CLI entry point
├── internal/
│   ├── config/
│   │   ├── config.go                # Configuration management
│   │   └── config_test.go           # 7 tests
│   ├── logging/
│   │   ├── logger.go                # File-based logging
│   │   └── logger_test.go           # 6 tests
│   ├── services/
│   │   ├── ai_provider/
│   │   │   ├── provider.go          # LMStudio provider
│   │   │   └── types.go             # OpenAI-compatible types
│   │   └── agent/
│   │       └── agent.go             # Agent loop with persistence
│   └── storage/
│       ├── database.go              # SQLite persistence
│       └── database_test.go         # 8 tests
├── docs/
│   ├── STORAGE.md                   # Storage API reference
│   ├── STORAGE_IMPLEMENTATION.md    # Implementation details
│   ├── LOGGING_AND_TESTS.md         # Logging and testing guide
│   └── INTERACTIVE_MODE.md          # Interactive mode usage
├── .babycoder/
│   ├── logs/                        # Timestamped log files
│   │   └── babycoder_*.log
│   ├── babycoder.db                 # SQLite database
│   └── babycoder.json               # Project configuration
```

## Documentation

- **[Storage Service](docs/STORAGE.md)** - Database schema and API reference
- **[Logging & Tests](docs/LOGGING_AND_TESTS.md)** - Logging system and test coverage
- **[Interactive Mode](docs/INTERACTIVE_MODE.md)** - Using the interactive CLI
- **[Implementation Details](docs/STORAGE_IMPLEMENTATION.md)** - Technical deep dive

babyCoder is built for the developer community. Contributions welcome in:
- New tool implementations
- LSP integration improvements
- Model evaluation benchmarks
- Documentation and examples

## License

MIT License - See LICENSE file for details

## Contact

For questions, issues, or collaboration: [repository_url]