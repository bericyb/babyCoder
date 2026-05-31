package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// ANSI escape sequences used by the console listener. Kept private so the
// rest of the package keeps a single source of truth for terminal styling.
const (
	colorReset      = "\033[0m"
	colorDim        = "\033[2m"
	colorBold       = "\033[1m"
	colorItalic     = "\033[3m"
	colorGray       = "\033[90m"
	colorCyan       = "\033[36m"
	colorGreen      = "\033[32m"
	colorRed        = "\033[31m"
	colorYellow     = "\033[33m"
	colorBlue       = "\033[34m"
	colorBrightBlue = "\033[94m"
)

// ConsoleListener implements agent.Listener for an interactive terminal. It
// owns a single Spinner that is shown only while a chat completion request
// is in flight and prints structured, colorized output for every assistant
// message, tool call, and tool result.
//
// ConsoleListener is safe to share between sequential agent runs but is not
// designed for concurrent runs sharing the same instance — that would
// interleave output unpredictably.
type ConsoleListener struct {
	output          io.Writer
	spinner         *Spinner
	spinnerMutex    sync.Mutex
	toolResultLines int // Maximum lines of tool result to display (0 = no limit)
	toolResultChars int // Maximum characters of tool result to display (0 = no limit)
}

// NewConsoleListener returns a ConsoleListener that writes to stdout with
// sensible defaults for an interactive REPL: tool results truncated to a
// short preview so the terminal does not get flooded by large file reads.
func NewConsoleListener() *ConsoleListener {
	return &ConsoleListener{
		output:          os.Stdout,
		toolResultLines: 8,
		toolResultChars: 600,
	}
}

// OnRequestStart begins a spinner while the model is generating.
func (listener *ConsoleListener) OnRequestStart(iteration int) {
	listener.spinnerMutex.Lock()
	defer listener.spinnerMutex.Unlock()

	if listener.spinner != nil {
		listener.spinner.Stop()
	}
	message := "Thinking..."
	if iteration > 0 {
		message = fmt.Sprintf("Thinking... (step %d)", iteration+1)
	}
	listener.spinner = NewSpinner(message)
	listener.spinner.Start()
}

// OnRequestEnd stops the in-flight spinner.
func (listener *ConsoleListener) OnRequestEnd(iteration int) {
	listener.spinnerMutex.Lock()
	defer listener.spinnerMutex.Unlock()

	if listener.spinner != nil {
		listener.spinner.Stop()
		listener.spinner = nil
	}
}

// OnAssistantMessage prints any reasoning text the model produced (dimmed)
// followed by the assistant's user-facing content.
func (listener *ConsoleListener) OnAssistantMessage(content string, reasoning string) {
	reasoning = strings.TrimSpace(reasoning)
	if reasoning != "" {
		listener.printReasoning(reasoning)
	}

	content = strings.TrimSpace(content)
	if content != "" {
		listener.printAssistantContent(content)
	}
}

// OnToolCall renders the upcoming tool invocation as a labeled card with
// each argument on its own line, truncating large string values so the
// terminal stays readable.
func (listener *ConsoleListener) OnToolCall(name string, arguments map[string]any) {
	fmt.Fprintf(listener.output, "%s%s→%s %s%s%s\n",
		colorBrightBlue, colorBold, colorReset,
		colorCyan, name, colorReset,
	)

	if len(arguments) == 0 {
		return
	}

	// Sort keys for stable output.
	keys := make([]string, 0, len(arguments))
	for key := range arguments {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Compute padding for aligned key column.
	maxKeyWidth := 0
	for _, key := range keys {
		if len(key) > maxKeyWidth {
			maxKeyWidth = len(key)
		}
	}

	for _, key := range keys {
		formatted := formatArgumentValue(arguments[key])
		fmt.Fprintf(listener.output, "  %s%-*s%s  %s\n",
			colorGray, maxKeyWidth, key, colorReset,
			formatted,
		)
	}
}

// OnToolResult prints a one-line status (success / failure + duration) and a
// truncated preview of the tool output.
func (listener *ConsoleListener) OnToolResult(name string, result string, success bool, durationMilliseconds int64) {
	marker := fmt.Sprintf("%s✓%s", colorGreen, colorReset)
	label := "ok"
	if !success {
		marker = fmt.Sprintf("%s✗%s", colorRed, colorReset)
		label = "failed"
	}

	fmt.Fprintf(listener.output, "  %s %s%s %s(%s)%s\n",
		marker,
		colorDim, label,
		colorGray, formatDuration(durationMilliseconds), colorReset,
	)

	preview := strings.TrimSpace(result)
	if preview == "" {
		fmt.Fprintln(listener.output)
		return
	}

	truncated, didTruncate := truncatePreview(preview, listener.toolResultLines, listener.toolResultChars)
	for _, line := range strings.Split(truncated, "\n") {
		fmt.Fprintf(listener.output, "  %s│%s %s\n", colorGray, colorReset, line)
	}
	if didTruncate {
		fmt.Fprintf(listener.output, "  %s│ … (truncated)%s\n", colorDim, colorReset)
	}
	fmt.Fprintln(listener.output)
}

// printReasoning renders the model's reasoning text in dimmed italics under a
// thought-bubble header.
func (listener *ConsoleListener) printReasoning(reasoning string) {
	fmt.Fprintf(listener.output, "%s%s💭 thinking%s\n", colorDim, colorItalic, colorReset)
	for _, line := range strings.Split(reasoning, "\n") {
		fmt.Fprintf(listener.output, "%s%s  %s%s\n", colorDim, colorItalic, line, colorReset)
	}
	fmt.Fprintln(listener.output)
}

// printAssistantContent renders the assistant's user-visible message with a
// subtle prefix so it is visually distinguishable from tool output.
func (listener *ConsoleListener) printAssistantContent(content string) {
	fmt.Fprintf(listener.output, "%s%sAssistant%s\n", colorBold, colorYellow, colorReset)
	for _, line := range strings.Split(content, "\n") {
		fmt.Fprintf(listener.output, "  %s\n", line)
	}
	fmt.Fprintln(listener.output)
}

// formatArgumentValue produces a single-line, human-friendly rendering of an
// argument value. Strings are quoted; very long strings are summarized with a
// character count. Maps and slices are pretty-printed compactly; if they
// would exceed a reasonable inline length they are summarized.
func formatArgumentValue(value any) string {
	const maxInlineLength = 120
	const stringPreviewLength = 80

	switch typedValue := value.(type) {
	case string:
		if len(typedValue) > stringPreviewLength {
			return fmt.Sprintf("%s\"%s…\"%s %s<%d chars>%s",
				colorGreen, escapeForPreview(typedValue[:stringPreviewLength]), colorReset,
				colorDim, len(typedValue), colorReset,
			)
		}
		return fmt.Sprintf("%s%q%s", colorGreen, typedValue, colorReset)
	case bool:
		return fmt.Sprintf("%s%t%s", colorYellow, typedValue, colorReset)
	case float64:
		// JSON numbers always decode to float64. Render integers without
		// a decimal point for readability.
		if typedValue == float64(int64(typedValue)) {
			return fmt.Sprintf("%s%d%s", colorYellow, int64(typedValue), colorReset)
		}
		return fmt.Sprintf("%s%g%s", colorYellow, typedValue, colorReset)
	case nil:
		return fmt.Sprintf("%snull%s", colorDim, colorReset)
	default:
		// Fall back to compact JSON for slices and maps.
		encoded, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprintf("%s<unrenderable>%s", colorRed, colorReset)
		}
		rendered := string(encoded)
		if len(rendered) > maxInlineLength {
			return fmt.Sprintf("%s<%d chars of %T>%s",
				colorDim, len(rendered), value, colorReset,
			)
		}
		return rendered
	}
}

// escapeForPreview replaces newlines and tabs with visible escape sequences
// so a multi-line value still renders on a single line in a tool-call card.
func escapeForPreview(value string) string {
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, "\r", "\\r")
	value = strings.ReplaceAll(value, "\t", "\\t")
	return value
}

// truncatePreview shortens a multi-line string to at most maxLines lines and
// maxCharacters characters, returning the truncated value and a flag
// indicating whether truncation occurred. A zero value for either limit
// disables that limit.
func truncatePreview(text string, maxLines int, maxCharacters int) (string, bool) {
	truncated := false

	if maxLines > 0 {
		lines := strings.Split(text, "\n")
		if len(lines) > maxLines {
			lines = lines[:maxLines]
			text = strings.Join(lines, "\n")
			truncated = true
		}
	}

	if maxCharacters > 0 && len(text) > maxCharacters {
		text = text[:maxCharacters]
		truncated = true
	}

	return text, truncated
}

// formatDuration renders a millisecond duration in the most appropriate unit.
func formatDuration(milliseconds int64) string {
	if milliseconds < 1000 {
		return fmt.Sprintf("%dms", milliseconds)
	}
	duration := time.Duration(milliseconds) * time.Millisecond
	return duration.Truncate(10 * time.Millisecond).String()
}
