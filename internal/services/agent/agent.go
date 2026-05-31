package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/exar/babycoder/internal/config"
	"github.com/exar/babycoder/internal/services/ai_provider"
	"github.com/exar/babycoder/internal/storage"
)

// Agent represents the coding agent
type Agent struct {
	provider      ai_provider.Provider
	configuration *config.AgentConfiguration
	database      *storage.Database
	sessionID     string
	tools         []ai_provider.Tool
	messages      []ai_provider.Message
	systemPrompt  string // Custom system prompt (optional)
	dreamTimer    *time.Timer
	projectRoot   string
	listener      Listener
}

// NewAgent creates a new agent instance
func NewAgent(provider ai_provider.Provider, configuration *config.AgentConfiguration, database *storage.Database, projectRoot string) *Agent {
	return &Agent{
		provider:      provider,
		configuration: configuration,
		database:      database,
		projectRoot:   projectRoot,
		sessionID:     "",
		tools:         []ai_provider.Tool{},
		messages:      []ai_provider.Message{},
		listener:      noopListener{},
	}
}

// SetListener installs a Listener to receive lifecycle events. Passing nil
// reverts the agent to the no-op default so call sites never have to nil-check.
func (agent *Agent) SetListener(listener Listener) {
	if listener == nil {
		agent.listener = noopListener{}
		return
	}
	agent.listener = listener
}

// SetSessionID sets the session ID for this agent
func (agent *Agent) SetSessionID(sessionID string) {
	agent.sessionID = sessionID
}

// GetSessionID returns the current session ID
func (agent *Agent) GetSessionID() string {
	return agent.sessionID
}

// SetSystemPrompt sets a custom system prompt for the agent
func (agent *Agent) SetSystemPrompt(prompt string) {
	agent.systemPrompt = prompt
}

// AddSystemMessage adds a system message to the conversation
func (agent *Agent) AddSystemMessage(content string) {
	message := ai_provider.Message{
		Role:    "system",
		Content: content,
	}
	agent.messages = append(agent.messages, message)

	// Save to database if session is active
	if agent.sessionID != "" && agent.database != nil {
		agent.saveMessage(message)
	}
}

// AddUserMessage adds a user message to the conversation
func (agent *Agent) AddUserMessage(content string) {
	message := ai_provider.Message{
		Role:    "user",
		Content: content,
	}
	agent.messages = append(agent.messages, message)

	// Save to database if session is active
	if agent.sessionID != "" && agent.database != nil {
		agent.saveMessage(message)
	}
}

// saveMessage persists a message to the database
func (agent *Agent) saveMessage(message ai_provider.Message) error {
	if agent.database == nil || agent.sessionID == "" {
		return nil
	}

	dbMessage := &storage.Message{
		SessionID:  agent.sessionID,
		Role:       message.Role,
		Content:    message.Content,
		ToolCalls:  storage.MarshalToolCalls(message.ToolCalls),
		ToolCallID: message.ToolCallID,
		CreatedAt:  time.Now(),
	}

	return agent.database.SaveMessage(dbMessage)
}

// RegisterTool registers a tool that the agent can use
func (agent *Agent) RegisterTool(tool ai_provider.Tool) {
	agent.tools = append(agent.tools, tool)
}

// ToolExecutor is a function that executes a tool call and returns the result
type ToolExecutor func(toolName string, arguments map[string]any) (string, error)

// UpdateDreamNow immediately updates the dream file based on the current session
func (agent *Agent) UpdateDreamNow(ctx context.Context) {
	agent.updateDream(ctx)
}

// Run executes the agent loop
func (agent *Agent) Run(ctx context.Context, executor ToolExecutor) error {
	// Cancel any existing dream timer
	if agent.dreamTimer != nil {
		agent.dreamTimer.Stop()
	}

	// Only add system message if this is the first message
	if len(agent.messages) == 0 {
		// Use custom prompt if set, otherwise use empty (will be set by caller)
		if agent.systemPrompt != "" {
			agent.AddSystemMessage(agent.systemPrompt)
		}
	}

	for iteration := 0; iteration < agent.configuration.MaxIterations; iteration++ {
		// Create chat completion request
		request := ai_provider.ChatCompletionRequest{
			Messages:   agent.messages,
			Tools:      agent.tools,
			ToolChoice: "auto",
		}

		// Call the AI provider
		agent.listener.OnRequestStart(iteration)
		response, err := agent.provider.ChatCompletion(ctx, request)
		agent.listener.OnRequestEnd(iteration)
		if err != nil {
			return fmt.Errorf("failed to get completion: %w", err)
		}

		if len(response.Choices) == 0 {
			return fmt.Errorf("no choices returned from AI provider")
		}

		choice := response.Choices[0]
		assistantMessage := choice.Message

		// Add assistant message to history
		agent.messages = append(agent.messages, assistantMessage)

		// Notify listener of the assistant message (content + any
		// out-of-band reasoning text).
		agent.listener.OnAssistantMessage(assistantMessage.Content, assistantReasoning(assistantMessage))

		// Save assistant message to database and capture the message ID
		var assistantMessageID int64
		if agent.sessionID != "" && agent.database != nil {
			dbMessage := &storage.Message{
				SessionID: agent.sessionID,
				Role:      assistantMessage.Role,
				Content:   assistantMessage.Content,
				ToolCalls: storage.MarshalToolCalls(assistantMessage.ToolCalls),
				CreatedAt: time.Now(),
			}

			if err := agent.database.SaveMessage(dbMessage); err != nil {
				log.Printf("Warning: failed to save assistant message: %v\n", err)
			} else {
				assistantMessageID = dbMessage.ID // Capture the ID for tool executions
			}

			// Update session
			session, err := agent.database.GetSession(agent.sessionID)
			if err == nil && session != nil {
				session.UpdatedAt = time.Now()
				agent.database.UpdateSession(session)
			}
		}

		// Check finish reason
		if choice.FinishReason == "stop" {
			// Agent has finished
			// Start dream update timer (10 seconds)
			agent.dreamTimer = time.AfterFunc(10*time.Second, func() {
				agent.updateDream(context.Background())
			})

			return nil
		}

		// Handle tool calls
		if choice.FinishReason == "tool_calls" && len(assistantMessage.ToolCalls) > 0 {
			for _, toolCall := range assistantMessage.ToolCalls {
				// Parse arguments
				var arguments map[string]any
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &arguments); err != nil {
					return fmt.Errorf("failed to parse tool arguments: %w", err)
				}

				// Notify listener before dispatching
				agent.listener.OnToolCall(toolCall.Function.Name, arguments)

				// Execute tool and track timing
				startTime := time.Now()
				result, err := executor(toolCall.Function.Name, arguments)
				executionTime := time.Since(startTime).Milliseconds()

				success := err == nil
				errorMsg := ""

				if err != nil {
					// Return error to the model
					result = fmt.Sprintf("Error executing tool: %s", err.Error())
					errorMsg = err.Error()
				}

				agent.listener.OnToolResult(toolCall.Function.Name, result, success, executionTime)

				// Save tool execution to database
				if agent.sessionID != "" && agent.database != nil {
					// Extract file_path from arguments if present
					filePath := ""
					if filePathArg, exists := arguments["file_path"]; exists {
						if fp, ok := filePathArg.(string); ok {
							filePath = fp
						}
					}

					toolExecution := &storage.ToolExecution{
						SessionID:   agent.sessionID,
						MessageID:   assistantMessageID, // Link to the assistant message that requested this tool
						ToolName:    toolCall.Function.Name,
						Arguments:   toolCall.Function.Arguments,
						Result:      result,
						FilePath:    filePath,
						Success:     success,
						ErrorMsg:    errorMsg,
						ExecutionMs: executionTime,
						CreatedAt:   time.Now(),
					}
					if err := agent.database.SaveToolExecution(toolExecution); err != nil {
						log.Printf("Warning: failed to save tool execution: %v\n", err)
					}
				}

				// Add tool response to messages
				toolMessage := ai_provider.Message{
					Role:       "tool",
					Content:    result,
					ToolCallID: toolCall.ID,
				}
				agent.messages = append(agent.messages, toolMessage)

				// Save tool response message
				if agent.sessionID != "" && agent.database != nil {
					if err := agent.saveMessage(toolMessage); err != nil {
						log.Printf("Warning: failed to save tool message: %v\n", err)
					}
				}
			}

			// Continue the loop to get the next response
			continue
		}

		// If we get here with another finish reason, something unexpected happened
	}

	return fmt.Errorf("agent exceeded maximum iterations (%d)", agent.configuration.MaxIterations)
}

// LoadSession loads a session from the database and restores message history
func (agent *Agent) LoadSession(sessionID string) error {
	if agent.database == nil {
		return fmt.Errorf("database not initialized")
	}

	// Get session
	session, err := agent.database.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Get messages
	dbMessages, err := agent.database.GetMessages(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get messages: %w", err)
	}

	// Convert database messages to agent messages
	agent.messages = []ai_provider.Message{}
	for _, dbMsg := range dbMessages {
		message := ai_provider.Message{
			Role:       dbMsg.Role,
			Content:    dbMsg.Content,
			ToolCallID: dbMsg.ToolCallID,
		}

		// Unmarshal tool calls if present
		if dbMsg.ToolCalls != "" {
			var toolCalls []ai_provider.ToolCall
			if err := storage.UnmarshalToolCalls(dbMsg.ToolCalls, &toolCalls); err == nil {
				message.ToolCalls = toolCalls
			}
		}

		agent.messages = append(agent.messages, message)
	}

	agent.sessionID = sessionID

	return nil
}

// assistantReasoning extracts the out-of-band reasoning text from an assistant
// message. Providers disagree on the field name: OpenAI o1 / DeepSeek use
// `reasoning_content`, others use `reasoning`. We accept both, preferring the
// former when populated.
func assistantReasoning(message ai_provider.Message) string {
	if message.ReasoningContent != "" {
		return message.ReasoningContent
	}
	return message.Reasoning
}
