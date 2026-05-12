package tools

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/exar/babycoder/internal/services/ai_provider"
)

// GetProjectStructureTool analyzes the entire Go project structure
type GetProjectStructureTool struct {
	ProjectRoot string // Made public for testing, normally set via registry
}

// PackageStructure represents a Go package's structure
type PackageStructure struct {
	Path         string
	Name         string
	Files        []string
	Imports      []string
	Exports      *PackageExports
	FileCount    int
	LineCount    int
}

// PackageExports holds exported symbols from a package
type PackageExports struct {
	Functions  []string
	Types      []string
	Constants  []string
	Variables  []string
}

// Execute analyzes the project structure
func (tool *GetProjectStructureTool) Execute(arguments map[string]interface{}) (string, error) {
	includeImports := getBoolArg(arguments, "include_imports", false)
	includeExports := getBoolArg(arguments, "include_exports", true)
	maxDepth := getIntArgDefault(arguments, "max_depth", 10)

	// Discover all Go packages
	packages, err := tool.discoverPackages(maxDepth)
	if err != nil {
		return "", fmt.Errorf("failed to discover packages: %w", err)
	}

	if len(packages) == 0 {
		return "No Go packages found in project.", nil
	}

	// Build output
	var output strings.Builder
	
	output.WriteString(fmt.Sprintf("# Go Project Structure\n\n"))
	output.WriteString(fmt.Sprintf("**Project Root:** %s\n", tool.ProjectRoot))
	output.WriteString(fmt.Sprintf("**Total Packages:** %d\n\n", len(packages)))

	// Organize packages by directory depth
	organized := tool.organizePackages(packages)

	output.WriteString("## Package Hierarchy\n\n")
	for _, pkg := range organized {
		depth := strings.Count(pkg.Path, string(filepath.Separator))
		indent := strings.Repeat("  ", depth)
		
		output.WriteString(fmt.Sprintf("%s📦 **%s** (`%s`)\n", indent, pkg.Name, pkg.Path))
		output.WriteString(fmt.Sprintf("%s   Files: %d | Lines: ~%d\n", indent, pkg.FileCount, pkg.LineCount))

		if includeExports && pkg.Exports != nil {
			if len(pkg.Exports.Types) > 0 {
				output.WriteString(fmt.Sprintf("%s   Types: %s\n", indent, 
					tool.formatList(pkg.Exports.Types, 5)))
			}
			if len(pkg.Exports.Functions) > 0 {
				output.WriteString(fmt.Sprintf("%s   Functions: %s\n", indent, 
					tool.formatList(pkg.Exports.Functions, 5)))
			}
		}

		if includeImports && len(pkg.Imports) > 0 {
			output.WriteString(fmt.Sprintf("%s   Imports: %s\n", indent, 
				tool.formatList(pkg.Imports, 3)))
		}

		output.WriteString("\n")
	}

	// Summary statistics
	output.WriteString("## Summary\n\n")
	totalFiles := 0
	totalLines := 0
	totalTypes := 0
	totalFunctions := 0

	for _, pkg := range packages {
		totalFiles += pkg.FileCount
		totalLines += pkg.LineCount
		if pkg.Exports != nil {
			totalTypes += len(pkg.Exports.Types)
			totalFunctions += len(pkg.Exports.Functions)
		}
	}

	output.WriteString(fmt.Sprintf("- **Total Go Files:** %d\n", totalFiles))
	output.WriteString(fmt.Sprintf("- **Total Lines:** ~%d\n", totalLines))
	output.WriteString(fmt.Sprintf("- **Exported Types:** %d\n", totalTypes))
	output.WriteString(fmt.Sprintf("- **Exported Functions:** %d\n", totalFunctions))

	return output.String(), nil
}

// discoverPackages finds all Go packages in the project
func (tool *GetProjectStructureTool) discoverPackages(maxDepth int) ([]*PackageStructure, error) {
	packages := make(map[string]*PackageStructure)

	err := filepath.Walk(tool.ProjectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip vendor, .git, node_modules
		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || name == ".git" || name == "node_modules" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
		}

		// Only process .go files (not test files for cleaner output)
		if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			dir := filepath.Dir(path)
			relDir, _ := filepath.Rel(tool.ProjectRoot, dir)
			
			// Check depth
			depth := strings.Count(relDir, string(filepath.Separator))
			if depth > maxDepth {
				return nil
			}

			// Get or create package structure
			if _, exists := packages[relDir]; !exists {
				packages[relDir] = &PackageStructure{
					Path:    relDir,
					Files:   []string{},
					Imports: []string{},
					Exports: &PackageExports{
						Functions: []string{},
						Types:     []string{},
						Constants: []string{},
						Variables: []string{},
					},
				}
			}

			pkg := packages[relDir]
			pkg.Files = append(pkg.Files, filepath.Base(path))
			pkg.FileCount++

			// Parse file for package info
			tool.analyzeFile(path, pkg)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Convert map to slice
	result := make([]*PackageStructure, 0, len(packages))
	for _, pkg := range packages {
		result = append(result, pkg)
	}

	return result, nil
}

// analyzeFile extracts package info from a Go file
func (tool *GetProjectStructureTool) analyzeFile(filePath string, pkg *PackageStructure) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return
	}

	// Get package name (from first file)
	if pkg.Name == "" {
		pkg.Name = node.Name.Name
	}

	// Count lines
	content, _ := os.ReadFile(filePath)
	pkg.LineCount += strings.Count(string(content), "\n")

	// Extract imports
	importMap := make(map[string]bool)
	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		// Only include non-stdlib imports for brevity
		if strings.Contains(importPath, ".") {
			importMap[importPath] = true
		}
	}
	for imp := range importMap {
		if !contains(pkg.Imports, imp) {
			pkg.Imports = append(pkg.Imports, imp)
		}
	}

	// Extract exported symbols
	ast.Inspect(node, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.FuncDecl:
			if decl.Name.IsExported() {
				funcName := decl.Name.Name
				if decl.Recv != nil {
					// Method
					recvType := tool.getReceiverType(decl.Recv)
					funcName = fmt.Sprintf("(%s) %s", recvType, funcName)
				}
				if !contains(pkg.Exports.Functions, funcName) {
					pkg.Exports.Functions = append(pkg.Exports.Functions, funcName)
				}
			}

		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name.IsExported() {
						typeName := s.Name.Name
						// Add type indicator
						switch s.Type.(type) {
						case *ast.StructType:
							typeName += " (struct)"
						case *ast.InterfaceType:
							typeName += " (interface)"
						}
						if !contains(pkg.Exports.Types, typeName) {
							pkg.Exports.Types = append(pkg.Exports.Types, typeName)
						}
					}

				case *ast.ValueSpec:
					for _, name := range s.Names {
						if name.IsExported() {
							if decl.Tok == token.CONST {
								if !contains(pkg.Exports.Constants, name.Name) {
									pkg.Exports.Constants = append(pkg.Exports.Constants, name.Name)
								}
							} else if decl.Tok == token.VAR {
								if !contains(pkg.Exports.Variables, name.Name) {
									pkg.Exports.Variables = append(pkg.Exports.Variables, name.Name)
								}
							}
						}
					}
				}
			}
		}
		return true
	})
}

// getReceiverType extracts the receiver type from a method
func (tool *GetProjectStructureTool) getReceiverType(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}

	field := recv.List[0]
	switch t := field.Type.(type) {
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return "*" + ident.Name
		}
	case *ast.Ident:
		return t.Name
	}
	return ""
}

// organizePackages sorts packages for hierarchical display
func (tool *GetProjectStructureTool) organizePackages(packages []*PackageStructure) []*PackageStructure {
	sort.Slice(packages, func(i, j int) bool {
		// Sort by path depth first, then alphabetically
		depthI := strings.Count(packages[i].Path, string(filepath.Separator))
		depthJ := strings.Count(packages[j].Path, string(filepath.Separator))
		if depthI != depthJ {
			return depthI < depthJ
		}
		return packages[i].Path < packages[j].Path
	})
	return packages
}

// formatList formats a slice into a readable string
func (tool *GetProjectStructureTool) formatList(items []string, max int) string {
	if len(items) == 0 {
		return "none"
	}

	sort.Strings(items)

	if len(items) <= max {
		return strings.Join(items, ", ")
	}

	return fmt.Sprintf("%s, ... (+%d more)", 
		strings.Join(items[:max], ", "), 
		len(items)-max)
}

// contains checks if a string slice contains a value
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetDefinition returns the tool definition
func (tool *GetProjectStructureTool) GetDefinition() ai_provider.Tool {
	return ai_provider.Tool{
		Type: "function",
		Function: ai_provider.ToolFunction{
			Name:        "get_project_structure",
			Description: "Analyze and display the entire Go project structure, showing all packages, their exported types/functions, and organization. Perfect for understanding the codebase architecture at a glance.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"include_imports": map[string]interface{}{
						"type":        "boolean",
						"description": "Include imported packages for each package (default: false, can be verbose)",
					},
					"include_exports": map[string]interface{}{
						"type":        "boolean",
						"description": "Include exported types and functions for each package (default: true)",
					},
					"max_depth": map[string]interface{}{
						"type":        "number",
						"description": "Maximum directory depth to analyze (default: 10)",
					},
				},
			},
		},
	}
}
