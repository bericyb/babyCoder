package agent

// Listener receives lifecycle events from an Agent run. Implementations are
// used to surface in-flight progress (model reasoning, assistant content,
// tool invocations and their results) to whichever transport is wrapping the
// agent — typically a terminal UI, but it could equally be a log or test
// harness.
//
// All methods are called synchronously from the agent loop. Implementations
// MUST NOT block for long, and MUST be safe to call from the goroutine that
// drives Agent.Run.
//
// A nil-safe no-op implementation is installed by NewAgent, so an Agent
// without a Listener simply does nothing extra.
type Listener interface {
	// OnRequestStart fires immediately before the agent issues a chat
	// completion request to the AI provider. iteration is zero-based.
	OnRequestStart(iteration int)

	// OnRequestEnd fires once the chat completion request has returned
	// (successfully or not) and before any per-message events are emitted.
	OnRequestEnd(iteration int)

	// OnAssistantMessage fires once per assistant message produced by the
	// model. reasoning is the out-of-band reasoning text the model emitted
	// (may be empty); content is the user-visible assistant text (may be
	// empty when the message is purely a tool call).
	OnAssistantMessage(content string, reasoning string)

	// OnToolCall fires immediately before each tool call is dispatched to
	// the executor. arguments is the already-parsed argument map.
	OnToolCall(name string, arguments map[string]any)

	// OnToolResult fires after a tool call completes. success reflects
	// whether the executor returned a non-nil error; result is the raw
	// string returned to the model (truncated by the listener if desired).
	OnToolResult(name string, result string, success bool, durationMilliseconds int64)
}

// noopListener is the default Listener installed on every Agent so the loop
// can call listener methods unconditionally.
type noopListener struct{}

func (noopListener) OnRequestStart(int)                   {}
func (noopListener) OnRequestEnd(int)                     {}
func (noopListener) OnAssistantMessage(string, string)    {}
func (noopListener) OnToolCall(string, map[string]any)    {}
func (noopListener) OnToolResult(string, string, bool, int64) {}
