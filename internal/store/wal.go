package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// WALEntry represents a single entry in the write-ahead log.
// Content is never stored — only metadata and content length (redacted by design).
type WALEntry struct {
	Timestamp  string         `json:"ts"`
	Operation  string         `json:"op"`     // "create", "update", "delete"
	Entity     string         `json:"entity"` // "observation", "memory", "session", etc.
	EntityID   string         `json:"id"`
	ContentLen int            `json:"content_len"` // length of content, not content itself
	Meta       map[string]any `json:"meta,omitempty"`
}

// WAL is an append-only JSONL file for auditing every write operation.
// Separate from the SQLite audit table — this is a file-based log for
// crash recovery and memory poisoning detection.
type WAL struct {
	mu   sync.Mutex
	file *os.File
}

// NewWAL creates a new WAL that writes to dataDir/wal/write_log.jsonl.
// The wal directory is created with 0700 permissions (owner-only).
func NewWAL(dataDir string) (*WAL, error) {
	walDir := filepath.Join(dataDir, "wal")
	if err := os.MkdirAll(walDir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(walDir, "write_log.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	return &WAL{file: f}, nil
}

// Log appends an entry to the WAL. Thread-safe.
// Errors are silently ignored — the WAL must never block normal operation.
func (w *WAL) Log(entry WALEntry) {
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	// Write entry + newline as a single append (JSONL format).
	w.file.Write(append(data, '\n'))
}

// Close flushes and closes the underlying file.
func (w *WAL) Close() error {
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}
