package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/exar/babycoder/internal/prompts"
	"github.com/exar/babycoder/internal/services/agent"
	"github.com/exar/babycoder/internal/services/ai_provider"
	"github.com/exar/babycoder/internal/storage"
	"github.com/google/uuid"
)

// AgentInterface defines the interface for an agent (to avoid circular imports)
type AgentInterface interface {
	AddSystemMessage(content string)
	AddUserMessage(content string)
	RunIteration(ctx context.Context, executor agent.ToolExecutor) (string, error)
	HasPendingToolCalls() bool
	GetMessages() []ai_provider.Message
}

// AgentFactory creates a new agent instance with the given session ID
type AgentFactory func(sessionID string) (AgentInterface, agent.ToolExecutor, error)

// SubAgentTool spawns an isolated sub-agent to handle a specific research task
// and returns an executive summary without polluting the parent context
type SubAgentTool struct {
	ProjectRoot   string
	ParentSession string
	Database      *storage.Database
	AgentFactory  AgentFactory
	PromptManager *prompts.PromptManager // Centralized prompt management
}

// GetDefinition returns the tool definition for the AI provider
func (tool *SubAgentTool) GetDefinition() ai_provider.Tool {
	return ai_provider.Tool{
		Type: "function",
		Function: ai_provider.ToolFunction{
			Name:        "run_subagent",
			Description: "Spawn an isolated sub-agent to research a specific question or task. The sub-agent has access to all tools and will return a concise executive summary. Use this to prevent context pollution when deep investigation is needed (e.g., 'how does the notification system work?', 'analyze the authentication flow').",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task": map[string]interface{}{
						"type":        "string",
						"description": "Clear description of the research task or question for the sub-agent to investigate (e.g., 'Find out how error handling works in the API layer')",
					},
					"max_iterations": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of agent loop iterations to prevent runaway execution (default: 10, max: 50)",
					},
					"timeout_seconds": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum execution time in seconds (default: 300, max: 1800)",
					},
					"custom_instructions": map[string]interface{}{
						"type":        "string",
						"description": "Optional additional instructions to append to the sub-agent's prompt",
					},
				},
				"required": []string{"task"},
			},
		},
	}
}

// SubAgentResult contains the execution result from a sub-agent
type SubAgentResult struct {
	Summary           string `json:"summary"`
	SessionID         string `json:"session_id"`
	ExecutionTimeMs   int64  `json:"execution_time_ms"`
	IterationsUsed    int    `json:"iterations_used"`
	ToolCallsExecuted int    `json:"tool_calls_executed"`
	Success           bool   `json:"success"`
	ErrorMessage      string `json:"error_message,omitempty"`
}

// Execute spawns a sub-agent and returns an executive summary
func (tool *SubAgentTool) Execute(arguments map[string]interface{}) (string, error) {
	startTime := time.Now()

	// Parse arguments
	task, ok := arguments["task"].(string)
	if !ok || task == "" {
		return "", fmt.Errorf("task parameter is required and must be a non-empty string")
	}

	maxIterations := 10 // Default
	if maxIter, ok := arguments["max_iterations"].(float64); ok {
		maxIterations = int(maxIter)
		if maxIterations > 50 {
			maxIterations = 50
		}
	}

	timeoutSeconds := 300 // Default 5 minutes
	if timeout, ok := arguments["timeout_seconds"].(float64); ok {
		timeoutSeconds = int(timeout)
		if timeoutSeconds > 1800 { // Max 30 minutes
			timeoutSeconds = 1800
		}
	}

	customInstructions := ""
	if custom, ok := arguments["custom_instructions"].(string); ok {
		customInstructions = custom
	}

	// Create sub-agent session
	subSessionID := uuid.New().String()
	parentPtr := &tool.ParentSession
	session := &storage.Session{
		ID:              subSessionID,
		ProjectRoot:     tool.ProjectRoot,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		Title:           fmt.Sprintf("🤖 Sub-agent: %s", truncateString(task, 50)),
		Status:          "active",
		ParentSessionID: parentPtr,
		SessionType:     "subagent",
		TaskDescription: task,
	}

	if err := tool.Database.CreateSession(session); err != nil {
		return "", fmt.Errorf("failed to create sub-agent session: %w", err)
	}

	// Create sub-agent and tool executor using factory
	subAgent, toolExecutor, err := tool.AgentFactory(subSessionID)
	if err != nil {
		tool.Database.UpdateSessionStatus(subSessionID, "failed")
		return "", fmt.Errorf("failed to create sub-agent: %w", err)
	}

	// Get system prompt from prompt manager (with variable substitution)
	var systemPrompt string
	if tool.PromptManager != nil {
		systemPrompt = tool.PromptManager.RenderPrompt(prompts.SubAgent, map[string]string{
			"task": task,
		})
	} else {
		// Fallback to default if no prompt manager
		systemPrompt = prompts.DefaultSubAgentPrompt
		systemPrompt = strings.ReplaceAll(systemPrompt, "{{task}}", task)
	}

	// Append custom instructions if provided
	if customInstructions != "" {
		systemPrompt += "\n\n" + customInstructions
	}

	subAgent.AddSystemMessage(systemPrompt)
	subAgent.AddUserMessage(task)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	// Run sub-agent loop
	var iterationsUsed int
	var toolCallsExecuted int
	var lastResponse string
	var executionError error

	for iterationsUsed < maxIterations {
		// Check timeout
		select {
		case <-ctx.Done():
			executionError = fmt.Errorf("sub-agent execution timed out after %d seconds", timeoutSeconds)
			goto GenerateSummary
		default:
		}

		// Run one agent iteration
		response, err := subAgent.RunIteration(ctx, toolExecutor)
		if err != nil {
			executionError = fmt.Errorf("sub-agent iteration %d failed: %w", iterationsUsed+1, err)
			goto GenerateSummary
		}

		lastResponse = response
		iterationsUsed++

		// Count tool calls in this iteration
		messages := subAgent.GetMessages()
		if len(messages) > 0 {
			lastMsg := messages[len(messages)-1]
			if lastMsg.Role == "assistant" && len(lastMsg.ToolCalls) > 0 {
				toolCallsExecuted += len(lastMsg.ToolCalls)
			}
		}

		// Check if agent finished (no more tool calls)
		if !subAgent.HasPendingToolCalls() {
			break
		}
	}

GenerateSummary:
	executionTime := time.Since(startTime).Milliseconds()

	// Update session status
	if executionError != nil {
		tool.Database.UpdateSessionStatus(subSessionID, "failed")
	} else {
		tool.Database.UpdateSessionStatus(subSessionID, "completed")
	}

	// Extract or generate executive summary
	summary := extractExecutiveSummary(lastResponse, task, executionError)

	result := SubAgentResult{
		Summary:           summary,
		SessionID:         subSessionID,
		ExecutionTimeMs:   executionTime,
		IterationsUsed:    iterationsUsed,
		ToolCallsExecuted: toolCallsExecuted,
		Success:           executionError == nil,
	}

	if executionError != nil {
		result.ErrorMessage = executionError.Error()
	}

	// Format result as markdown
	output := fmt.Sprintf(`# Sub-Agent Research Complete

**Task:** %s
**Session ID:** %s
**Execution Time:** %.2fs
**Iterations Used:** %d/%d
**Tool Calls:** %d
**Status:** %s

---

%s

---

*Full conversation trace available in session %s*`, 
		task,
		subSessionID,
		float64(executionTime)/1000.0,
		iterationsUsed,
		maxIterations,
		toolCallsExecuted,
		map[bool]string{true: "✅ Success", false: "⚠️ Failed or Incomplete"}[result.Success],
		summary,
		subSessionID,
	)

	return output, nil
}

// extractExecutiveSummary extracts or generates an executive summary from the agent's response
func extractExecutiveSummary(response string, task string, executionError error) string {
	if executionError != nil {
		return fmt.Sprintf("⚠️ **Sub-agent execution failed:** %s\n\nPartial findings:\n%s", 
			executionError.Error(), 
			truncateString(response, 500))
	}

	// Try to find explicit executive summary section
	if summary := extractSection(response, "## EXECUTIVE SUMMARY"); summary != "" {
		return summary
	}

	if summary := extractSection(response, "## Executive Summary"); summary != "" {
		return summary
	}

	if summary := extractSection(response, "# EXECUTIVE SUMMARY"); summary != "" {
		return summary
	}

	// Fallback: use last response truncated
	if response == "" {
		return fmt.Sprintf("⚠️ No findings. The sub-agent may not have completed its investigation of: %s", task)
	}

	// Return last 500 chars as summary
	return fmt.Sprintf("## Summary\n\n%s\n\n*(Note: Explicit executive summary not found, showing last response)*", 
		truncateString(response, 500))
}

// extractSection extracts content after a markdown header
func extractSection(text string, header string) string {
	if len(text) < len(header) {
		return ""
	}

	// Find header
	for i := 0; i <= len(text)-len(header); i++ {
		if text[i:i+len(header)] == header {
			// Found header, extract rest
			remaining := text[i+len(header):]
			// Trim any following newlines
			for len(remaining) > 0 && (remaining[0] == '\n' || remaining[0] == '\r') {
				remaining = remaining[1:]
			}
			return remaining
		}
	}

	return ""
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
