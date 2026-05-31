# Prompt Management

babyCoder uses a simple, centralized prompt system that's easy to customize.

## Default Prompts

### Main Agent Prompt
```
You are a helpful coding assistant specializing in Golang development.

You have access to various tools to analyze and modify code. When you need to use a tool, call it with the appropriate parameters.

Always explain your reasoning before taking actions.
```

### Sub-Agent Prompt
```
You are a research assistant. Investigate the given task using available tools and provide a clear summary of your findings.

Task: {{task}}

Provide a concise summary with key findings and relevant file references.
```

## Customizing Prompts

### Option 1: Inline Override

Edit `.babycoder/babycoder.json`:

```json
{
  "prompts": {
    "main_agent": "You are a senior Go architect. Focus on enterprise patterns and best practices...",
    "sub_agent": "You are a research assistant. Task: {{task}}\n\nBe thorough and include code examples."
  }
}
```

### Option 2: Load from File

Create `prompts/main_agent.txt`:
```
You are a helpful coding assistant specializing in Golang.

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

Sub-agent prompts support `{{task}}` placeholder which gets replaced with the actual research task.

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
    "main_agent": "You are a backend Go developer specializing in microservices and Kubernetes deployments.",
    "sub_agent": "Research the task: {{task}}\n\nFocus on production readiness and observability."
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
  "sub_agent": "file://./my-prompt.txt",
}
pm.LoadFromConfig(config)

// Get prompt
mainPrompt := pm.GetPrompt(prompts.MainAgent)

// Render with variables
subPrompt := pm.RenderPrompt(prompts.SubAgent, map[string]string{
  "task": "Analyze authentication",
})
```

## Files

- `internal/prompts/manager.go` - Prompt loading and management
- `internal/prompts/defaults.go` - Default prompts
- `.babycoder/babycoder.json` - Configuration file with prompt overrides

## See Also

- `.babycoder/babycoder.json.example` - Example configuration file
- The prompts are intentionally simple - customize as needed for your use case
