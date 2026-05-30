package main

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
	"github.com/exar/babycoder/internal/logging"
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

func main() {
	// Initialize logging to file
	workingDirectory, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get working directory: %v\n", err)
		os.Exit(1)
	}

	logDirectory := filepath.Join(workingDirectory, ".babycoder")
	logger, err := logging.NewLogger(logDirectory, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize logging: %v\n", err)
	} else {
		defer logger.Close()
	}

	// If no arguments, start interactive mode
	if len(os.Args) < 2 {
		runInteractive()
		return
	}

	// Any arguments = non-interactive mode with prompt
	prompt := strings.Join(os.Args[1:], " ")
	runNonInteractive(prompt)
}

// agentContext holds all shared dependencies for agent initialization
type agentContext struct {
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

// initializeAgentContext creates and initializes all shared agent dependencies
func initializeAgentContext(sessionTitle string) (*agentContext, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	configuration, err := config.LoadOrDefaultConfiguration(workingDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	databasePath := filepath.Join(workingDirectory, ".babycoder.db")
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
	codeAnalyzer := analyzer.NewAnalyzer(workingDirectory)
	testRunner := testrunner.NewTestRunner(workingDirectory)

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

	toolExecutor := func(toolName string, arguments map[string]interface{}) (string, error) {
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

	return &agentContext{
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

// createAgentFactory returns a factory function for creating sub-agents
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
		subToolExecutor := func(toolName string, arguments map[string]interface{}) (string, error) {
			return subToolRegistry.Execute(toolName, arguments)
		}
		return subAgent, subToolExecutor, nil
	}
}

func runInteractive() {
	ctx, err := initializeAgentContext("Interactive session")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}
	defer ctx.database.Close()

	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║                   babyCoder Interactive                    ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("Model: %s | Endpoint: %s\n", ctx.provider.GetModel(), ctx.configuration.AIProvider.Endpoint)
	fmt.Println()

	fmt.Println("Analyzing project...")
	go ctx.codeAnalyzer.Analyze()
	go ctx.testRunner.RunTests("")

	// Check if dream was loaded
	dreamPath := filepath.Join(ctx.workingDirectory, ".babycoder", "dream.txt")
	if _, err := os.Stat(dreamPath); err == nil {
		if dreamBytes, err := os.ReadFile(dreamPath); err == nil && len(dreamBytes) > 0 {
			fmt.Println("💭 Project memory loaded")
		}
	}

	reader := bufio.NewReader(os.Stdin)
	agentContext := context.Background()

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

		if strings.HasPrefix(input, "#") {
			rule := strings.TrimSpace(strings.TrimPrefix(input, "#"))
			if rule == "" {
				fmt.Println("⚠ Rule cannot be empty")
				continue
			}
			if err := ctx.rulesManager.AddRule(rule); err != nil {
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

		ctx.codeAgent.AddUserMessage(input)
		fmt.Println()

		spinner := ui.NewSpinner("Thinking...")
		spinner.Start()
		err = ctx.codeAgent.Run(agentContext, ctx.toolExecutor)
		spinner.Stop()

		if err != nil {
			fmt.Printf("Agent error: %v\n\n", err)
			continue
		}

		if err := ctx.testRunner.RunIfDirty(); err != nil {
			log.Printf("Warning: test run failed: %v\n", err)
		}

		messages := ctx.codeAgent.GetMessages()
		if len(messages) > 0 {
			lastMessage := messages[len(messages)-1]
			if lastMessage.Role == "assistant" && lastMessage.Content != "" {
				fmt.Printf("Assistant: %s\n\n", lastMessage.Content)
			}
		}
	}
}

func runNonInteractive(prompt string) {
	ctx, err := initializeAgentContext(truncateString(prompt, 50))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}
	defer ctx.database.Close()

	ctx.codeAgent.AddUserMessage(prompt)
	agentContext := context.Background()

	spinner := ui.NewSpinner("Thinking...")
	spinner.Start()
	err = ctx.codeAgent.Run(agentContext, ctx.toolExecutor)
	spinner.Stop()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Agent error: %v\n", err)
		os.Exit(1)
	}

	messages := ctx.codeAgent.GetMessages()
	if len(messages) > 0 {
		lastMessage := messages[len(messages)-1]
		if lastMessage.Role == "assistant" && lastMessage.Content != "" {
			fmt.Printf("%s\n", lastMessage.Content)
		}
	}

	// Run dream update immediately before exit
	fmt.Println("\n💭 Updating project memory...")
	ctx.codeAgent.UpdateDreamNow(agentContext)
}

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

func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength-3] + "..."
}
