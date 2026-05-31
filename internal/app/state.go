// Package app assembles the long-lived dependencies for a babyCoder run
// (configuration, database, AI provider, domain services, and a configured
// agent.Agent) and exposes the two top-level execution modes: an interactive
// REPL and a single-shot non-interactive run.
package app

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/exar/babycoder/internal/config"
	"github.com/exar/babycoder/internal/prompts"
	"github.com/exar/babycoder/internal/rules"
	"github.com/exar/babycoder/internal/services/agent"
	"github.com/exar/babycoder/internal/services/ai_provider"
	"github.com/exar/babycoder/internal/services/analyzer"
	"github.com/exar/babycoder/internal/services/testrunner"
	"github.com/exar/babycoder/internal/services/tools"
	"github.com/exar/babycoder/internal/storage"
	"github.com/exar/babycoder/internal/ui"
	"github.com/google/uuid"
)

// State bundles together the long-lived dependencies for a babyCoder run:
// configuration, infrastructure (database, AI provider), domain services
// (analyzer, test runner, prompt/rules managers, tool registry), and a
// configured agent.Agent ready to execute. It is built once at startup by
// New and consumed by RunInteractive / RunNonInteractive.
type State struct {
	workingDirectory string
	configuration    *config.Configuration
	database         *storage.Database
	provider         ai_provider.Provider
	sessionID        string
	codeAnalyzer     *analyzer.Analyzer
	testRunner       *testrunner.TestRunner
	promptManager    *prompts.PromptManager
	rulesManager     *rules.RulesManager
	codeAgent        *agent.Agent
	toolRegistry     *tools.ToolRegistry
	toolExecutor     agent.ToolExecutor
}

// New creates and initializes all shared dependencies for a babyCoder
// session. The sessionTitle is stored on the newly created primary session row.
func New(sessionTitle string) (*State, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	configuration, err := config.LoadOrDefaultConfiguration(workingDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	databasePath := filepath.Join(workingDirectory, ".babycoder", "babycoder.db")
	database, err := storage.NewDatabase(databasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	provider, err := ai_provider.NewProvider(&configuration.AIProvider)
	if err != nil {
		database.Close()
		return nil, fmt.Errorf("failed to create AI provider: %w", err)
	}

	sessionID := createNewSession(database, workingDirectory, sessionTitle)
	codeAnalyzer := analyzer.NewAnalyzer(workingDirectory, provider)
	testRunner := testrunner.NewTestRunner(workingDirectory, provider)

	promptManager, err := prompts.NewPromptManagerFromConfig(
		workingDirectory,
		configuration.Prompts.MainAgent,
		configuration.Prompts.SubAgent,
	)
	if err != nil {
		database.Close()
		return nil, fmt.Errorf("failed to create prompt manager: %w", err)
	}

	rulesManager := rules.NewRulesManager(workingDirectory)
	toolRegistry := tools.NewToolRegistry(workingDirectory, sessionID, codeAnalyzer, testRunner, database)
	codeAgent := agent.NewAgent(provider, &configuration.Agent, database, workingDirectory)
	codeAgent.SetSessionID(sessionID)

	// Build system prompt with rules and dream context
	rulesText, _ := rulesManager.GetRulesAsText()

	dreamPath := filepath.Join(workingDirectory, ".babycoder", "dream.txt")
	dreamContent := ""
	if dreamBytes, err := os.ReadFile(dreamPath); err == nil && len(dreamBytes) > 0 {
		dreamContent = string(dreamBytes)
	}

	mainAgentPrompt := promptManager.BuildSystemPrompt(prompts.MainAgent, rulesText, dreamContent)
	codeAgent.SetSystemPrompt(mainAgentPrompt)

	// Register all tools
	for _, toolDef := range toolRegistry.GetAllDefinitions() {
		codeAgent.RegisterTool(toolDef)
	}

	toolExecutor := func(toolName string, arguments map[string]any) (string, error) {
		return toolRegistry.Execute(toolName, arguments)
	}

	// Create and register sub-agent tool
	agentFactory := createAgentFactory(provider, configuration, database, workingDirectory, codeAnalyzer, testRunner)
	subAgentTool := &tools.SubAgentTool{
		ProjectRoot:   workingDirectory,
		ParentSession: sessionID,
		Database:      database,
		AgentFactory:  agentFactory,
		PromptManager: promptManager,
	}
	toolRegistry.RegisterTool(subAgentTool)
	codeAgent.RegisterTool(subAgentTool.GetDefinition())

	// Attach a console listener so the user sees the model's reasoning,
	// assistant content, and tool invocations in real time between
	// iterations of the agent loop.
	codeAgent.SetListener(ui.NewConsoleListener())

	return &State{
		workingDirectory: workingDirectory,
		configuration:    configuration,
		database:         database,
		provider:         provider,
		sessionID:        sessionID,
		codeAnalyzer:     codeAnalyzer,
		testRunner:       testRunner,
		promptManager:    promptManager,
		rulesManager:     rulesManager,
		codeAgent:        codeAgent,
		toolRegistry:     toolRegistry,
		toolExecutor:     toolExecutor,
	}, nil
}

// Close releases resources held by the State. Safe to call once at shutdown.
func (state *State) Close() error {
	if state.database != nil {
		return state.database.Close()
	}
	return nil
}

// RunInteractive starts the interactive REPL loop. Each user line is sent to
// the configured code agent; lines beginning with '#' are treated as new
// project rules and '/exit' terminates the loop.
func (state *State) RunInteractive() {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║                   babyCoder Interactive                    ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("Model: %s | Endpoint: %s\n", state.provider.GetModel(), state.configuration.AIProvider.Endpoint)
	fmt.Println()

	// Background analysis and tests only run once the agent has supplied
	// a build/test command via check_code_status / run_tests.

	// Check if dream was loaded
	dreamPath := filepath.Join(state.workingDirectory, ".babycoder", "dream.txt")
	if _, err := os.Stat(dreamPath); err == nil {
		if dreamBytes, err := os.ReadFile(dreamPath); err == nil && len(dreamBytes) > 0 {
			fmt.Println("💭 Project memory loaded")
		}
	}

	reader := bufio.NewReader(os.Stdin)
	runContext := context.Background()

	fmt.Println()
	for {
		fmt.Print("You: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("\nError reading input: %v\n", err)
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if rule, ok := strings.CutPrefix(input, "#"); ok {
			if rule == "" {
				fmt.Println("⚠ Rule cannot be empty")
				continue
			}
			if err := state.rulesManager.AddRule(rule); err != nil {
				fmt.Printf("⚠ Failed to add rule: %v\n\n", err)
			} else {
				fmt.Printf("✓ Rule added\n\n")
			}
			continue
		}

		if input == "/exit" {
			fmt.Println("\nGoodbye!")
			return
		}

		state.codeAgent.AddUserMessage(input)
		fmt.Println()

		err = state.codeAgent.Run(runContext, state.toolExecutor)

		if err != nil {
			fmt.Printf("Agent error: %v\n\n", err)
			continue
		}

		if err := state.testRunner.RunIfDirty(runContext); err != nil {
			log.Printf("Warning: test run failed: %v\n", err)
		}
	}
}

// RunNonInteractive runs the agent once against the supplied prompt and
// prints the final assistant message. It also triggers an immediate dream
// (project memory) update before returning.
func (state *State) RunNonInteractive(prompt string) error {
	state.codeAgent.AddUserMessage(prompt)
	runContext := context.Background()

	err := state.codeAgent.Run(runContext, state.toolExecutor)

	if err != nil {
		return fmt.Errorf("agent error: %w", err)
	}

	// Run dream update immediately before exit
	fmt.Println("\n💭 Updating project memory...")
	state.codeAgent.UpdateDreamNow(runContext)
	return nil
}

// createAgentFactory returns a factory function for creating sub-agents. The
// factory is handed to the sub-agent tool so each sub-agent invocation gets
// its own agent.Agent, tool registry, and tool executor bound to a fresh
// session ID.
func createAgentFactory(
	provider ai_provider.Provider,
	configuration *config.Configuration,
	database *storage.Database,
	workingDirectory string,
	codeAnalyzer *analyzer.Analyzer,
	testRunner *testrunner.TestRunner,
) tools.AgentFactory {
	return func(subSessionID string) (tools.AgentInterface, agent.ToolExecutor, error) {
		subAgent := agent.NewAgent(provider, &configuration.Agent, database, workingDirectory)
		subAgent.SetSessionID(subSessionID)
		subToolRegistry := tools.NewToolRegistry(workingDirectory, subSessionID, codeAnalyzer, testRunner, database)
		for _, toolDef := range subToolRegistry.GetAllDefinitions() {
			subAgent.RegisterTool(toolDef)
		}
		subToolExecutor := func(toolName string, arguments map[string]any) (string, error) {
			return subToolRegistry.Execute(toolName, arguments)
		}
		return subAgent, subToolExecutor, nil
	}
}

// createNewSession inserts a new primary session row and returns its ID. A
// failure here is logged but non-fatal: the run can still proceed without
// persisted history.
func createNewSession(database *storage.Database, projectRoot string, title string) string {
	sessionID := uuid.New().String()
	session := &storage.Session{
		ID:              sessionID,
		ProjectRoot:     projectRoot,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		Title:           title,
		Status:          "active",
		ParentSessionID: nil,
		SessionType:     "primary",
		TaskDescription: "",
	}

	if err := database.CreateSession(session); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create session: %v\n", err)
	}

	return sessionID
}
