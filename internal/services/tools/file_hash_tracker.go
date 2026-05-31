package tools

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync"
)

// ErrFileNotRead indicates that an edit was attempted on a file that has not
// been read in the current session. The caller must invoke read_file before
// editing so that line numbers and content assumptions are accurate.
var ErrFileNotRead = errors.New("file has not been read in this session")

// ErrFileChangedSinceRead indicates that the on-disk contents of a file no
// longer match what was last recorded by a read_file call. This happens when
// a prior edit in this session modified the file, or when something external
// to the agent changed it. The caller must invoke read_file again before
// editing.
var ErrFileChangedSinceRead = errors.New("file contents have changed since it was last read")

// FileHashTracker stores SHA-256 hashes of file contents at the time they
// were last read by the read_file tool. Edit tools consult this tracker
// before mutating a file and refuse to proceed when the on-disk contents do
// not match the recorded hash. The tracker is purely in-memory and is scoped
// to a single ToolRegistry instance, which means each top-level session has
// its own isolated view.
type FileHashTracker struct {
	mutex  sync.RWMutex
	hashes map[string]string
}

// NewFileHashTracker constructs an empty tracker.
func NewFileHashTracker() *FileHashTracker {
	return &FileHashTracker{
		hashes: make(map[string]string),
	}
}

// readAndHashFile is the single source of truth for how this package
// obtains a file's bytes and hash for tracking purposes. Both the record
// path and the verify path must go through it so they cannot drift apart
// (e.g. one using buffered reading or encoding normalization while the
// other does not).
func readAndHashFile(absolutePath string) ([]byte, string, error) {
	bytes, readError := os.ReadFile(absolutePath)
	if readError != nil {
		return nil, "", readError
	}
	return bytes, hashBytes(bytes), nil
}

// RecordReadFromDisk reads the file at absolutePath, hashes it, and stores
// the hash under that path. The file's raw bytes are returned to the caller
// so a separate disk read is not required. Any prior entry for the path is
// overwritten.
func (tracker *FileHashTracker) RecordReadFromDisk(absolutePath string) ([]byte, error) {
	bytes, hash, readError := readAndHashFile(absolutePath)
	if readError != nil {
		return nil, readError
	}
	if tracker != nil {
		tracker.mutex.Lock()
		tracker.hashes[absolutePath] = hash
		tracker.mutex.Unlock()
	}
	return bytes, nil
}

// VerifyOnDiskForEdit reads the file at absolutePath, hashes it, and
// compares the hash to what was recorded by the most recent
// RecordReadFromDisk call. It returns the file's raw bytes alongside the
// verification result so callers performing an edit do not need a second
// disk read. The error is ErrFileNotRead if no entry exists for the path,
// ErrFileChangedSinceRead if the contents differ, or nil if the edit may
// proceed.
func (tracker *FileHashTracker) VerifyOnDiskForEdit(absolutePath string) ([]byte, error) {
	bytes, currentHash, readError := readAndHashFile(absolutePath)
	if readError != nil {
		return nil, readError
	}

	if tracker == nil {
		// A nil tracker is treated as permissive so that tests and code paths
		// that have not been wired up yet are not silently blocked. Production
		// construction in NewToolRegistry always supplies a real instance.
		return bytes, nil
	}

	tracker.mutex.RLock()
	storedHash, exists := tracker.hashes[absolutePath]
	tracker.mutex.RUnlock()

	if !exists {
		return bytes, fmt.Errorf("%w: %s", ErrFileNotRead, absolutePath)
	}
	if currentHash != storedHash {
		return bytes, fmt.Errorf("%w: %s", ErrFileChangedSinceRead, absolutePath)
	}
	return bytes, nil
}

// hashBytes returns the lowercase hex SHA-256 of the supplied bytes.
func hashBytes(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
