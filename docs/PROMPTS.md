# Prompt Management

babyCoder uses a simple, centralized prompt system that's easy to customize.

## Default Prompts

### Main Agent Prompt
```
You are a helpful coding assistant.

You have access to various tools to analyze and modify code. When you need to use a tool, call it with the appropriate parameters.

Always explain your reasoning before taking actions.
```

## Customizing Prompts

### Option 1: Inline Override

Edit `.babycoder/babycoder.json`:

```json
{
  "prompts": {
    "main_agent": "You are a senior Go architect. Focus on enterprise patterns and best practices..."
  }
}
```

### Option 2: Load from File

Create `prompts/main_agent.txt`:
```
You are a helpful coding assistant.

Company Standards:
- Follow internal/docs/STYLE_GUIDE.md
- All code must have tests
- Use structured logging

Available tools: read_file, write_file, edit files, run tests, etc.
```

Reference in `.babycoder/babycoder.json`:
```json
{
  "prompts": {
    "main_agent": "file://./prompts/main_agent.txt"
  }
}
```

**File paths can be:**
- Relative to project: `file://./prompts/agent.txt`
- Absolute: `file:///home/user/prompts/agent.txt`
- Home directory: `file://~/.babycoder/prompts/agent.txt`

## Variable Substitution

Prompts support `{{variable}}` placeholders via `RenderPrompt`, which performs
simple string substitution against a caller-supplied map.

## Best Practices

### ✅ DO:
- Keep prompts simple and clear
- Focus on behavior, not formatting
- Test changes with real interactions
- Version control your custom prompts

### ❌ DON'T:
- Don't make prompts overly complex
- Don't include tool-specific instructions (tools have their own descriptions)
- Don't override unless you have a specific need

## Examples

### Minimal Override
```json
{
  "prompts": {
    "main_agent": "You are a Go coding assistant. Use tools actively. Explain your reasoning."
  }
}
```

### Company Standard
```json
{
  "prompts": {
    "main_agent": "file://~/.company/babycoder-prompt.txt"
  }
}
```

### Domain-Specific
```json
{
  "prompts": {
    "main_agent": "You are a backend Go developer specializing in microservices and Kubernetes deployments."
  }
}
```

## API Usage

```go
import "github.com/exar/babycoder/internal/prompts"

// Create manager
pm := prompts.NewPromptManager("/project/root")

// Load from config
config := map[string]interface{}{
  "main_agent": "Custom prompt...",
}
pm.LoadFromConfig(config)

// Get prompt
mainPrompt := pm.GetPrompt(prompts.MainAgent)
```

## Files

- `internal/prompts/manager.go` - Prompt loading and management
- `internal/prompts/defaults.go` - Default prompts
- `.babycoder/babycoder.json` - Configuration file with prompt overrides

## See Also

- `.babycoder/babycoder.json.example` - Example configuration file
- The prompts are intentionally simple - customize as needed for your use case
