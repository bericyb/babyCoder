package tools

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// writeTempFile creates a file in t.TempDir() with the given contents and
// returns its absolute path. It fails the test on any I/O error.
func writeTempFile(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatalf("failed to write temp file %s: %v", path, err)
	}
	return path
}

// TestFileHashTrackerRecordAndVerifyOK verifies the happy path: a recorded
// read can be subsequently verified against unchanged content.
func TestFileHashTrackerRecordAndVerifyOK(t *testing.T) {
	path := writeTempFile(t, "foo.txt", "hello world")

	tracker := NewFileHashTracker()
	if _, err := tracker.RecordReadFromDisk(path); err != nil {
		t.Fatalf("record failed: %v", err)
	}
	if _, err := tracker.VerifyOnDiskForEdit(path); err != nil {
		t.Fatalf("expected verification to pass, got: %v", err)
	}
}

// TestFileHashTrackerVerifyWithoutPriorRead asserts that an edit attempted
// on a file the tracker has never seen returns ErrFileNotRead.
func TestFileHashTrackerVerifyWithoutPriorRead(t *testing.T) {
	path := writeTempFile(t, "foo.txt", "anything")

	tracker := NewFileHashTracker()
	_, err := tracker.VerifyOnDiskForEdit(path)
	if err == nil {
		t.Fatal("expected ErrFileNotRead, got nil")
	}
	if !errors.Is(err, ErrFileNotRead) {
		t.Fatalf("expected ErrFileNotRead, got: %v", err)
	}
}

// TestFileHashTrackerVerifyDetectsChange asserts that a mutated file on
// disk after a recorded read returns ErrFileChangedSinceRead.
func TestFileHashTrackerVerifyDetectsChange(t *testing.T) {
	path := writeTempFile(t, "foo.txt", "original")

	tracker := NewFileHashTracker()
	if _, err := tracker.RecordReadFromDisk(path); err != nil {
		t.Fatalf("record failed: %v", err)
	}
	if err := os.WriteFile(path, []byte("modified"), 0644); err != nil {
		t.Fatalf("mutation failed: %v", err)
	}

	_, err := tracker.VerifyOnDiskForEdit(path)
	if err == nil {
		t.Fatal("expected ErrFileChangedSinceRead, got nil")
	}
	if !errors.Is(err, ErrFileChangedSinceRead) {
		t.Fatalf("expected ErrFileChangedSinceRead, got: %v", err)
	}
}

// TestFileHashTrackerRecordOverwrites confirms that a second RecordRead for
// the same path replaces the prior hash entry.
func TestFileHashTrackerRecordOverwrites(t *testing.T) {
	path := writeTempFile(t, "foo.txt", "first")

	tracker := NewFileHashTracker()
	if _, err := tracker.RecordReadFromDisk(path); err != nil {
		t.Fatalf("first record failed: %v", err)
	}
	if err := os.WriteFile(path, []byte("second"), 0644); err != nil {
		t.Fatalf("rewrite failed: %v", err)
	}
	if _, err := tracker.RecordReadFromDisk(path); err != nil {
		t.Fatalf("second record failed: %v", err)
	}

	if _, err := tracker.VerifyOnDiskForEdit(path); err != nil {
		t.Fatalf("expected verification against the latest recorded content to pass, got: %v", err)
	}
}

// TestFileHashTrackerNilReceiver asserts the tracker is safe to use when
// the receiver is nil, which keeps test ergonomics intact for callers that
// have not yet wired up a real tracker.
func TestFileHashTrackerNilReceiver(t *testing.T) {
	path := writeTempFile(t, "foo.txt", "anything")

	var tracker *FileHashTracker
	if _, err := tracker.RecordReadFromDisk(path); err != nil {
		t.Fatalf("nil-receiver record unexpectedly failed: %v", err)
	}
	if _, err := tracker.VerifyOnDiskForEdit(path); err != nil {
		t.Fatalf("expected nil tracker to be permissive, got: %v", err)
	}
}

// TestFileHashTrackerConcurrent exercises the tracker from multiple
// goroutines to surface data races under `go test -race`.
func TestFileHashTrackerConcurrent(t *testing.T) {
	tracker := NewFileHashTracker()
	tempDir := t.TempDir()

	paths := make([]string, 5)
	for index := range paths {
		paths[index] = filepath.Join(tempDir, string(rune('a'+index))+".txt")
		if err := os.WriteFile(paths[index], []byte{byte(index)}, 0644); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}

	var waitGroup sync.WaitGroup
	for index := 0; index < 50; index++ {
		waitGroup.Add(2)
		path := paths[index%len(paths)]
		go func(p string) {
			defer waitGroup.Done()
			_, _ = tracker.RecordReadFromDisk(p)
		}(path)
		go func(p string) {
			defer waitGroup.Done()
			_, _ = tracker.VerifyOnDiskForEdit(p)
		}(path)
	}
	waitGroup.Wait()
}

// --- Integration tests: edit tools against a real tracker --------------------

// TestLineEditFileRejectsWithoutPriorRead asserts that calling
// line_edit_file on a file that has not been read in the current session is
// rejected with an instructive error.
func TestLineEditFileRejectsWithoutPriorRead(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("alpha\nbeta\ngamma\n"), 0644); err != nil {
		t.Fatalf("failed to seed test file: %v", err)
	}

	tracker := NewFileHashTracker()
	editTool := &LineEditFileTool{projectRoot: tempDir, hashTracker: tracker}

	_, err := editTool.Execute(map[string]any{
		"file_path":   "test.txt",
		"start_line":  2,
		"end_line":    2,
		"new_content": "BETA",
	})
	if err == nil {
		t.Fatal("expected edit without prior read to be rejected, got nil")
	}
	if !strings.Contains(err.Error(), "must be read before editing") {
		t.Errorf("expected actionable error message, got: %v", err)
	}

	current, _ := os.ReadFile(testFile)
	if string(current) != "alpha\nbeta\ngamma\n" {
		t.Errorf("expected file to remain untouched, got: %q", string(current))
	}
}

// TestLineEditFileRejectsSecondEditWithoutReread reproduces the original
// bug: after one successful edit, a second edit without an intervening
// read_file is rejected so that stale line numbers cannot corrupt the file.
func TestLineEditFileRejectsSecondEditWithoutReread(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	original := "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"
	if err := os.WriteFile(testFile, []byte(original), 0644); err != nil {
		t.Fatalf("failed to seed test file: %v", err)
	}

	tracker := NewFileHashTracker()
	readTool := &ReadFileTool{projectRoot: tempDir, hashTracker: tracker}
	editTool := &LineEditFileTool{projectRoot: tempDir, hashTracker: tracker}

	if _, err := readTool.Execute(map[string]any{"file_path": "test.txt"}); err != nil {
		t.Fatalf("initial read failed: %v", err)
	}

	if _, err := editTool.Execute(map[string]any{
		"file_path":   "test.txt",
		"start_line":  3,
		"end_line":    4,
		"new_content": "THREE\nTHREE_AND_A_HALF\nFOUR",
	}); err != nil {
		t.Fatalf("first edit unexpectedly failed: %v", err)
	}

	_, err := editTool.Execute(map[string]any{
		"file_path":   "test.txt",
		"start_line":  8,
		"end_line":    8,
		"new_content": "EIGHT",
	})
	if err == nil {
		t.Fatal("expected second edit without re-read to be rejected, got nil")
	}
	if !strings.Contains(err.Error(), "must be read before editing") {
		t.Errorf("expected actionable error message, got: %v", err)
	}
}

// TestLineEditFileSucceedsAfterReread asserts that the rejection clears
// once read_file is called again.
func TestLineEditFileSucceedsAfterReread(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("a\nb\nc\n"), 0644); err != nil {
		t.Fatalf("failed to seed test file: %v", err)
	}

	tracker := NewFileHashTracker()
	readTool := &ReadFileTool{projectRoot: tempDir, hashTracker: tracker}
	editTool := &LineEditFileTool{projectRoot: tempDir, hashTracker: tracker}

	if _, err := readTool.Execute(map[string]any{"file_path": "test.txt"}); err != nil {
		t.Fatalf("initial read failed: %v", err)
	}
	if _, err := editTool.Execute(map[string]any{
		"file_path":   "test.txt",
		"start_line":  2,
		"end_line":    2,
		"new_content": "B",
	}); err != nil {
		t.Fatalf("first edit failed: %v", err)
	}
	if _, err := readTool.Execute(map[string]any{"file_path": "test.txt"}); err != nil {
		t.Fatalf("re-read failed: %v", err)
	}
	if _, err := editTool.Execute(map[string]any{
		"file_path":   "test.txt",
		"start_line":  3,
		"end_line":    3,
		"new_content": "C2",
	}); err != nil {
		t.Fatalf("post-reread edit unexpectedly failed: %v", err)
	}

	current, _ := os.ReadFile(testFile)
	if string(current) != "a\nB\nC2" {
		t.Errorf("unexpected file contents after edits: %q", string(current))
	}
}

// TestLineEditFileDetectsExternalModification verifies that an external
// writer modifying the file between read_file and line_edit_file causes the
// edit to be rejected.
func TestLineEditFileDetectsExternalModification(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("original\n"), 0644); err != nil {
		t.Fatalf("failed to seed test file: %v", err)
	}

	tracker := NewFileHashTracker()
	readTool := &ReadFileTool{projectRoot: tempDir, hashTracker: tracker}
	editTool := &LineEditFileTool{projectRoot: tempDir, hashTracker: tracker}

	if _, err := readTool.Execute(map[string]any{"file_path": "test.txt"}); err != nil {
		t.Fatalf("initial read failed: %v", err)
	}
	if err := os.WriteFile(testFile, []byte("tampered\n"), 0644); err != nil {
		t.Fatalf("external rewrite failed: %v", err)
	}

	_, err := editTool.Execute(map[string]any{
		"file_path":   "test.txt",
		"start_line":  1,
		"end_line":    1,
		"new_content": "NEW",
	})
	if err == nil {
		t.Fatal("expected edit to be rejected after external modification, got nil")
	}
	if !strings.Contains(err.Error(), "must be read before editing") {
		t.Errorf("expected actionable error message, got: %v", err)
	}
}

// TestFindAndReplaceEditFileRejectsWithoutPriorRead mirrors the line-edit
// guard test for the find-and-replace tool.
func TestFindAndReplaceEditFileRejectsWithoutPriorRead(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world\n"), 0644); err != nil {
		t.Fatalf("failed to seed test file: %v", err)
	}

	tracker := NewFileHashTracker()
	editTool := &FindAndReplaceEditFileTool{projectRoot: tempDir, hashTracker: tracker}

	_, err := editTool.Execute(map[string]any{
		"file_path":    "test.txt",
		"find_text":    "hello",
		"replace_text": "hi",
	})
	if err == nil {
		t.Fatal("expected find-and-replace without prior read to be rejected, got nil")
	}
	if !strings.Contains(err.Error(), "must be read before editing") {
		t.Errorf("expected actionable error message, got: %v", err)
	}
}

// TestFindAndReplaceEditFileRejectsSecondEditWithoutReread is the
// find-and-replace counterpart to the line-edit stale-state regression.
func TestFindAndReplaceEditFileRejectsSecondEditWithoutReread(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("alpha beta gamma\n"), 0644); err != nil {
		t.Fatalf("failed to seed test file: %v", err)
	}

	tracker := NewFileHashTracker()
	readTool := &ReadFileTool{projectRoot: tempDir, hashTracker: tracker}
	editTool := &FindAndReplaceEditFileTool{projectRoot: tempDir, hashTracker: tracker}

	if _, err := readTool.Execute(map[string]any{"file_path": "test.txt"}); err != nil {
		t.Fatalf("initial read failed: %v", err)
	}
	if _, err := editTool.Execute(map[string]any{
		"file_path":    "test.txt",
		"find_text":    "alpha",
		"replace_text": "ALPHA",
	}); err != nil {
		t.Fatalf("first edit unexpectedly failed: %v", err)
	}
	_, err := editTool.Execute(map[string]any{
		"file_path":    "test.txt",
		"find_text":    "gamma",
		"replace_text": "GAMMA",
	})
	if err == nil {
		t.Fatal("expected second edit without re-read to be rejected, got nil")
	}
	if !strings.Contains(err.Error(), "must be read before editing") {
		t.Errorf("expected actionable error message, got: %v", err)
	}
}
