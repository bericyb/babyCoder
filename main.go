package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/exar/babycoder/internal/app"
	"github.com/exar/babycoder/internal/logging"
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

func runInteractive() {
	state, err := app.New("Interactive session")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}
	defer state.Close()

	state.RunInteractive()
}

func runNonInteractive(prompt string) {
	state, err := app.New(truncateString(prompt, 50))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}
	defer state.Close()

	if err := state.RunNonInteractive(prompt); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength-3] + "..."
}
