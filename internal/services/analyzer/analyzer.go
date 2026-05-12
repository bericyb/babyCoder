package analyzer

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Diagnostic represents a single issue found in code
type Diagnostic struct {
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Severity string `json:"severity"` // error, warning, info
	Message  string `json:"message"`
	Source   string `json:"source"` // go build, go vet, parser
}

// PackageInfo represents information about a Go package
type PackageInfo struct {
	PackageName string              `json:"package_name"`
	FilePath    string              `json:"file_path"`
	Functions   []FunctionSignature `json:"functions"`
	Structs     []StructDefinition  `json:"structs"`
	Interfaces  []InterfaceDefinition `json:"interfaces"`
	Imports     []string            `json:"imports"`
}

// FunctionSignature represents a function declaration
type FunctionSignature struct {
	Name       string `json:"name"`
	Receiver   string `json:"receiver,omitempty"` // For methods
	Parameters string `json:"parameters"`
	Results    string `json:"results"`
	Line       int    `json:"line"`
}

// StructDefinition represents a struct type
type StructDefinition struct {
	Name   string   `json:"name"`
	Fields []string `json:"fields"`
	Line   int      `json:"line"`
}

// InterfaceDefinition represents an interface type
type InterfaceDefinition struct {
	Name    string   `json:"name"`
	Methods []string `json:"methods"`
	Line    int      `json:"line"`
}

// Analyzer performs background code analysis
type Analyzer struct {
	projectRoot  string
	mutex        sync.RWMutex
	diagnostics  []Diagnostic
	packages     map[string]*PackageInfo // filepath -> PackageInfo
	lastAnalysis time.Time
	isAnalyzing  bool
}

// NewAnalyzer creates a new code analyzer
func NewAnalyzer(projectRoot string) *Analyzer {
	return &Analyzer{
		projectRoot: projectRoot,
		diagnostics: []Diagnostic{},
		packages:    make(map[string]*PackageInfo),
	}
}

// AnalyzeAsync triggers background analysis without blocking
func (analyzer *Analyzer) AnalyzeAsync() {
	go analyzer.Analyze()
}

// Analyze runs full project analysis (parsing, build, vet)
func (analyzer *Analyzer) Analyze() error {
	analyzer.mutex.Lock()
	if analyzer.isAnalyzing {
		analyzer.mutex.Unlock()
		return nil // Already running
	}
	analyzer.isAnalyzing = true
	analyzer.mutex.Unlock()

	defer func() {
		analyzer.mutex.Lock()
		analyzer.isAnalyzing = false
		analyzer.lastAnalysis = time.Now()
		analyzer.mutex.Unlock()
	}()

	// Clear previous results
	analyzer.mutex.Lock()
	analyzer.diagnostics = []Diagnostic{}
	analyzer.packages = make(map[string]*PackageInfo)
	analyzer.mutex.Unlock()

	// 1. Parse all Go files and build package info
	if err := analyzer.parseGoFiles(); err != nil {
		return fmt.Errorf("parse failed: %w", err)
	}

	// 2. Run go build to find compile errors
	analyzer.runGoBuild()

	// 3. Run go vet to find potential issues
	analyzer.runGoVet()

	return nil
}

// parseGoFiles walks the project and parses all .go files
func (analyzer *Analyzer) parseGoFiles() error {
	fileSet := token.NewFileSet()

	return filepath.Walk(analyzer.projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip non-Go files
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Skip vendor and hidden directories
		relativePath, _ := filepath.Rel(analyzer.projectRoot, path)
		if strings.Contains(relativePath, "vendor/") || strings.HasPrefix(relativePath, ".") {
			return nil
		}

		// Parse the file
		file, err := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
		if err != nil {
			// Add parse error as diagnostic
			analyzer.addDiagnostic(Diagnostic{
				FilePath: relativePath,
				Line:     0,
				Column:   0,
				Severity: "error",
				Message:  fmt.Sprintf("Parse error: %s", err.Error()),
				Source:   "parser",
			})
			return nil // Continue parsing other files
		}

		// Extract package information
		packageInfo := analyzer.extractPackageInfo(file, fileSet, relativePath)
		analyzer.mutex.Lock()
		analyzer.packages[relativePath] = packageInfo
		analyzer.mutex.Unlock()

		return nil
	})
}

// extractPackageInfo extracts functions, structs, interfaces from AST
func (analyzer *Analyzer) extractPackageInfo(file *ast.File, fileSet *token.FileSet, filePath string) *PackageInfo {
	packageInfo := &PackageInfo{
		PackageName: file.Name.Name,
		FilePath:    filePath,
		Functions:   []FunctionSignature{},
		Structs:     []StructDefinition{},
		Interfaces:  []InterfaceDefinition{},
		Imports:     []string{},
	}

	// Extract imports
	for _, importSpec := range file.Imports {
		importPath := strings.Trim(importSpec.Path.Value, `"`)
		packageInfo.Imports = append(packageInfo.Imports, importPath)
	}

	// Walk the AST
	ast.Inspect(file, func(node ast.Node) bool {
		switch declaration := node.(type) {
		case *ast.FuncDecl:
			// Extract function/method signature
			functionSignature := FunctionSignature{
				Name:       declaration.Name.Name,
				Parameters: analyzer.formatFieldList(declaration.Type.Params),
				Results:    analyzer.formatFieldList(declaration.Type.Results),
				Line:       fileSet.Position(declaration.Pos()).Line,
			}

			// Check if it's a method (has receiver)
			if declaration.Recv != nil && len(declaration.Recv.List) > 0 {
				functionSignature.Receiver = analyzer.formatFieldList(declaration.Recv)
			}

			packageInfo.Functions = append(packageInfo.Functions, functionSignature)

		case *ast.TypeSpec:
			// Check if it's a struct
			if structType, ok := declaration.Type.(*ast.StructType); ok {
				structDefinition := StructDefinition{
					Name:   declaration.Name.Name,
					Fields: []string{},
					Line:   fileSet.Position(declaration.Pos()).Line,
				}

				// Extract field names and types
				if structType.Fields != nil {
					for _, field := range structType.Fields.List {
						fieldType := analyzer.formatExpr(field.Type)
						if len(field.Names) > 0 {
							for _, name := range field.Names {
								structDefinition.Fields = append(structDefinition.Fields, fmt.Sprintf("%s %s", name.Name, fieldType))
							}
						} else {
							// Embedded field
							structDefinition.Fields = append(structDefinition.Fields, fieldType)
						}
					}
				}

				packageInfo.Structs = append(packageInfo.Structs, structDefinition)
			}

			// Check if it's an interface
			if interfaceType, ok := declaration.Type.(*ast.InterfaceType); ok {
				interfaceDefinition := InterfaceDefinition{
					Name:    declaration.Name.Name,
					Methods: []string{},
					Line:    fileSet.Position(declaration.Pos()).Line,
				}

				// Extract method signatures
				if interfaceType.Methods != nil {
					for _, method := range interfaceType.Methods.List {
						if len(method.Names) > 0 {
							methodName := method.Names[0].Name
							methodType := analyzer.formatExpr(method.Type)
							interfaceDefinition.Methods = append(interfaceDefinition.Methods, fmt.Sprintf("%s%s", methodName, methodType))
						}
					}
				}

				packageInfo.Interfaces = append(packageInfo.Interfaces, interfaceDefinition)
			}
		}
		return true
	})

	return packageInfo
}

// formatFieldList formats function parameters or results
func (analyzer *Analyzer) formatFieldList(fieldList *ast.FieldList) string {
	if fieldList == nil || len(fieldList.List) == 0 {
		return ""
	}

	var parts []string
	for _, field := range fieldList.List {
		fieldType := analyzer.formatExpr(field.Type)
		if len(field.Names) > 0 {
			for _, name := range field.Names {
				parts = append(parts, fmt.Sprintf("%s %s", name.Name, fieldType))
			}
		} else {
			parts = append(parts, fieldType)
		}
	}

	return strings.Join(parts, ", ")
}

// formatExpr formats an AST expression to string
func (analyzer *Analyzer) formatExpr(expr ast.Expr) string {
	switch exprType := expr.(type) {
	case *ast.Ident:
		return exprType.Name
	case *ast.StarExpr:
		return "*" + analyzer.formatExpr(exprType.X)
	case *ast.ArrayType:
		return "[]" + analyzer.formatExpr(exprType.Elt)
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", analyzer.formatExpr(exprType.Key), analyzer.formatExpr(exprType.Value))
	case *ast.SelectorExpr:
		return fmt.Sprintf("%s.%s", analyzer.formatExpr(exprType.X), exprType.Sel.Name)
	case *ast.FuncType:
		params := analyzer.formatFieldList(exprType.Params)
		results := analyzer.formatFieldList(exprType.Results)
		if results != "" {
			return fmt.Sprintf("func(%s) (%s)", params, results)
		}
		return fmt.Sprintf("func(%s)", params)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.ChanType:
		return "chan " + analyzer.formatExpr(exprType.Value)
	case *ast.Ellipsis:
		return "..." + analyzer.formatExpr(exprType.Elt)
	default:
		return "unknown"
	}
}

// runGoBuild executes go build and captures errors
func (analyzer *Analyzer) runGoBuild() {
	command := exec.Command("go", "build", "./...")
	command.Dir = analyzer.projectRoot

	var stderr bytes.Buffer
	command.Stderr = &stderr

	err := command.Run()
	if err != nil {
		// Parse build errors
		output := stderr.String()
		analyzer.parseBuildOutput(output, "go build")
	}
}

// runGoVet executes go vet and captures warnings
func (analyzer *Analyzer) runGoVet() {
	command := exec.Command("go", "vet", "./...")
	command.Dir = analyzer.projectRoot

	var stderr bytes.Buffer
	command.Stderr = &stderr

	err := command.Run()
	if err != nil {
		// Parse vet warnings
		output := stderr.String()
		analyzer.parseBuildOutput(output, "go vet")
	}
}

// parseBuildOutput parses go build/vet output into diagnostics
func (analyzer *Analyzer) parseBuildOutput(output string, source string) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Parse format: ./path/file.go:123:45: error message
		parts := strings.SplitN(line, ":", 4)
		if len(parts) >= 4 {
			filePath := strings.TrimPrefix(parts[0], "./")
			lineNum := 0
			colNum := 0
			fmt.Sscanf(parts[1], "%d", &lineNum)
			fmt.Sscanf(parts[2], "%d", &colNum)
			message := strings.TrimSpace(parts[3])

			severity := "error"
			if source == "go vet" {
				severity = "warning"
			}

			analyzer.addDiagnostic(Diagnostic{
				FilePath: filePath,
				Line:     lineNum,
				Column:   colNum,
				Severity: severity,
				Message:  message,
				Source:   source,
			})
		}
	}
}

// addDiagnostic adds a diagnostic to the list (thread-safe)
func (analyzer *Analyzer) addDiagnostic(diagnostic Diagnostic) {
	analyzer.mutex.Lock()
	defer analyzer.mutex.Unlock()
	analyzer.diagnostics = append(analyzer.diagnostics, diagnostic)
}

// GetDiagnostics returns all diagnostics (thread-safe)
func (analyzer *Analyzer) GetDiagnostics() []Diagnostic {
	analyzer.mutex.RLock()
	defer analyzer.mutex.RUnlock()
	
	// Return a copy to avoid race conditions
	diagnosticsCopy := make([]Diagnostic, len(analyzer.diagnostics))
	copy(diagnosticsCopy, analyzer.diagnostics)
	return diagnosticsCopy
}

// GetFileDiagnostics returns diagnostics for a specific file
func (analyzer *Analyzer) GetFileDiagnostics(filePath string) []Diagnostic {
	analyzer.mutex.RLock()
	defer analyzer.mutex.RUnlock()

	var fileDiagnostics []Diagnostic
	for _, diagnostic := range analyzer.diagnostics {
		if diagnostic.FilePath == filePath {
			fileDiagnostics = append(fileDiagnostics, diagnostic)
		}
	}
	return fileDiagnostics
}

// GetPackageInfo returns package info for a specific file
func (analyzer *Analyzer) GetPackageInfo(filePath string) *PackageInfo {
	analyzer.mutex.RLock()
	defer analyzer.mutex.RUnlock()

	return analyzer.packages[filePath]
}

// GetAllPackages returns all parsed package information
func (analyzer *Analyzer) GetAllPackages() map[string]*PackageInfo {
	analyzer.mutex.RLock()
	defer analyzer.mutex.RUnlock()

	// Return a copy
	packagesCopy := make(map[string]*PackageInfo)
	for key, value := range analyzer.packages {
		packagesCopy[key] = value
	}
	return packagesCopy
}

// GetStatus returns analysis status and summary
func (analyzer *Analyzer) GetStatus() map[string]interface{} {
	analyzer.mutex.RLock()
	defer analyzer.mutex.RUnlock()

	errorCount := 0
	warningCount := 0
	for _, diagnostic := range analyzer.diagnostics {
		if diagnostic.Severity == "error" {
			errorCount++
		} else if diagnostic.Severity == "warning" {
			warningCount++
		}
	}

	return map[string]interface{}{
		"is_analyzing":  analyzer.isAnalyzing,
		"last_analysis": analyzer.lastAnalysis,
		"error_count":   errorCount,
		"warning_count": warningCount,
		"file_count":    len(analyzer.packages),
	}
}
