package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

const (
	maxLogSize = 10 * 1024 * 1024 // 10MB
	maxLogFiles = 2                 // Keep 2 files (.log and .log.1)
)

// Logger wraps the standard logger with simple rolling file support
type Logger struct {
	logger   *log.Logger
	LogFile  *os.File  // Exported for redirecting standard log
	filePath string
	verbose  bool
}

// NewLogger creates a new logger with rolling file support
func NewLogger(logDirectory string, verbose bool) (*Logger, error) {
	// Create log directory if it doesn't exist
	if err := os.MkdirAll(logDirectory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	logFilePath := filepath.Join(logDirectory, "babycoder.log")

	// Open or create log file
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Create logger
	logger := log.New(logFile, "", 0) // No prefix, we'll add our own

	l := &Logger{
		logger:   logger,
		LogFile:  logFile,
		filePath: logFilePath,
		verbose:  verbose,
	}

	// Check if rotation is needed
	l.rotateIfNeeded()

	return l, nil
}

// rotateIfNeeded checks file size and rotates if necessary
func (l *Logger) rotateIfNeeded() error {
	info, err := l.LogFile.Stat()
	if err != nil {
		return err
	}

	if info.Size() >= maxLogSize {
		return l.rotate()
	}

	return nil
}

// rotate performs log rotation
func (l *Logger) rotate() error {
	// Close current file
	if err := l.LogFile.Close(); err != nil {
		return err
	}

	// Rotate old logs
	oldPath := l.filePath + ".1"
	if _, err := os.Stat(oldPath); err == nil {
		os.Remove(oldPath) // Remove oldest log
	}

	// Rename current log to .1
	if err := os.Rename(l.filePath, oldPath); err != nil {
		return err
	}

	// Create new log file
	logFile, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	l.LogFile = logFile
	l.logger.SetOutput(logFile)

	return nil
}

// Close closes the log file
func (l *Logger) Close() error {
	if l.LogFile != nil {
		return l.LogFile.Close()
	}
	return nil
}

// GetLogFilePath returns the path to the log file
func (l *Logger) GetLogFilePath() string {
	return l.filePath
}

// SetVerbose sets the verbose flag
func (l *Logger) SetVerbose(verbose bool) {
	l.verbose = verbose
}

// log writes a log message with level and timestamp
func (l *Logger) log(level string, format string, v ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, v...)
	l.logger.Printf("[%s] [%s] %s\n", timestamp, level, message)
	
	// Check for rotation after each write
	l.rotateIfNeeded()
}

// Debug logs a debug message (only if verbose is enabled)
func (l *Logger) Debug(format string, v ...interface{}) {
	if l.verbose {
		l.log("DEBUG", format, v...)
	}
}

// Info logs an informational message
func (l *Logger) Info(format string, v ...interface{}) {
	l.log("INFO", format, v...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, v ...interface{}) {
	l.log("WARN", format, v...)
}

// Error logs an error message
func (l *Logger) Error(format string, v ...interface{}) {
	l.log("ERROR", format, v...)
}

// Fatal logs a fatal error message and exits
func (l *Logger) Fatal(format string, v ...interface{}) {
	l.log("FATAL", format, v...)
	os.Exit(1)
}

// Printf logs a formatted message (compatible with log.Logger)
func (l *Logger) Printf(format string, v ...interface{}) {
	l.log("INFO", format, v...)
}
