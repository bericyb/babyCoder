# babyCoder Interactive Mode - Quick Start

## Running babyCoder

Simply run `babyCoder` with no arguments to start interactive mode:

```bash
./babyCoder
```

You'll see a welcome screen:

```
╔════════════════════════════════════════════════════════════╗
║                   babyCoder Interactive                    ║
╚════════════════════════════════════════════════════════════╝

Model:    local-model
Endpoint: http://127.0.0.1:1234/v1

Type your message and press Enter to send.
Type /exit to quit.
Type /new to start a new session.
```

## Interactive Commands

### Chat with the Agent

Simply type your message and press Enter:

```
You: Hello! Can you help me refactor this function?
```

The agent will process your request and respond. Once complete, you can continue the conversation:

```
You: Can you give me an example?
```

### Available Commands

- `/exit` - Exit babyCoder and save the session
- `/new` - Start a new session (completes current session)
- `/help` - Show available commands

## Example Session

```
You: What is a goroutine?