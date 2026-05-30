package ui

import (
	"fmt"
	"sync"
	"time"
)

// Spinner provides a simple terminal loading indicator
type Spinner struct {
	message    string
	stopChan   chan struct{}
	stopped    bool
	stoppedMux sync.Mutex
}

// NewSpinner creates a new spinner with the given message
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message:  message,
		stopChan: make(chan struct{}),
		stopped:  false,
	}
}

// Start begins displaying the spinner animation
func (s *Spinner) Start() {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	frameIndex := 0

	// Hide cursor
	fmt.Print("\033[?25l")

	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-s.stopChan:
				return
			case <-ticker.C:
				s.stoppedMux.Lock()
				if s.stopped {
					s.stoppedMux.Unlock()
					return
				}
				s.stoppedMux.Unlock()

				frame := frames[frameIndex]
				fmt.Printf("\r%s %s", frame, s.message)
				frameIndex = (frameIndex + 1) % len(frames)
			}
		}
	}()
}

// Stop halts the spinner and clears the line
func (s *Spinner) Stop() {
	s.stoppedMux.Lock()
	if s.stopped {
		s.stoppedMux.Unlock()
		return
	}
	s.stopped = true
	s.stoppedMux.Unlock()

	close(s.stopChan)

	// Clear the line
	fmt.Print("\r\033[K")

	// Show cursor
	fmt.Print("\033[?25h")
}
