package agent

import (
	"context"
	"sync"
	"testing"

	"github.com/exar/babycoder/internal/config"
	"github.com/exar/babycoder/internal/services/ai_provider"
)

// scriptedProvider returns a pre-baked sequence of completion responses, one
// per call. It lets us simulate a multi-turn agent loop (e.g. tool call →
// final stop) without a real AI backend.
type scriptedProvider struct {
	responses []ai_provider.ChatCompletionResponse
	callCount int
}

func (provider *scriptedProvider) ChatCompletion(ctx context.Context, request ai_provider.ChatCompletionRequest) (*ai_provider.ChatCompletionResponse, error) {
	if provider.callCount >= len(provider.responses) {
		response := provider.responses[len(provider.responses)-1]
		return &response, nil
	}
	response := provider.responses[provider.callCount]
	provider.callCount++
	return &response, nil
}

func (provider *scriptedProvider) GetModel() string { return "scripted" }

// recordedEvent captures a listener callback for later assertions.
type recordedEvent struct {
	kind             string
	iteration        int
	content          string
	reasoning        string
	toolName         string
	toolArguments    map[string]any
	toolResult       string
	toolSuccess      bool
	toolDurationMs   int64
}

// recordingListener implements Listener by appending every event into an
// in-memory slice. Safe for concurrent use, though the agent loop is
// single-threaded in practice.
type recordingListener struct {
	mutex  sync.Mutex
	events []recordedEvent
}

func (listener *recordingListener) record(event recordedEvent) {
	listener.mutex.Lock()
	defer listener.mutex.Unlock()
	listener.events = append(listener.events, event)
}

func (listener *recordingListener) OnRequestStart(iteration int) {
	listener.record(recordedEvent{kind: "request_start", iteration: iteration})
}

func (listener *recordingListener) OnRequestEnd(iteration int) {
	listener.record(recordedEvent{kind: "request_end", iteration: iteration})
}

func (listener *recordingListener) OnAssistantMessage(content, reasoning string) {
	listener.record(recordedEvent{kind: "assistant", content: content, reasoning: reasoning})
}

func (listener *recordingListener) OnToolCall(name string, arguments map[string]any) {
	listener.record(recordedEvent{kind: "tool_call", toolName: name, toolArguments: arguments})
}

func (listener *recordingListener) OnToolResult(name string, result string, success bool, durationMilliseconds int64) {
	listener.record(recordedEvent{
		kind:           "tool_result",
		toolName:       name,
		toolResult:     result,
		toolSuccess:    success,
		toolDurationMs: durationMilliseconds,
	})
}

// TestListenerReceivesEventsAcrossToolCallLoop verifies that an Agent.Run
// which involves a tool call followed by a final assistant message fires the
// full lifecycle of listener events in the correct order, including any
// out-of-band reasoning text.
func TestListenerReceivesEventsAcrossToolCallLoop(t *testing.T) {
	provider := &scriptedProvider{
		responses: []ai_provider.ChatCompletionResponse{
			{
				Choices: []ai_provider.Choice{
					{
						Message: ai_provider.Message{
							Role:             "assistant",
							Content:          "",
							ReasoningContent: "I should call the test tool first.",
							ToolCalls: []ai_provider.ToolCall{
								{
									ID:   "call_1",
									Type: "function",
									Function: ai_provider.ToolCallFunction{
										Name:      "echo",
										Arguments: `{"message":"hello"}`,
									},
								},
							},
						},
						FinishReason: "tool_calls",
					},
				},
			},
			{
				Choices: []ai_provider.Choice{
					{
						Message: ai_provider.Message{
							Role:    "assistant",
							Content: "All done.",
						},
						FinishReason: "stop",
					},
				},
			},
		},
	}

	configuration := &config.AgentConfiguration{MaxIterations: 5}
	codeAgent := NewAgent(provider, configuration, nil, t.TempDir())

	listener := &recordingListener{}
	codeAgent.SetListener(listener)

	codeAgent.AddUserMessage("please run the echo tool then stop")

	executor := func(toolName string, arguments map[string]any) (string, error) {
		return "echoed: " + arguments["message"].(string), nil
	}

	if err := codeAgent.Run(context.Background(), executor); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	expectedKinds := []string{
		"request_start",
		"request_end",
		"assistant",
		"tool_call",
		"tool_result",
		"request_start",
		"request_end",
		"assistant",
	}

	if len(listener.events) != len(expectedKinds) {
		t.Fatalf("expected %d events, got %d: %+v", len(expectedKinds), len(listener.events), listener.events)
	}

	for index, expected := range expectedKinds {
		if listener.events[index].kind != expected {
			t.Errorf("event %d: expected kind %q, got %q", index, expected, listener.events[index].kind)
		}
	}

	// First assistant event should expose reasoning_content from the
	// scripted response.
	firstAssistant := listener.events[2]
	if firstAssistant.reasoning != "I should call the test tool first." {
		t.Errorf("expected first assistant reasoning to be propagated, got %q", firstAssistant.reasoning)
	}

	// Tool call event should carry the parsed arguments.
	toolCall := listener.events[3]
	if toolCall.toolName != "echo" {
		t.Errorf("expected tool name echo, got %q", toolCall.toolName)
	}
	if toolCall.toolArguments["message"] != "hello" {
		t.Errorf("expected tool argument message=hello, got %+v", toolCall.toolArguments)
	}

	// Tool result event should carry success and the executor's output.
	toolResult := listener.events[4]
	if !toolResult.toolSuccess {
		t.Errorf("expected tool result success to be true")
	}
	if toolResult.toolResult != "echoed: hello" {
		t.Errorf("expected tool result %q, got %q", "echoed: hello", toolResult.toolResult)
	}

	// Final assistant event should carry the user-visible content.
	finalAssistant := listener.events[7]
	if finalAssistant.content != "All done." {
		t.Errorf("expected final assistant content %q, got %q", "All done.", finalAssistant.content)
	}
}

// TestListenerDefaultsToNoop ensures an Agent constructed without an
// explicit listener still runs successfully and tolerates a nil SetListener
// call without panicking.
func TestListenerDefaultsToNoop(t *testing.T) {
	provider := &scriptedProvider{
		responses: []ai_provider.ChatCompletionResponse{
			{
				Choices: []ai_provider.Choice{
					{
						Message:      ai_provider.Message{Role: "assistant", Content: "done"},
						FinishReason: "stop",
					},
				},
			},
		},
	}

	codeAgent := NewAgent(provider, &config.AgentConfiguration{MaxIterations: 3}, nil, t.TempDir())
	codeAgent.SetListener(nil) // should revert to no-op, not panic

	codeAgent.AddUserMessage("hello")
	if err := codeAgent.Run(context.Background(), func(string, map[string]any) (string, error) {
		return "", nil
	}); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
}
