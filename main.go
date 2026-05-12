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
	"github.com/exar/babycoder/internal/services/doctracker"
	"github.com/exar/babycoder/internal/services/testrunner"
	"github.com/exar/babycoder/internal/services/tools"
	"github.com/exar/babycoder/internal/storage"
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

	command := os.Args[1]

	switch command {
	case "sessions":
		manageSessions()
	case "init":
		initProject()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("\nAvailable commands:")
		fmt.Println("  (no args)            Start interactive agent mode")
		fmt.Println("  sessions list        List recent sessions")
		fmt.Println("  sessions show <id>   Show details of a session")
		fmt.Println("  sessions delete <id> Delete a session")
		fmt.Println("  init                 Initialize babyCoder in current directory")
		os.Exit(1)
	}
}

func runInteractive() {
	// Load configuration
	workingDirectory, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get working directory: %v\n", err)
		os.Exit(1)
	}

	configuration, err := config.LoadOrDefaultConfiguration(workingDirectory)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize database
	databasePath := filepath.Join(workingDirectory, ".babycoder.db")
	database, err := storage.NewDatabase(databasePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	// Create AI provider
	provider, err := ai_provider.NewProvider(&configuration.AIProvider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create AI provider: %v\n", err)
		os.Exit(1)
	}

	// Print welcome message
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║                   babyCoder Interactive                    ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("Model:    %s\n", provider.GetModel())
	fmt.Printf("Endpoint: %s\n", configuration.AIProvider.Endpoint)
	fmt.Println()
	fmt.Println("Type your message and press Enter to send.")
	fmt.Println("Type /exit to quit.")
	fmt.Println("Type /new to start a new session.")
	fmt.Println()

	// Create initial session
	sessionID := createNewSession(database, workingDirectory, "Interactive session")
	
	// Create code analyzer
	codeAnalyzer := analyzer.NewAnalyzer(workingDirectory)
	
	// Create test runner
	testRunner := testrunner.NewTestRunner(workingDirectory)
	
	// Create documentation tracker
	docTracker := doctracker.NewDocTracker(workingDirectory, database, provider, 2) // 2 workers
	docTracker.Start()
	defer docTracker.Stop()
	
	// Run initial analysis and tests in background
	fmt.Println("Analyzing project and running tests...")
	go codeAnalyzer.Analyze()
	go testRunner.RunTests("")
	
	// Create prompt manager and load configuration
	promptManager := prompts.NewPromptManager(workingDirectory)
	if configuration.Prompts.MainAgent != "" || configuration.Prompts.SubAgent != "" {
		promptConfig := map[string]interface{}{
			"main_agent": configuration.Prompts.MainAgent,
			"sub_agent":  configuration.Prompts.SubAgent,
		}
		if err := promptManager.LoadFromConfig(promptConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load prompt configuration: %v\n", err)
		}
	}
	
	// Create rules manager
	rulesManager := rules.NewRulesManager(workingDirectory)
	
	// Create tool registry with all services
	toolRegistry := tools.NewToolRegistry(workingDirectory, sessionID, codeAnalyzer, testRunner, docTracker, database)
	
	// Create agent
	codeAgent := agent.NewAgent(provider, &configuration.Agent, database)
	codeAgent.SetSessionID(sessionID)
	
	// Set system prompt from prompt manager with rules injection
	mainAgentPrompt := promptManager.GetPrompt(prompts.MainAgent)
	rulesText, err := rulesManager.GetRulesAsText()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load rules: %v\n", err)
	} else if rulesText != "" {
		// Inject rules after main prompt
		mainAgentPrompt = mainAgentPrompt + "\n\n" + rulesText
	}
	codeAgent.SetSystemPrompt(mainAgentPrompt)

	// Register all tools with the agent
	for _, toolDef := range toolRegistry.GetAllDefinitions() {
		codeAgent.RegisterTool(toolDef)
	}

	// Create tool executor that uses the registry
	toolExecutor := func(toolName string, arguments map[string]interface{}) (string, error) {
		return toolRegistry.Execute(toolName, arguments)
	}

	// Create AgentFactory for sub-agents
	agentFactory := func(subSessionID string) (tools.AgentInterface, agent.ToolExecutor, error) {
		// Create new agent instance with same configuration
		subAgent := agent.NewAgent(provider, &configuration.Agent, database)
		subAgent.SetSessionID(subSessionID)
		
		// Sub-agents don't need the main prompt, they'll get their own from SubAgentTool

		// Create new tool registry for sub-agent
		subToolRegistry := tools.NewToolRegistry(workingDirectory, subSessionID, codeAnalyzer, testRunner, docTracker, database)
		
		// Register all tools except run_subagent (prevent infinite recursion)
		for _, toolDef := range subToolRegistry.GetAllDefinitions() {
			subAgent.RegisterTool(toolDef)
		}

		// Create tool executor for sub-agent
		subToolExecutor := func(toolName string, arguments map[string]interface{}) (string, error) {
			return subToolRegistry.Execute(toolName, arguments)
		}

		return subAgent, subToolExecutor, nil
	}

	// Register SubAgentTool dynamically with the factory
	subAgentTool := &tools.SubAgentTool{
		ProjectRoot:   workingDirectory,
		ParentSession: sessionID,
		Database:      database,
		AgentFactory:  agentFactory,
		PromptManager: promptManager,
	}
	toolRegistry.RegisterTool(subAgentTool)
	codeAgent.RegisterTool(subAgentTool.GetDefinition())

	// Create reader for STDIN
	reader := bufio.NewReader(os.Stdin)
	ctx := context.Background()

	for {
		fmt.Print("You: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("\nError reading input: %v\n", err)
			break
		}

		input = strings.TrimSpace(input)

		// Handle empty input
		if input == "" {
			continue
		}

		// Handle rule creation (# prefix)
		if strings.HasPrefix(input, "#") {
			rule := strings.TrimSpace(strings.TrimPrefix(input, "#"))
			if rule == "" {
				fmt.Println("⚠ Rule cannot be empty")
				continue
			}
			
			if err := rulesManager.AddRule(rule); err != nil {
				fmt.Printf("⚠ Failed to add rule: %v\n\n", err)
			} else {
				fmt.Printf("✓ Rule added to %s\n\n", rulesManager.GetRulesPath())
			}
			continue
		}

		// Handle commands
		if strings.HasPrefix(input, "/") {
			command := strings.ToLower(input)
			
			switch command {
			case "/exit":
				fmt.Println("\nGoodbye!")
				
				// Update session status
				session, err := database.GetSession(sessionID)
				if err == nil && session != nil {
					session.Status = "completed"
					session.UpdatedAt = time.Now()
					database.UpdateSession(session)
				}
				
				return
				
			case "/new":
				// Mark current session as completed
				session, err := database.GetSession(sessionID)
				if err == nil && session != nil {
					session.Status = "completed"
					session.UpdatedAt = time.Now()
					database.UpdateSession(session)
				}
				
				// Create new session
				sessionID = createNewSession(database, workingDirectory, "Interactive session")
				codeAgent = agent.NewAgent(provider, &configuration.Agent, database)
				codeAgent.SetSessionID(sessionID)
				
				// Reload system prompt with rules
				mainAgentPrompt = promptManager.GetPrompt(prompts.MainAgent)
				rulesText, err = rulesManager.GetRulesAsText()
				if err == nil && rulesText != "" {
					mainAgentPrompt = mainAgentPrompt + "\n\n" + rulesText
				}
				codeAgent.SetSystemPrompt(mainAgentPrompt)
				
				fmt.Printf("\n✓ Started new session: %s\n\n", sessionID)
				continue
				
			case "/help":
				fmt.Println("\nAvailable commands:")
				fmt.Println("  /exit   - Exit babyCoder")
				fmt.Println("  /new    - Start a new session")
				fmt.Println("  /rules  - View current rules")
				fmt.Println("  /help   - Show this help message")
				fmt.Println("\nRule management:")
				fmt.Println("  # <rule> - Add a new rule to rules.md")
				fmt.Println()
				continue
			
			case "/rules":
				rules, err := rulesManager.LoadRules()
				if err != nil {
					fmt.Printf("⚠ Failed to load rules: %v\n\n", err)
					continue
				}
				
				if len(rules) == 0 {
					fmt.Println("\nNo rules defined yet. Add rules with: # <rule>")
				} else {
					fmt.Printf("\nCurrent rules (%s):\n", rulesManager.GetRulesPath())
					for i, rule := range rules {
						fmt.Printf("  %d. %s\n", i+1, rule)
					}
					fmt.Println()
				}
				continue
				
			default:
				fmt.Printf("Unknown command: %s (type /help for available commands)\n\n", input)
				continue
			}
		}

		// Add user message
		codeAgent.AddUserMessage(input)

		// Run agent
		fmt.Println()
		if err := codeAgent.Run(ctx, toolExecutor); err != nil {
			fmt.Printf("Agent error: %v\n\n", err)
			
			// Update session status
			session, err := database.GetSession(sessionID)
			if err == nil && session != nil {
				session.Status = "failed"
				session.UpdatedAt = time.Now()
				database.UpdateSession(session)
			}
			
			continue
		}

		// Agent completed successfully - run tests if needed
		if err := testRunner.RunIfDirty(); err != nil {
			log.Printf("Warning: test run failed: %v\n", err)
		}

		// Update session
		session, err := database.GetSession(sessionID)
		if err == nil && session != nil {
			session.UpdatedAt = time.Now()
			database.UpdateSession(session)
		}

		// Print assistant's final response
		messages := codeAgent.GetMessages()
		if len(messages) > 0 {
			lastMessage := messages[len(messages)-1]
			if lastMessage.Role == "assistant" && lastMessage.Content != "" {
				fmt.Printf("Assistant: %s\n\n", lastMessage.Content)
			}
		}
	}
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
		// Log to file but don't fail
		fmt.Fprintf(os.Stderr, "Warning: failed to create session: %v\n", err)
	}

	return sessionID
}

func runAgent() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: babyCoder agent <prompt>")
		os.Exit(1)
	}

	prompt := os.Args[2]

	// Load configuration
	workingDirectory, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get working directory: %v\n", err)
		os.Exit(1)
	}

	configuration, err := config.LoadOrDefaultConfiguration(workingDirectory)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize database
	databasePath := filepath.Join(workingDirectory, ".babycoder.db")
	database, err := storage.NewDatabase(databasePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	// Create AI provider
	provider, err := ai_provider.NewProvider(&configuration.AIProvider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create AI provider: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Using model: %s\n", provider.GetModel())
	fmt.Printf("Endpoint: %s\n", configuration.AIProvider.Endpoint)
	fmt.Println()

	// Create agent
	codeAgent := agent.NewAgent(provider, &configuration.Agent, database)

	// Create new session
	sessionID := uuid.New().String()
	session := &storage.Session{
		ID:              sessionID,
		ProjectRoot:     workingDirectory,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		Title:           truncateString(prompt, 50),
		Status:          "active",
		ParentSessionID: nil,
		SessionType:     "primary",
		TaskDescription: "",
	}

	if err := database.CreateSession(session); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create session: %v\n", err)
		os.Exit(1)
	}

	codeAgent.SetSessionID(sessionID)
	fmt.Printf("Session ID: %s\n\n", sessionID)

	// Add user prompt
	codeAgent.AddUserMessage(prompt)

	// Create simple tool executor (no tools yet)
	toolExecutor := func(toolName string, arguments map[string]interface{}) (string, error) {
		return "", fmt.Errorf("no tools registered yet")
	}

	// Run agent
	ctx := context.Background()
	if err := codeAgent.Run(ctx, toolExecutor); err != nil {
		session.Status = "failed"
		database.UpdateSession(session)
		fmt.Fprintf(os.Stderr, "Agent error: %v\n", err)
		os.Exit(1)
	}

	// Mark session as completed
	session.Status = "completed"
	session.UpdatedAt = time.Now()
	database.UpdateSession(session)

	// Print final response
	messages := codeAgent.GetMessages()
	if len(messages) > 0 {
		lastMessage := messages[len(messages)-1]
		if lastMessage.Role == "assistant" {
			fmt.Println("\n=== Final Response ===")
			fmt.Println(lastMessage.Content)
		}
	}

	// Print session stats
	stats, err := database.GetSessionStats(sessionID)
	if err == nil {
		fmt.Println("\n=== Session Stats ===")
		fmt.Printf("Messages: %d\n", stats["message_count"])
		fmt.Printf("Tool Executions: %d\n", stats["tool_execution_count"])
		fmt.Printf("Total Tokens: %d\n", stats["total_tokens"])
	}
}

func initProject() {
	workingDirectory, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get working directory: %v\n", err)
		os.Exit(1)
	}

	configPath := fmt.Sprintf("%s/.babycoder.json", workingDirectory)

	// Check if configuration already exists
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("Configuration already exists at: %s\n", configPath)
		return
	}

	// Create default configuration
	configuration := config.DefaultConfiguration()
	configuration.Project.Root = workingDirectory

	// Save configuration
	if err := config.SaveConfiguration(configuration, configPath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save configuration: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Initialized babyCoder configuration at: %s\n", configPath)
	fmt.Println("\nDefault settings:")
	fmt.Printf("  AI Provider: %s\n", configuration.AIProvider.Type)
	fmt.Printf("  Endpoint: %s\n", configuration.AIProvider.Endpoint)
	fmt.Printf("  Model: %s\n", configuration.AIProvider.Model)
	fmt.Printf("  Temperature: %.2f\n", configuration.AIProvider.Temperature)
	fmt.Println("\nEdit .babycoder.json to customize settings.")
}

func manageSessions() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: babyCoder sessions <subcommand>")
		fmt.Println("\nSubcommands:")
		fmt.Println("  list               List recent sessions")
		fmt.Println("  show <id>          Show session details")
		fmt.Println("  delete <id>        Delete a session")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	// Load configuration and database
	workingDirectory, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get working directory: %v\n", err)
		os.Exit(1)
	}

	databasePath := filepath.Join(workingDirectory, ".babycoder.db")
	database, err := storage.NewDatabase(databasePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	switch subcommand {
	case "list":
		listSessions(database, workingDirectory)
	case "show":
		if len(os.Args) < 4 {
			fmt.Println("Usage: babyCoder sessions show <session_id>")
			os.Exit(1)
		}
		showSession(database, os.Args[3])
	case "delete":
		if len(os.Args) < 4 {
			fmt.Println("Usage: babyCoder sessions delete <session_id>")
			os.Exit(1)
		}
		deleteSession(database, os.Args[3])
	default:
		fmt.Printf("Unknown subcommand: %s\n", subcommand)
		os.Exit(1)
	}
}

func listSessions(database *storage.Database, projectRoot string) {
	sessions, err := database.ListSessions(projectRoot, 20)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list sessions: %v\n", err)
		os.Exit(1)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		return
	}

	fmt.Printf("Recent sessions (showing %d):\n\n", len(sessions))
	for _, session := range sessions {
		fmt.Printf("ID:      %s\n", session.ID)
		fmt.Printf("Title:   %s\n", session.Title)
		fmt.Printf("Status:  %s\n", session.Status)
		fmt.Printf("Created: %s\n", session.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("Updated: %s\n", session.UpdatedAt.Format("2006-01-02 15:04:05"))

		// Get stats
		stats, err := database.GetSessionStats(session.ID)
		if err == nil {
			fmt.Printf("Messages: %d | Tools: %d | Tokens: %d\n",
				stats["message_count"],
				stats["tool_execution_count"],
				stats["total_tokens"])
		}

		fmt.Println()
	}
}

func showSession(database *storage.Database, sessionID string) {
	session, err := database.GetSession(sessionID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get session: %v\n", err)
		os.Exit(1)
	}
	if session == nil {
		fmt.Printf("Session not found: %s\n", sessionID)
		return
	}

	fmt.Printf("=== Session %s ===\n\n", sessionID)
	fmt.Printf("Title:        %s\n", session.Title)
	fmt.Printf("Status:       %s\n", session.Status)
	fmt.Printf("Project Root: %s\n", session.ProjectRoot)
	fmt.Printf("Created:      %s\n", session.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated:      %s\n", session.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()

	// Get stats
	stats, err := database.GetSessionStats(sessionID)
	if err == nil {
		fmt.Println("=== Statistics ===")
		fmt.Printf("Messages:        %d\n", stats["message_count"])
		fmt.Printf("Tool Executions: %d\n", stats["tool_execution_count"])
		fmt.Printf("Prompt Tokens:   %d\n", stats["total_prompt_tokens"])
		fmt.Printf("Response Tokens: %d\n", stats["total_response_tokens"])
		fmt.Printf("Total Tokens:    %d\n", stats["total_tokens"])
		fmt.Println()
	}

	// Get messages
	messages, err := database.GetMessages(sessionID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get messages: %v\n", err)
		return
	}

	fmt.Println("=== Conversation ===")
	for i, msg := range messages {
		fmt.Printf("\n[%d] %s:\n", i+1, msg.Role)
		fmt.Println(truncateString(msg.Content, 200))
		if msg.ToolCalls != "" {
			fmt.Printf("  (Has tool calls)\n")
		}
	}

	// Get tool executions
	executions, err := database.GetToolExecutions(sessionID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get tool executions: %v\n", err)
		return
	}

	if len(executions) > 0 {
		fmt.Println("\n=== Tool Executions ===")
		for i, exec := range executions {
			status := "✓"
			if !exec.Success {
				status = "✗"
			}
			fmt.Printf("\n[%d] %s %s (%dms)\n", i+1, status, exec.ToolName, exec.ExecutionMs)
			
			// Show file path if available
			if exec.FilePath != "" {
				fmt.Printf("    File: %s\n", exec.FilePath)
			}
			
			// Show arguments (parsed)
			if exec.Arguments != "" {
				fmt.Printf("    Arguments: %s\n", exec.Arguments)
			}
			
			// Show result (truncated)
			if exec.Success {
				resultPreview := truncateString(exec.Result, 100)
				fmt.Printf("    Result: %s\n", resultPreview)
			} else {
				fmt.Printf("    Error: %s\n", exec.ErrorMsg)
			}
		}
	}
}

func deleteSession(database *storage.Database, sessionID string) {
	session, err := database.GetSession(sessionID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get session: %v\n", err)
		os.Exit(1)
	}
	if session == nil {
		fmt.Printf("Session not found: %s\n", sessionID)
		return
	}

	if err := database.DeleteSession(sessionID); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to delete session: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Deleted session: %s\n", sessionID)
}

func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength-3] + "..."
}
