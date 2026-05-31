package ai_provider

// Message represents a chat message
type Message struct {
	Role             string     `json:"role"`                         // system, user, assistant, tool
	Content          string     `json:"content"`                      // Message content
	ReasoningContent string     `json:"reasoning_content,omitempty"`  // Out-of-band reasoning text emitted by reasoning-capable models (DeepSeek, LMStudio, etc.)
	Reasoning        string     `json:"reasoning,omitempty"`          // Alternate field name used by some providers
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`         // Tool calls made by assistant
	ToolCallID       string     `json:"tool_call_id,omitempty"`       // ID for tool response messages
	Name             string     `json:"name,omitempty"`               // Tool name for tool messages
}

// ToolCall represents a function call request from the model
type ToolCall struct {
	ID       string              `json:"id"`
	Type     string              `json:"type"` // Always "function"
	Function ToolCallFunction    `json:"function"`
}

// ToolCallFunction represents the function details in a tool call
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string of arguments
}

// Tool represents a tool definition that can be called by the model
type Tool struct {
	Type     string       `json:"type"` // Always "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction represents the function definition
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema
}

// ChatCompletionRequest represents a request to the chat completion API
type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []Tool    `json:"tools,omitempty"`
	ToolChoice  string    `json:"tool_choice,omitempty"` // "auto", "none", or specific tool
	Temperature float32   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

// ChatCompletionResponse represents a response from the chat completion API
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a completion choice
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"` // stop, length, tool_calls, content_filter
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ErrorResponse represents an error response from the API
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error information
type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}
