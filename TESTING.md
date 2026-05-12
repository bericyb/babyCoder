# babyCoder - Testing the Agent Loop

## Quick Start

### 1. Initialize the Project

```bash
./babyCoder init
```

This creates a `.babycoder.json` configuration file with default settings.

### 2. Configure LMStudio

Edit `.babycoder.json` to point to your LMStudio instance:

```json
{
  "ai_provider": {
    "type": "lmstudio",
    "endpoint": "http://localhost:1234/v1",
    "model": "your-model-name",
    "temperature": 0.2,
    "api_key": ""
  },
  "agent": {
    "max_iterations": 10,
    "verbose": true,
    "auto_commit": false
  },
  "project": {
    "root": "/path/to/your/project",
    "exclude_patterns": [
      "vendor/",
      "**/*_test.go",
      "**/*.pb.go"
    ]
  }
}
```

### 3. Start LMStudio

Make sure LMStudio is running and serving a model on the configured endpoint (default: `http://localhost:1234/v1`).

### 4. Run the Agent

```bash
./babyCoder agent "Hello! Can you help me understand how to use you?"
```

## Current Implementation Status

### ✅ Completed
- Configuration system with JSON support
- AI provider service with LMStudio/OpenAI spec support
- Agent service with tool calling loop
- Basic CLI commands (init, agent)
- Message history management
- Verbose logging option

### 🚧 Next Steps
- Implement tools service with code analysis capabilities
- Add hashline-based code editing
- LSP integration for type checking
- Project context management
- File system operations

## Architecture

```
babyCoder/
├── main.go                           # CLI entry point
├── .babycoder.json                   # Configuration file (created by init)
├── internal/
│   ├── config/
│   │   └── config.go                # Configuration loading and defaults
│   └── services/
│       ├── ai_provider/
│       │   ├── types.go             # OpenAI-compatible type definitions
│       │   └── provider.go          # LMStudio provider implementation
│       └── agent/
│           └── agent.go             # Agent loop with tool calling
```

## How It Works

1. **Configuration**: The agent loads settings from `.babycoder.json` or uses defaults
2. **Provider Setup**: Creates an AI provider client (LMStudio) with the configured endpoint
3. **Agent Loop**:
   - Sends messages to the LLM
   - Receives responses (text or tool calls)
   - Executes requested tools
   - Returns results to the LLM
   - Continues until completion or max iterations

4. **Tool Calling**: The agent supports OpenAI-style tool calling:
   - Tools are registered with JSON Schema definitions
   - LLM requests tool execution with structured arguments
   - Results are fed back into the conversation

## Example Session

```bash
$ ./babyCoder agent "What is 2 + 2?"

Using model: local-model
Endpoint: http://localhost:1234/v1

=== Iteration 1/10 ===
Assistant: 2 + 2 equals 4. This is basic arithmetic addition.

Agent finished successfully

=== Final Response ===
2 + 2 equals 4. This is basic arithmetic addition.
```

## Testing Without LMStudio

To test the basic structure without a running LMStudio instance, you can:

1. Review the code structure
2. Check that configuration loads correctly: `./babyCoder init`
3. Inspect the generated `.babycoder.json`

Once LMStudio is running, the agent will be able to:
- Process natural language prompts
- Call registered tools (to be implemented)
- Iterate on tasks until completion
- Return structured results

## Configuration Options

### AI Provider
- `type`: Provider type (currently only "lmstudio" supported)
- `endpoint`: LMStudio API endpoint
- `model`: Model identifier to use
- `temperature`: Sampling temperature (0.0 to 1.0)
- `api_key`: Optional API key for authentication

### Agent
- `max_iterations`: Maximum number of loop iterations (default: 10)
- `verbose`: Enable detailed logging (default: true)
- `auto_commit`: Automatically commit code changes (default: false)

### Project
- `root`: Project root directory
- `exclude_patterns`: Glob patterns for files to ignore
