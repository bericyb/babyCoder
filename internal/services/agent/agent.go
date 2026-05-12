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
	messageIndex  int
	systemPrompt  string // Custom system prompt (optional)
}

// NewAgent creates a new agent instance
func NewAgent(provider ai_provider.Provider, configuration *config.AgentConfiguration, database *storage.Database) *Agent {
	return &Agent{
		provider:      provider,
		configuration: configuration,
		database:      database,
		sessionID:     "",
		tools:         []ai_provider.Tool{},
		messages:      []ai_provider.Message{},
		messageIndex:  0,
	}
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
		SessionID:    agent.sessionID,
		Role:         message.Role,
		Content:      message.Content,
		ToolCalls:    storage.MarshalToolCalls(message.ToolCalls),
		ToolCallID:   message.ToolCallID,
		MessageIndex: agent.messageIndex,
		CreatedAt:    time.Now(),
	}

	agent.messageIndex++
	return agent.database.SaveMessage(dbMessage)
}

// RegisterTool registers a tool that the agent can use
func (agent *Agent) RegisterTool(tool ai_provider.Tool) {
	agent.tools = append(agent.tools, tool)
}

// ToolExecutor is a function that executes a tool call and returns the result
type ToolExecutor func(toolName string, arguments map[string]interface{}) (string, error)

// Run executes the agent loop
func (agent *Agent) Run(ctx context.Context, executor ToolExecutor) error {
	// Only add system message if this is the first message
	if len(agent.messages) == 0 {
		// Use custom prompt if set, otherwise use empty (will be set by caller)
		if agent.systemPrompt != "" {
			agent.AddSystemMessage(agent.systemPrompt)
		}
	}

	if agent.configuration.Verbose {
		log.Printf("Starting agent loop with max iterations: %d\n", agent.configuration.MaxIterations)
	}

	for iteration := 0; iteration < agent.configuration.MaxIterations; iteration++ {
		if agent.configuration.Verbose {
			log.Printf("=== Iteration %d/%d ===\n", iteration+1, agent.configuration.MaxIterations)
		}

		// Create chat completion request
		request := ai_provider.ChatCompletionRequest{
			Messages:   agent.messages,
			Tools:      agent.tools,
			ToolChoice: "auto",
		}

		// Call the AI provider
		response, err := agent.provider.ChatCompletion(ctx, request)
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
		
		// Save assistant message to database and capture the message ID
		var assistantMessageID int64
		if agent.sessionID != "" && agent.database != nil {
			dbMessage := &storage.Message{
				SessionID:    agent.sessionID,
				Role:         assistantMessage.Role,
				Content:      assistantMessage.Content,
				ToolCalls:    storage.MarshalToolCalls(assistantMessage.ToolCalls),
				MessageIndex: agent.messageIndex,
				CreatedAt:    time.Now(),
			}
			agent.messageIndex++
			
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

		if agent.configuration.Verbose {
			log.Printf("Assistant: %s\n", assistantMessage.Content)
			if len(assistantMessage.ToolCalls) > 0 {
				log.Printf("Tool calls: %d\n", len(assistantMessage.ToolCalls))
			}
		}

		// Check finish reason
		if choice.FinishReason == "stop" {
			// Agent has finished
			if agent.configuration.Verbose {
				log.Println("Agent finished successfully")
			}
			return nil
		}

		// Handle tool calls
		if choice.FinishReason == "tool_calls" && len(assistantMessage.ToolCalls) > 0 {
			for _, toolCall := range assistantMessage.ToolCalls {
				if agent.configuration.Verbose {
					log.Printf("Executing tool: %s (ID: %s)\n", toolCall.Function.Name, toolCall.ID)
					log.Printf("Arguments: %s\n", toolCall.Function.Arguments)
				}

				// Parse arguments
				var arguments map[string]interface{}
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &arguments); err != nil {
					return fmt.Errorf("failed to parse tool arguments: %w", err)
				}

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
					if agent.configuration.Verbose {
						log.Printf("Tool execution error: %s\n", err)
					}
				} else {
					if agent.configuration.Verbose {
						log.Printf("Tool result: %s\n", result)
					}
				}
				
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

		// If we get here with another finish reason, log it
		if agent.configuration.Verbose {
			log.Printf("Unexpected finish reason: %s\n", choice.FinishReason)
		}
	}

	return fmt.Errorf("agent exceeded maximum iterations (%d)", agent.configuration.MaxIterations)
}

// RunIteration runs a single iteration of the agent loop and returns the assistant's response
// This is useful for sub-agents that need fine-grained control over execution
func (agent *Agent) RunIteration(ctx context.Context, executor ToolExecutor) (string, error) {
	// Create chat completion request
	request := ai_provider.ChatCompletionRequest{
		Messages:   agent.messages,
		Tools:      agent.tools,
		ToolChoice: "auto",
	}

	// Call the AI provider
	response, err := agent.provider.ChatCompletion(ctx, request)
	if err != nil {
		return "", fmt.Errorf("failed to get completion: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from AI provider")
	}

	choice := response.Choices[0]
	assistantMessage := choice.Message

	// Add assistant message to history
	agent.messages = append(agent.messages, assistantMessage)
	
	// Save assistant message to database and capture the message ID
	var assistantMessageID int64
	if agent.sessionID != "" && agent.database != nil {
		dbMessage := &storage.Message{
			SessionID:    agent.sessionID,
			Role:         assistantMessage.Role,
			Content:      assistantMessage.Content,
			ToolCalls:    storage.MarshalToolCalls(assistantMessage.ToolCalls),
			MessageIndex: agent.messageIndex,
			CreatedAt:    time.Now(),
		}
		agent.messageIndex++
		
		if err := agent.database.SaveMessage(dbMessage); err != nil {
			log.Printf("Warning: failed to save assistant message: %v\n", err)
		} else {
			assistantMessageID = dbMessage.ID
		}
		
		// Update session
		session, err := agent.database.GetSession(agent.sessionID)
		if err == nil && session != nil {
			session.UpdatedAt = time.Now()
			agent.database.UpdateSession(session)
		}
	}

	// Handle tool calls if present
	if choice.FinishReason == "tool_calls" && len(assistantMessage.ToolCalls) > 0 {
		for _, toolCall := range assistantMessage.ToolCalls {
			// Parse arguments
			var arguments map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &arguments); err != nil {
				return "", fmt.Errorf("failed to parse tool arguments: %w", err)
			}

			// Execute tool and track timing
			startTime := time.Now()
			result, err := executor(toolCall.Function.Name, arguments)
			executionTime := time.Since(startTime).Milliseconds()
			
			success := err == nil
			errorMsg := ""
			
			if err != nil {
				result = fmt.Sprintf("Error executing tool: %s", err.Error())
				errorMsg = err.Error()
			}
			
			// Save tool execution to database
			if agent.sessionID != "" && agent.database != nil {
				filePath := ""
				if filePathArg, exists := arguments["file_path"]; exists {
					if fp, ok := filePathArg.(string); ok {
						filePath = fp
					}
				}
				
				toolExecution := &storage.ToolExecution{
					SessionID:   agent.sessionID,
					MessageID:   assistantMessageID,
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
	}

	return assistantMessage.Content, nil
}

// HasPendingToolCalls returns true if the last message contains tool calls that need execution
func (agent *Agent) HasPendingToolCalls() bool {
	if len(agent.messages) == 0 {
		return false
	}

	lastMessage := agent.messages[len(agent.messages)-1]
	
	// If last message is from assistant and has tool calls, we need to process them
	if lastMessage.Role == "assistant" && len(lastMessage.ToolCalls) > 0 {
		// But check if we've already processed them (next message would be a tool response)
		if len(agent.messages) >= 2 {
			secondLast := agent.messages[len(agent.messages)-2]
			if secondLast.Role == "assistant" && len(secondLast.ToolCalls) > 0 {
				// This means we DID process them and got a new assistant response
				return false
			}
		}
		return false // Tool calls are processed in the same iteration
	}

	return false
}

// GetMessages returns the conversation history
func (agent *Agent) GetMessages() []ai_provider.Message {
	return agent.messages
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
	agent.messageIndex = len(agent.messages)

	if agent.configuration.Verbose {
		log.Printf("Loaded session %s with %d messages\n", sessionID, len(agent.messages))
	}

	return nil
}
