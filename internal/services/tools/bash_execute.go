package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/exar/babycoder/internal/services/ai_provider"
)

// BashExecuteTool implements synchronous bash command execution
type BashExecuteTool struct {
	projectRoot string
}

// Execute runs a bash command synchronously
func (tool *BashExecuteTool) Execute(arguments map[string]any) (string, error) {
	command, err := getStringArg(arguments, "command")
	if err != nil {
		return "", err
	}

	workingDir := getStringArgDefault(arguments, "working_dir", tool.projectRoot)
	timeoutSeconds := getFloat64ArgDefault(arguments, "timeout_seconds", 30)

	if timeoutSeconds > 300 {
		return "", fmt.Errorf("timeout cannot exceed 300 seconds")
	}

	timeout := time.Duration(timeoutSeconds) * time.Second

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Execute command
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n" + stderr.String()
	}

	if err != nil {
		return fmt.Sprintf("Command failed:\n%s\nError: %v", output, err), err
	}

	return fmt.Sprintf("Command succeeded:\n%s", output), nil
}

// GetDefinition returns the tool definition for the AI provider
func (tool *BashExecuteTool) GetDefinition() ai_provider.Tool {
	return ai_provider.Tool{
		Type: "function",
		Function: ai_provider.ToolFunction{
			Name:        "bash_execute",
			Description: "Execute a bash command synchronously and return the output. Use for quick commands that complete in seconds (max 30s default, 300s max).",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "The bash command to execute (e.g., 'ls -la', 'curl http://localhost:8080', 'grep -r TODO .')",
					},
					"working_dir": map[string]any{
						"type":        "string",
						"description": "Optional working directory (relative to project root or absolute). Defaults to project root.",
					},
					"timeout_seconds": map[string]any{
						"type":        "number",
						"description": "Optional timeout in seconds (default: 30, max: 300)",
					},
				},
				"required": []string{"command"},
			},
		},
	}
}
