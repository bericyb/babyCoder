# Rules System

babyCoder supports project-specific rules that get automatically injected into the agent's system prompt.

## Overview

**Rules** are instructions or guidelines that you want the AI to follow throughout your entire project. They're stored in a `rules.md` file in your project root and are automatically loaded into every agent session.

## Creating Rules

### Interactive Mode

Use the `#` prefix to add a rule:

```
You: # Always use error handling for file operations
✓ Rule added to /path/to/project/rules.md

You: # Write tests for all public functions
✓ Rule added to /path/to/project/rules.md
```

**Rules are NOT sent to the AI** - they're saved to the file and loaded into the system prompt automatically.

### Manual Editing

You can also edit `rules.md` directly:

```markdown
# Rules

- Always use error handling for file operations
- Write tests for all public functions
- Use descriptive variable names (no abbreviations)
- Follow the architectural guidelines in AGENTS.md
```

## Viewing Rules

Use the `/rules` command:

```
You: /rules

Current rules (/path/to/project/rules.md):
  1. Always use error handling for file operations
  2. Write tests for all public functions
  3. Use descriptive variable names (no abbreviations)
  4. Follow the architectural guidelines in AGENTS.md
```

## How Rules Work

### System Prompt Injection

When the agent starts, rules are injected into the system prompt:

```
You are a helpful coding assistant specializing in Golang development.

You have access to various tools to analyze and modify code. When you need to use a tool, call it with the appropriate parameters.

Always explain your reasoning before taking actions.

Project Rules:
- Always use error handling for file operations
- Write tests for all public functions
- Use descriptive variable names (no abbreviations)
- Follow the architectural guidelines in AGENTS.md
```

### Automatic Loading

- Rules are loaded on agent startup
- Rules are reloaded when you use `/new` to start a new session
- Changes to `rules.md` take effect in new sessions

## Use Cases

### Project Standards

```markdown
# Rules

- All API endpoints must have OpenAPI documentation
- Database queries must use prepared statements
- All public functions must have godoc comments
- Use structured logging (never fmt.Printf for logs)
```

### Team Conventions

```markdown
# Rules

- Follow the company Go style guide at docs/STYLE.md
- All PRs require tests with >80% coverage
- Use the project's error handling pattern (errors package)
- Configuration must use environment variables, not hardcoded values
```

### Domain-Specific Guidelines

```markdown
# Rules

- All financial calculations must use decimal types, never float64
- User input must be validated before database operations
- Sensitive data must be encrypted at rest
- All external API calls must have timeout and retry logic
```

### Learning Goals

```markdown
# Rules

- Explain your reasoning for each architectural decision
- Suggest performance optimizations when applicable
- Point out potential security issues in code
- Recommend best practices for Go concurrency
```

## File Format

**Location:** `./rules.md` (project root)

**Format:** Markdown list (bullet points with `-` or `*`)

**Example:**
```markdown
# Rules

- Rule one
- Rule two
- Rule three
```

**Parsing:**
- Only lines starting with `- ` or `* ` are parsed as rules
- Other text (headers, paragraphs) is ignored
- Leading/trailing whitespace is trimmed
- Empty rules are skipped

## Managing Rules

### Add a rule
```
You: # New rule here
```

### View rules
```
You: /rules
```

### Edit rules
Open `rules.md` in your editor and modify directly.

### Clear all rules
Delete the `rules.md` file:
```bash
rm rules.md
```

## Best Practices

### ✅ DO:
- Keep rules concise and actionable
- Focus on project-specific conventions
- Update rules as your project evolves
- Version control `rules.md` with your code
- Use rules for standards that apply across the entire codebase

### ❌ DON'T:
- Don't add too many rules (agent has context limits)
- Don't duplicate information that's in documentation
- Don't add temporary task-specific instructions (just tell the agent directly)
- Don't add contradictory rules

## Example Workflow

```
# Start babyCoder
$ ./babyCoder

You: # Always write tests for new functions
✓ Rule added to /Users/me/project/rules.md

You: # Use the repository pattern for database access
✓ Rule added to /Users/me/project/rules.md

You: /rules

Current rules (/Users/me/project/rules.md):
  1. Always write tests for new functions
  2. Use the repository pattern for database access

You: Create a new user registration function

Agent: I'll create a user registration function following the project rules.
Let me start by creating the repository interface...
[Creates code with tests, following the rules]
```

## Technical Details

**Module:** `internal/rules/rules.go`

**Key Functions:**
```go
// Create rules manager
rm := rules.NewRulesManager(projectRoot)

// Check if rules exist
exists := rm.RulesExist()

// Load rules
rules, err := rm.LoadRules()

// Add a rule
err := rm.AddRule("New rule")

// Get formatted text for system prompt
text, err := rm.GetRulesAsText()
```

**Integration:**
- Rules are loaded in `main.go` → `runInteractive()`
- Injected after main prompt, before tool descriptions
- Reloaded on `/new` command

## See Also

- `PROMPTS.md` - For customizing the main agent prompt
- `rules.md.example` - Example rules file
- `internal/rules/rules_test.go` - 9 tests covering all functionality
