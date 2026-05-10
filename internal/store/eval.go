package store

import (
	"encoding/json"
	"fmt"
	"time"
)

// EvalCandidate is one captured retrieval call recorded for later replay.
// All fields except CapturedAt are caller-provided; the timestamp comes from
// the DB default so candidates from concurrent processes order consistently.
type EvalCandidate struct {
	ID          string   `json:"id"`
	CapturedAt  string   `json:"capturedAt"`
	Source      string   `json:"source"`    // "mcp" | "http" | "cli"
	Operation   string   `json:"operation"` // "search" | "recall" | "graph_query"
	QueryText   string   `json:"queryText"`
	ReturnedIDs []string `json:"returnedIds"`
	ResultCount int      `json:"resultCount"`
	SessionID   *string  `json:"sessionId,omitempty"`
}

// EvalStore appends and reads eval_candidates. Writes are best-effort: if the
// DB rejects the insert we log nothing (callers continue uninterrupted) and
// the metric simply isn't captured. We never want eval capture to slow down
// or fail a real retrieval.
type EvalStore struct {
	db *DB
}

func NewEvalStore(db *DB) *EvalStore {
	return &EvalStore{db: db}
}

// Append inserts one candidate. The caller is expected to have already
// scrubbed query_text. Returns nil even on DB error so the calling
// retrieval path is never blocked by a capture failure; the error is
// silently dropped.
func (s *EvalStore) Append(c EvalCandidate) error {
	if c.QueryText == "" {
		return nil
	}
	idsJSON, err := json.Marshal(c.ReturnedIDs)
	if err != nil {
		return nil
	}
	if c.ID == "" {
		c.ID = fmt.Sprintf("eval_%d", time.Now().UnixNano())
	}
	_, _ = s.db.Exec(`
		INSERT INTO eval_candidates
			(id, source, operation, query_text, returned_ids, result_count, session_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Source, c.Operation, c.QueryText, string(idsJSON), c.ResultCount, c.SessionID,
	)
	return nil
}

// List returns the most recent N candidates in reverse-chronological order.
// Used by the export endpoint and the (future) replay command.
func (s *EvalStore) List(limit int) ([]EvalCandidate, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`
		SELECT id, captured_at, source, operation, query_text, returned_ids, result_count, session_id
		  FROM eval_candidates
		 ORDER BY captured_at DESC
		 LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list eval candidates: %w", err)
	}
	defer rows.Close()

	out := make([]EvalCandidate, 0, limit)
	for rows.Next() {
		var c EvalCandidate
		var idsJSON string
		if err := rows.Scan(&c.ID, &c.CapturedAt, &c.Source, &c.Operation, &c.QueryText, &idsJSON, &c.ResultCount, &c.SessionID); err != nil {
			return nil, fmt.Errorf("scan eval candidate: %w", err)
		}
		_ = json.Unmarshal([]byte(idsJSON), &c.ReturnedIDs)
		out = append(out, c)
	}
	return out, rows.Err()
}

// Count returns total rows. Cheap; the captured_at index is small.
func (s *EvalStore) Count() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM eval_candidates`).Scan(&n)
	return n, err
}
