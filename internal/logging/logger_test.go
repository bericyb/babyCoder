package logging

import (
	"os"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	// Create temporary directory for logs
	tempDir := t.TempDir()

	logger, err := NewLogger(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Verify logger was created
	if logger == nil {
		t.Fatal("Logger is nil")
	}

	// Verify log file exists
	if _, err := os.Stat(logger.GetLogFilePath()); os.IsNotExist(err) {
		t.Fatalf("Log file was not created: %s", logger.GetLogFilePath())
	}

	// Verify log file is in the correct directory
	if !strings.HasPrefix(logger.GetLogFilePath(), tempDir) {
		t.Fatalf("Log file not in expected directory. Expected prefix: %s, Got: %s",
			tempDir, logger.GetLogFilePath())
	}
}

func TestLoggerWritesMessages(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := NewLogger(tempDir, true)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Write various log levels
	logger.Info("Test info message")
	logger.Warn("Test warning message")
	logger.Error("Test error message")
	logger.Debug("Test debug message")

	logger.Close()

	// Read log file
	content, err := os.ReadFile(logger.GetLogFilePath())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// Verify messages are in the log
	expectedMessages := []string{
		"[INFO] Test info message",
		"[WARN] Test warning message",
		"[ERROR] Test error message",
		"[DEBUG] Test debug message",
	}

	for _, expected := range expectedMessages {
		if !strings.Contains(logContent, expected) {
			t.Errorf("Log file missing expected message: %s", expected)
		}
	}
}

func TestDebugOnlyLogsWhenVerbose(t *testing.T) {
	tempDir := t.TempDir()

	// Test with verbose = false
	logger, err := NewLogger(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	logger.Debug("This should not appear")
	logger.Info("This should appear")
	logger.Close()

	content, err := os.ReadFile(logger.GetLogFilePath())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	if strings.Contains(logContent, "This should not appear") {
		t.Error("Debug message appeared when verbose = false")
	}

	if !strings.Contains(logContent, "This should appear") {
		t.Error("Info message did not appear")
	}
}

func TestLoggerClose(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := NewLogger(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	logPath := logger.GetLogFilePath()

	// Close logger
	if err := logger.Close(); err != nil {
		t.Fatalf("Failed to close logger: %v", err)
	}

	// Verify log file still exists after close
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Log file was deleted after close")
	}
}

func TestLoggerRotation(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := NewLogger(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Write enough data to trigger rotation (>10MB)
	largeMessage := strings.Repeat("x", 1024*1024) // 1MB
	for i := 0; i < 12; i++ {
		logger.Info("Large message: %s", largeMessage)
	}

	// Check if .log.1 file exists (rotation occurred)
	rotatedPath := logger.GetLogFilePath() + ".1"
	if _, err := os.Stat(rotatedPath); os.IsNotExist(err) {
		t.Error("Log rotation did not occur - .log.1 file not found")
	}

	// Verify main log file is smaller than max size
	info, err := os.Stat(logger.GetLogFilePath())
	if err != nil {
		t.Fatalf("Failed to stat log file: %v", err)
	}

	if info.Size() >= maxLogSize {
		t.Errorf("Log file size (%d) exceeds max size (%d) after rotation", info.Size(), maxLogSize)
	}
}

