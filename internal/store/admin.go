package store

import (
	"encoding/json"
	"fmt"
)

// AdminStore implements the user's right over their own memory (A5): purge a
// repo's memory, reset everything to cold start, and export/import. "Apagar é
// apagar": these are hard deletes of the actual rows, not soft hides. Callers
// must also drop the affected docs from the BM25/vector indexes.
type AdminStore struct {
	db *DB
}

func NewAdminStore(db *DB) *AdminStore { return &AdminStore{db: db} }

// PurgeProject hard-deletes all memory belonging to one repo across every layer
// and the ledger, in FK-dependency order, atomically. Returns the per-table
// counts. Memories are global rows scoped via their sessions; a memory shared
// across repos is removed if any of its sessions belong to this project
// (documented edge — memories are overwhelmingly single-repo).
func (s *AdminStore) PurgeProject(project string) (map[string]int64, error) {
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	counts := map[string]int64{}
	exec := func(label, query string, args ...any) error {
		res, err := tx.Exec(query, args...)
		if err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
		n, _ := res.RowsAffected()
		counts[label] = n
		return nil
	}

	// Children first (FKs have no ON DELETE CASCADE).
	steps := []struct {
		label, query string
	}{
		{"intuition_contradictions", `DELETE FROM intuition_contradictions WHERE intuition_id IN (SELECT id FROM intuitions WHERE project = ?)`},
		{"intuitions", `DELETE FROM intuitions WHERE project = ?`},
		{"injection_log", `DELETE FROM injection_log WHERE project = ?`},
		{"token_ledger", `DELETE FROM token_ledger WHERE project = ? OR session_id IN (SELECT id FROM sessions WHERE project = ?)`},
		{"memories", `DELETE FROM memories WHERE id IN (
			SELECT m.id FROM memories m
			  JOIN json_each(m.session_ids) je ON 1=1
			  JOIN sessions s ON s.id = je.value
			 WHERE s.project = ?)`},
		{"session_summaries", `DELETE FROM session_summaries WHERE project = ?`},
		{"embeddings", `DELETE FROM embeddings WHERE session_id IN (SELECT id FROM sessions WHERE project = ?)`},
		{"compressed_observations", `DELETE FROM compressed_observations WHERE session_id IN (SELECT id FROM sessions WHERE project = ?)`},
		{"raw_observations", `DELETE FROM raw_observations WHERE session_id IN (SELECT id FROM sessions WHERE project = ?)`},
		{"sessions", `DELETE FROM sessions WHERE project = ?`},
	}
	for _, st := range steps {
		// token_ledger needs the project twice; everything else once.
		if st.label == "token_ledger" {
			if err := exec(st.label, st.query, project, project); err != nil {
				tx.Rollback()
				return nil, err
			}
			continue
		}
		if err := exec(st.label, st.query, project); err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit purge: %w", err)
	}
	return counts, nil
}

// ResetAll wipes every memory table back to cold start. Irreversible.
func (s *AdminStore) ResetAll() error {
	tables := []string{
		"intuition_contradictions", "intuitions", "injection_log", "token_ledger",
		"memories", "semantic_memories", "procedural_memories", "session_summaries",
		"embeddings", "compressed_observations", "raw_observations",
		"graph_edges", "graph_nodes", "lessons", "insights", "sessions", "dedup_cache",
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for _, t := range tables {
		if _, err := tx.Exec("DELETE FROM " + t); err != nil {
			tx.Rollback()
			return fmt.Errorf("reset %s: %w", t, err)
		}
	}
	return tx.Commit()
}

// MemoryExport is the portable, versioned snapshot of a repo's memory (A5).
type MemoryExport struct {
	SchemaVersion int                        `json:"schemaVersion"`
	Project       string                     `json:"project"`
	Memories      []MemoryRow                `json:"memories"`
	Intuitions    []IntuitionRow             `json:"intuitions"`
	Summaries     []SummaryRow               `json:"summaries"`
	Compressed    []CompressedObservationRow `json:"compressed"`
}

// HasProject reports whether any session exists for the project.
func (s *AdminStore) HasProject(project string) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE project = ?`, project).Scan(&n)
	return n > 0, err
}

// MarshalExport serializes an export to indented JSON.
func MarshalExport(e *MemoryExport) ([]byte, error) {
	return json.MarshalIndent(e, "", "  ")
}
