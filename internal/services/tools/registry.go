package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/exar/babycoder/internal/services/ai_provider"
	"github.com/exar/babycoder/internal/services/analyzer"
	"github.com/exar/babycoder/internal/services/doctracker"
	"github.com/exar/babycoder/internal/services/testrunner"
	"github.com/exar/babycoder/internal/storage"
)

// ToolRegistry manages all available tools
type ToolRegistry struct {
	projectRoot string
	sessionID   string
	tools       map[string]Tool
	analyzer    *analyzer.Analyzer
	testRunner  *testrunner.TestRunner
	docTracker  *doctracker.DocTracker
	database    *storage.Database
}

// Tool represents a function that can be called by the agent
type Tool interface {
	Execute(arguments map[string]interface{}) (string, error)
	GetDefinition() ai_provider.Tool
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry(projectRoot, sessionID string, codeAnalyzer *analyzer.Analyzer, testRunner *testrunner.TestRunner, docTracker *doctracker.DocTracker, database *storage.Database) *ToolRegistry {
	registry := &ToolRegistry{
		projectRoot: projectRoot,
		sessionID:   sessionID,
		tools:       make(map[string]Tool),
		analyzer:    codeAnalyzer,
		testRunner:  testRunner,
		docTracker:  docTracker,
		database:    database,
	}

	// Register file operation tools
	registry.registerTool(&ReadFileTool{projectRoot: projectRoot, analyzer: codeAnalyzer})
	registry.registerTool(&WriteFileTool{projectRoot: projectRoot, analyzer: codeAnalyzer, testRunner: testRunner, docTracker: docTracker})
	registry.registerTool(&ListFilesTool{projectRoot: projectRoot})
	registry.registerTool(&LineEditFileTool{projectRoot: projectRoot, analyzer: codeAnalyzer, testRunner: testRunner, docTracker: docTracker})
	registry.registerTool(&FindAndReplaceEditFileTool{projectRoot: projectRoot, analyzer: codeAnalyzer, testRunner: testRunner, docTracker: docTracker})

	// Register code analysis tools
	registry.registerTool(&CheckCodeStatusTool{analyzer: codeAnalyzer})
	registry.registerTool(&GetFileDiagnosticsTool{analyzer: codeAnalyzer})
	registry.registerTool(&GetPackageOutlineTool{analyzer: codeAnalyzer})
	registry.registerTool(&GetProjectStructureTool{ProjectRoot: projectRoot})

	// Register test tools
	registry.registerTool(&GetTestStatusTool{testRunner: testRunner})
	registry.registerTool(&GetFailingTestsTool{testRunner: testRunner})
	registry.registerTool(&RunTestsTool{testRunner: testRunner})

	// Register bash tool
	registry.registerTool(&BashExecuteTool{projectRoot: projectRoot})

	return registry
}

// registerTool adds a tool to the registry
func (registry *ToolRegistry) registerTool(tool Tool) {
	definition := tool.GetDefinition()
	registry.tools[definition.Function.Name] = tool
}

// RegisterTool adds a tool to the registry (public method for dynamic registration)
func (registry *ToolRegistry) RegisterTool(tool Tool) {
	registry.registerTool(tool)
}

// GetTool retrieves a tool by name
func (registry *ToolRegistry) GetTool(name string) (Tool, bool) {
	tool, exists := registry.tools[name]
	return tool, exists
}

// GetAllDefinitions returns all tool definitions for the AI provider
func (registry *ToolRegistry) GetAllDefinitions() []ai_provider.Tool {
	definitions := make([]ai_provider.Tool, 0, len(registry.tools))
	for _, tool := range registry.tools {
		definitions = append(definitions, tool.GetDefinition())
	}
	return definitions
}

// Execute executes a tool by name with the given arguments
func (registry *ToolRegistry) Execute(toolName string, arguments map[string]interface{}) (string, error) {
	tool, exists := registry.GetTool(toolName)
	if !exists {
		return "", fmt.Errorf("tool not found: %s", toolName)
	}

	return tool.Execute(arguments)
}

// Helper function to resolve file paths relative to project root
func (registry *ToolRegistry) resolvePath(filePath string) (string, error) {
	// If absolute path, verify it's within project root
	if filepath.IsAbs(filePath) {
		if !strings.HasPrefix(filePath, registry.projectRoot) {
			return "", fmt.Errorf("access denied: path outside project root")
		}
		return filePath, nil
	}

	// Relative path - join with project root
	resolved := filepath.Join(registry.projectRoot, filePath)

	// Verify it's still within project root (prevent ../ escapes)
	absResolved, err := filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	absProjectRoot, err := filepath.Abs(registry.projectRoot)
	if err != nil {
		return "", fmt.Errorf("failed to resolve project root: %w", err)
	}

	if !strings.HasPrefix(absResolved, absProjectRoot) {
		return "", fmt.Errorf("access denied: path outside project root")
	}

	return absResolved, nil
}

// Helper function to get string argument
func getStringArg(arguments map[string]interface{}, key string) (string, error) {
	value, exists := arguments[key]
	if !exists {
		return "", fmt.Errorf("missing required argument: %s", key)
	}

	strValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("argument %s must be a string", key)
	}

	return strValue, nil
}

// Helper function to get int argument
func getIntArg(arguments map[string]interface{}, key string) (int, error) {
	value, exists := arguments[key]
	if !exists {
		return 0, fmt.Errorf("missing required argument: %s", key)
	}

	// Handle both float64 (from JSON) and int
	switch v := value.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	default:
		return 0, fmt.Errorf("argument %s must be a number", key)
	}
}

// Helper function to get optional bool argument
func getBoolArg(arguments map[string]interface{}, key string, defaultValue bool) bool {
	value, exists := arguments[key]
	if !exists {
		return defaultValue
	}

	boolValue, ok := value.(bool)
	if !ok {
		return defaultValue
	}

	return boolValue
}

// Helper function to check if file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Helper function to get optional string argument with default
func getStringArgDefault(arguments map[string]interface{}, key string, defaultValue string) string {
	value, exists := arguments[key]
	if !exists {
		return defaultValue
	}

	strValue, ok := value.(string)
	if !ok {
		return defaultValue
	}

	return strValue
}

// Helper function to get optional float64 argument with default
func getFloat64ArgDefault(arguments map[string]interface{}, key string, defaultValue float64) float64 {
	value, exists := arguments[key]
	if !exists {
		return defaultValue
	}

	floatValue, ok := value.(float64)
	if !ok {
		return defaultValue
	}

	return floatValue
}

// Helper function to get optional int argument with default
func getIntArgDefault(arguments map[string]interface{}, key string, defaultValue int) int {
	value, exists := arguments[key]
	if !exists {
		return defaultValue
	}

	// Handle both float64 (from JSON) and int
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return defaultValue
	}
}

// Helper function to get string array argument
func getStringArrayArg(arguments map[string]interface{}, key string) ([]string, error) {
	value, exists := arguments[key]
	if !exists {
		return nil, fmt.Errorf("missing required argument: %s", key)
	}

	arrayValue, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf("argument %s must be an array", key)
	}

	result := make([]string, len(arrayValue))
	for i, item := range arrayValue {
		strItem, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("argument %s must be an array of strings", key)
		}
		result[i] = strItem
	}

	return result, nil
}

// Helper function to get optional string array argument with default
func getStringArrayArgDefault(arguments map[string]interface{}, key string, defaultValue []string) []string {
	value, exists := arguments[key]
	if !exists {
		return defaultValue
	}

	arrayValue, ok := value.([]interface{})
	if !ok {
		return defaultValue
	}

	result := make([]string, len(arrayValue))
	for i, item := range arrayValue {
		strItem, ok := item.(string)
		if !ok {
			return defaultValue
		}
		result[i] = strItem
	}

	return result
}

