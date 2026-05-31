package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/exar/babycoder/internal/services/ai_provider"
)

// updateDream updates the project dream file based on the current session
func (agent *Agent) updateDream(ctx context.Context) {
	if len(agent.messages) <= 2 {
		return // Not enough content to summarize
	}
	
	if agent.projectRoot == "" {
		return // No project root set
	}
	
	dreamPath := filepath.Join(agent.projectRoot, ".babycoder", "dream.txt")
	
	// Step 1: Summarize session
	summary := agent.summarizeSession(ctx)
	if summary == "" {
		log.Println("Dream: failed to generate session summary")
		return
	}
	
	// Step 2: Read current dream
	currentDream, err := os.ReadFile(dreamPath)
	if err != nil {
		currentDream = []byte("") // First time, empty dream
	}
	
	// Step 3: Decide if update needed
	result := agent.decideDreamUpdate(ctx, string(currentDream), summary)
	if result == "" || strings.TrimSpace(result) == "NO_UPDATE" {
		return
	}
	
	// Step 4: Write updated dream
	if err := os.WriteFile(dreamPath, []byte(result), 0644); err != nil {
		log.Printf("Dream: failed to write file: %v\n", err)
	}
}

// summarizeSession generates a 2-3 sentence summary of the current session
func (agent *Agent) summarizeSession(ctx context.Context) string {
	// Get last 10 messages
	start := len(agent.messages) - 10
	if start < 0 {
		start = 0
	}
	recentMessages := agent.messages[start:]
	
	// Format into text
	var msgText strings.Builder
	for _, msg := range recentMessages {
		content := msg.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		msgText.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, content))
	}
	
	prompt := "Summarize this coding session in 2-3 sentences. Focus on what changed:\n\n" + msgText.String()
	
	request := ai_provider.ChatCompletionRequest{
		Messages: []ai_provider.Message{{Role: "user", Content: prompt}},
	}
	
	response, err := agent.provider.ChatCompletion(ctx, request)
	if err != nil || len(response.Choices) == 0 {
		return ""
	}
	
	return strings.TrimSpace(response.Choices[0].Message.Content)
}

// decideDreamUpdate asks the LLM whether to update the dream file
func (agent *Agent) decideDreamUpdate(ctx context.Context, currentDream, sessionSummary string) string {
	if currentDream == "" {
		currentDream = "No project summary yet."
	}
	
	prompt := fmt.Sprintf(`You are an expert technical writer tasked with maintaining a high-level, persistent summary of a coding project's context. Your goal is to synthesize the new session information into the existing 'Project Context' without losing any critical facts from the old context.

--- PROJECT CONTEXT (MUST RETAIN ALL FACTS) ---
%s
----------------------------------------------

--- NEW SESSION ACTIVITY SUMMARY ---
%s
----------------------------------------------

Instructions:
1. Review the "NEW SESSION ACTIVITY SUMMARY" against the "PROJECT CONTEXT".
2. If the new activity introduces a major feature, architectural decision, or critical piece of knowledge not covered in the context, integrate it into the existing summary naturally.
3. Do NOT simply summarize the session; *update* the project narrative. Maintain the tone and scope of the original context.
4. If the new activity is minor (e.g., fixing a typo, adding boilerplate code), output exactly: NO_UPDATE.
5. Output only the final, integrated Project Context text. Do not include any introductory or concluding remarks.`, currentDream, sessionSummary)
	request := ai_provider.ChatCompletionRequest{
		Messages: []ai_provider.Message{{Role: "user", Content: prompt}},
	}
	
	response, err := agent.provider.ChatCompletion(ctx, request)
	if err != nil || len(response.Choices) == 0 {
		return "NO_UPDATE"
	}
	
	return strings.TrimSpace(response.Choices[0].Message.Content)
}