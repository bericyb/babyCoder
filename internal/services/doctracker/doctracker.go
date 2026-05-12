package doctracker

import (
	"bytes"
	gocontext "context"
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/exar/babycoder/internal/services/ai_provider"
	"github.com/exar/babycoder/internal/storage"
)

// DocTracker manages documentation tracking and auto-updates
type DocTracker struct {
	projectRoot string
	database    *storage.Database
	aiProvider  ai_provider.Provider
	updateQueue chan *storage.DocumentationUpdate
	workerCount int
	mutex       sync.RWMutex
	isRunning   bool
	stopChan    chan struct{}
}

// NewDocTracker creates a new documentation tracker
func NewDocTracker(projectRoot string, database *storage.Database, aiProvider ai_provider.Provider, workerCount int) *DocTracker {
	return &DocTracker{
		projectRoot: projectRoot,
		database:    database,
		aiProvider:  aiProvider,
		updateQueue: make(chan *storage.DocumentationUpdate, 50),
		workerCount: workerCount,
		stopChan:    make(chan struct{}),
	}
}

// Start begins the background workers
func (tracker *DocTracker) Start() {
	tracker.mutex.Lock()
	if tracker.isRunning {
		tracker.mutex.Unlock()
		return
	}
	tracker.isRunning = true
	tracker.mutex.Unlock()

	// Start worker goroutines
	for i := 0; i < tracker.workerCount; i++ {
		go tracker.worker()
	}

	log.Printf("DocTracker started with %d workers\n", tracker.workerCount)
}

// Stop gracefully shuts down the workers
func (tracker *DocTracker) Stop() {
	tracker.mutex.Lock()
	if !tracker.isRunning {
		tracker.mutex.Unlock()
		return
	}
	tracker.isRunning = false
	tracker.mutex.Unlock()

	close(tracker.stopChan)
	log.Println("DocTracker stopped")
}

// CheckFileAsync triggers async check of a Go file
func (tracker *DocTracker) CheckFileAsync(filePath string) {
	go tracker.CheckFile(filePath)
}

// CheckFile analyzes a file and queues updates if needed
func (tracker *DocTracker) CheckFile(filePath string) error {
	// Make path relative to project root
	relativePath, err := filepath.Rel(tracker.projectRoot, filePath)
	if err != nil {
		relativePath = filePath
	}

	// Parse the file
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, filePath, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	// Get existing hashes from database
	existingHashes, err := tracker.database.GetDocumentationHashes(relativePath)
	if err != nil {
		return fmt.Errorf("failed to get existing hashes: %w", err)
	}

	existingMap := make(map[string]*storage.DocumentationHash)
	for _, hash := range existingHashes {
		existingMap[hash.SymbolName] = hash
	}

	// Walk AST and check exported symbols
	ast.Inspect(file, func(node ast.Node) bool {
		switch decl := node.(type) {
		case *ast.FuncDecl:
			// Only check exported functions
			if decl.Name.IsExported() {
				tracker.checkFunction(fileSet, file, decl, relativePath, existingMap)
			}

		case *ast.GenDecl:
			// Check type declarations (structs, interfaces)
			for _, spec := range decl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if typeSpec.Name.IsExported() {
						tracker.checkType(fileSet, file, typeSpec, decl, relativePath, existingMap)
					}
				}
			}
		}
		return true
	})

	return nil
}

// checkFunction checks if a function's docs are stale
func (tracker *DocTracker) checkFunction(fileSet *token.FileSet, file *ast.File, funcDecl *ast.FuncDecl, filePath string, existingMap map[string]*storage.DocumentationHash) {
	symbolName := funcDecl.Name.Name
	
	// Get signature
	signature := tracker.extractFunctionSignature(fileSet, funcDecl)
	signatureHash := tracker.hashString(signature)

	// Get doc comment
	docComment := ""
	if funcDecl.Doc != nil {
		docComment = funcDecl.Doc.Text()
	}
	docHash := tracker.hashString(docComment)

	// Check against existing
	existing, exists := existingMap[symbolName]
	isStale := false
	
	if exists {
		// Signature changed but doc didn't?
		if existing.SignatureHash != signatureHash && existing.DocHash == docHash {
			isStale = true
			// Queue update job
			tracker.queueUpdate(filePath, symbolName, existing.SignatureHash, signature, docComment, fileSet, funcDecl)
		}
	}

	// Save/update hash
	docHashRecord := &storage.DocumentationHash{
		FilePath:      filePath,
		SymbolName:    symbolName,
		SymbolType:    "func",
		SignatureHash: signatureHash,
		DocHash:       docHash,
		IsStale:       isStale,
		LastChecked:   time.Now(),
	}

	tracker.database.SaveDocumentationHash(docHashRecord)
}

// checkType checks if a type's (struct/interface) docs are stale
func (tracker *DocTracker) checkType(fileSet *token.FileSet, file *ast.File, typeSpec *ast.TypeSpec, genDecl *ast.GenDecl, filePath string, existingMap map[string]*storage.DocumentationHash) {
	symbolName := typeSpec.Name.Name
	symbolType := "type"

	// Determine specific type
	switch typeSpec.Type.(type) {
	case *ast.StructType:
		symbolType = "struct"
	case *ast.InterfaceType:
		symbolType = "interface"
	}

	// Get signature
	signature := tracker.extractTypeSignature(fileSet, typeSpec)
	signatureHash := tracker.hashString(signature)

	// Get doc comment
	docComment := ""
	if genDecl.Doc != nil {
		docComment = genDecl.Doc.Text()
	}
	docHash := tracker.hashString(docComment)

	// Check against existing
	existing, exists := existingMap[symbolName]
	isStale := false

	if exists {
		if existing.SignatureHash != signatureHash && existing.DocHash == docHash {
			isStale = true
			tracker.queueUpdate(filePath, symbolName, existing.SignatureHash, signature, docComment, fileSet, typeSpec)
		}
	}

	// Save/update hash
	docHashRecord := &storage.DocumentationHash{
		FilePath:      filePath,
		SymbolName:    symbolName,
		SymbolType:    symbolType,
		SignatureHash: signatureHash,
		DocHash:       docHash,
		IsStale:       isStale,
		LastChecked:   time.Now(),
	}

	tracker.database.SaveDocumentationHash(docHashRecord)
}

// extractFunctionSignature gets normalized function signature
func (tracker *DocTracker) extractFunctionSignature(fileSet *token.FileSet, funcDecl *ast.FuncDecl) string {
	var buf bytes.Buffer
	
	// Write function signature (receiver, name, params, results)
	if funcDecl.Recv != nil {
		buf.WriteString("func (")
		printer.Fprint(&buf, fileSet, funcDecl.Recv)
		buf.WriteString(") ")
	} else {
		buf.WriteString("func ")
	}
	
	buf.WriteString(funcDecl.Name.Name)
	printer.Fprint(&buf, fileSet, funcDecl.Type.Params)
	
	if funcDecl.Type.Results != nil {
		buf.WriteString(" ")
		printer.Fprint(&buf, fileSet, funcDecl.Type.Results)
	}

	return buf.String()
}

// extractTypeSignature gets normalized type signature
func (tracker *DocTracker) extractTypeSignature(fileSet *token.FileSet, typeSpec *ast.TypeSpec) string {
	var buf bytes.Buffer
	buf.WriteString("type ")
	buf.WriteString(typeSpec.Name.Name)
	buf.WriteString(" ")
	printer.Fprint(&buf, fileSet, typeSpec.Type)
	return buf.String()
}

// hashString creates SHA256 hash of string
func (tracker *DocTracker) hashString(s string) string {
	hash := sha256.Sum256([]byte(strings.TrimSpace(s)))
	return fmt.Sprintf("%x", hash)
}

// queueUpdate creates and queues a documentation update job
func (tracker *DocTracker) queueUpdate(filePath, symbolName, oldSig, newSig, oldDoc string, fileSet *token.FileSet, node ast.Node) {
	// Get surrounding context
	context := tracker.extractContext(fileSet, node)

	update := &storage.DocumentationUpdate{
		FilePath:     filePath,
		SymbolName:   symbolName,
		OldSignature: oldSig,
		NewSignature: newSig,
		OldDoc:       oldDoc,
		Status:       "pending",
		CreatedAt:    time.Now(),
	}

	// Save to database
	if err := tracker.database.CreateDocumentationUpdate(update); err != nil {
		log.Printf("Failed to create doc update record: %v\n", err)
		return
	}

	// Add context temporarily for LLM (not saved to DB)
	// We'll pass it through the channel metadata
	updateWithContext := *update
	updateWithContext.NewDoc = context // Temporary storage

	// Queue for processing
	select {
	case tracker.updateQueue <- &updateWithContext:
		log.Printf("Queued doc update for %s::%s\n", filePath, symbolName)
	default:
		log.Printf("Update queue full, skipping %s::%s\n", filePath, symbolName)
	}
}

// extractContext gets surrounding code for LLM context
func (tracker *DocTracker) extractContext(fileSet *token.FileSet, node ast.Node) string {
	var buf bytes.Buffer
	printer.Fprint(&buf, fileSet, node)
	return buf.String()
}

// worker processes documentation update jobs
func (tracker *DocTracker) worker() {
	for {
		select {
		case <-tracker.stopChan:
			return
			
		case update := <-tracker.updateQueue:
			tracker.processUpdate(update)
		}
	}
}

// processUpdate uses LLM to generate new documentation
func (tracker *DocTracker) processUpdate(update *storage.DocumentationUpdate) {
	// Update status to processing
	update.Status = "processing"
	tracker.database.UpdateDocumentationUpdate(update)

	// Extract context from temporary storage
	context := update.NewDoc
	update.NewDoc = "" // Clear it

	// Build prompt for LLM
	prompt := tracker.buildDocPrompt(update, context)

	// Call LLM
	ctx, cancel := gocontext.WithTimeout(gocontext.Background(), 30*time.Second)
	defer cancel()

	request := ai_provider.ChatCompletionRequest{
		Messages: []ai_provider.Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	response, err := tracker.aiProvider.ChatCompletion(ctx, request)
	if err != nil {
		log.Printf("LLM call failed for %s::%s: %v\n", update.FilePath, update.SymbolName, err)
		update.Status = "failed"
		update.ErrorMsg = err.Error()
		update.CompletedAt = time.Now()
		tracker.database.UpdateDocumentationUpdate(update)
		return
	}

	if len(response.Choices) == 0 {
		log.Printf("No response from LLM for %s::%s\n", update.FilePath, update.SymbolName)
		update.Status = "failed"
		update.ErrorMsg = "empty response from LLM"
		update.CompletedAt = time.Now()
		tracker.database.UpdateDocumentationUpdate(update)
		return
	}

	newDoc := strings.TrimSpace(response.Choices[0].Message.Content)

	// Apply the documentation update
	if err := tracker.applyDocUpdate(update.FilePath, update.SymbolName, newDoc); err != nil {
		log.Printf("Failed to apply doc update for %s::%s: %v\n", update.FilePath, update.SymbolName, err)
		update.Status = "failed"
		update.ErrorMsg = err.Error()
		update.CompletedAt = time.Now()
		tracker.database.UpdateDocumentationUpdate(update)
		return
	}

	// Success!
	update.NewDoc = newDoc
	update.Status = "completed"
	update.CompletedAt = time.Now()
	tracker.database.UpdateDocumentationUpdate(update)

	log.Printf("✓ Updated docs for %s::%s\n", update.FilePath, update.SymbolName)

	// Re-check the file to update hashes
	fullPath := filepath.Join(tracker.projectRoot, update.FilePath)
	tracker.CheckFile(fullPath)
}

// buildDocPrompt creates the LLM prompt for documentation generation
func (tracker *DocTracker) buildDocPrompt(update *storage.DocumentationUpdate, context string) string {
	prompt := fmt.Sprintf(`You are a Go documentation expert. Update the documentation comment for this code.

OLD SIGNATURE:
%s

NEW SIGNATURE:
%s

OLD DOCUMENTATION:
%s

CODE CONTEXT:
%s

Generate ONLY the new Go documentation comment (using // format). Follow these rules:
1. Be concise and clear
2. Start with the symbol name
3. Describe what it does, not how
4. Mention important parameters or return values
5. Follow Go documentation conventions
6. Do NOT include any explanation, just the comment itself
7. Use // for each line (not /* */)

NEW DOCUMENTATION:`, update.OldSignature, update.NewSignature, update.OldDoc, context)

	return prompt
}

// applyDocUpdate writes the new documentation to the file
func (tracker *DocTracker) applyDocUpdate(relativePath, symbolName, newDoc string) error {
	fullPath := filepath.Join(tracker.projectRoot, relativePath)

	// Parse the file
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, fullPath, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	// Find the symbol and update its doc
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		switch decl := node.(type) {
		case *ast.FuncDecl:
			if decl.Name.Name == symbolName {
				decl.Doc = tracker.createCommentGroup(newDoc, fileSet.Position(decl.Pos()).Line)
				found = true
				return false
			}

		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if typeSpec.Name.Name == symbolName {
						decl.Doc = tracker.createCommentGroup(newDoc, fileSet.Position(decl.Pos()).Line)
						found = true
						return false
					}
				}
			}
		}
		return true
	})

	if !found {
		return fmt.Errorf("symbol %s not found in file", symbolName)
	}

	// Write back to file
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fileSet, file); err != nil {
		return fmt.Errorf("failed to format file: %w", err)
	}

	if err := os.WriteFile(fullPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// createCommentGroup creates an AST comment group from doc string
func (tracker *DocTracker) createCommentGroup(doc string, lineNum int) *ast.CommentGroup {
	lines := strings.Split(strings.TrimSpace(doc), "\n")
	comments := make([]*ast.Comment, 0, len(lines))

	for i, line := range lines {
		text := strings.TrimSpace(line)
		// Ensure it starts with //
		if !strings.HasPrefix(text, "//") {
			text = "// " + text
		}
		comments = append(comments, &ast.Comment{
			Text: text,
			Slash: token.Pos(lineNum + i), // Approximate position
		})
	}

	return &ast.CommentGroup{
		List: comments,
	}
}

// GetStaleCount returns count of stale documentation
func (tracker *DocTracker) GetStaleCount() (int, error) {
	stale, err := tracker.database.GetStaleDocumentationHashes()
	if err != nil {
		return 0, err
	}
	return len(stale), nil
}

// GetPendingUpdateCount returns count of pending updates
func (tracker *DocTracker) GetPendingUpdateCount() (int, error) {
	pending, err := tracker.database.GetPendingDocumentationUpdates()
	if err != nil {
		return 0, err
	}
	return len(pending), nil
}
