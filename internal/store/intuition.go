package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// IntuitionRow is one rooted memory: a cross-cutting premise about how to reason
// in a context. Born only by convergence (3.5); auto-weakened by contradiction;
// always inspectable (invariant 11).
type IntuitionRow struct {
	ID                 string   `json:"id"`
	CreatedAt          string   `json:"createdAt"`
	UpdatedAt          string   `json:"updatedAt"`
	Project            string   `json:"project"`
	Statement          string   `json:"statement"`
	Strength           float64  `json:"strength"`
	EvidenceIDs        []string `json:"evidenceIds"`
	EvidenceCount      int      `json:"evidenceCount"`
	Concepts           []string `json:"concepts"`
	Files              []string `json:"files"`
	LastContradictedAt *string  `json:"lastContradictedAt,omitempty"`
	ContradictionCount int      `json:"contradictionCount"`
	Status             string   `json:"status"`
	SchemaVersion      int      `json:"schemaVersion"`
	BornSessionID      string   `json:"bornSessionId,omitempty"`
}

// Contradiction is one logged contradiction event behind the auto-weakening.
type Contradiction struct {
	ID            string  `json:"id"`
	IntuitionID   string  `json:"intuitionId"`
	TS            string  `json:"ts"`
	MemoryID      string  `json:"memoryId"`
	Detail        string  `json:"detail"`
	StrengthDelta float64 `json:"strengthDelta"`
}

// IntuitionStore is the rooted-layer store. Demotion floor and birth bar are
// policy decisions enforced by the service; the store is mechanism only.
type IntuitionStore struct {
	db *DB
}

func NewIntuitionStore(db *DB) *IntuitionStore { return &IntuitionStore{db: db} }

// Create inserts a new active intuition.
func (s *IntuitionStore) Create(row *IntuitionRow) error {
	now := TimeToString(time.Now())
	if row.CreatedAt == "" {
		row.CreatedAt = now
	}
	row.UpdatedAt = now
	if row.Strength == 0 {
		row.Strength = 6
	}
	if row.Status == "" {
		row.Status = "active"
	}
	if row.SchemaVersion == 0 {
		row.SchemaVersion = 1
	}
	evJSON, _ := json.Marshal(row.EvidenceIDs)
	conJSON, _ := json.Marshal(row.Concepts)
	filesJSON, _ := json.Marshal(row.Files)
	_, err := s.db.Exec(`
		INSERT INTO intuitions
			(id, created_at, updated_at, project, statement, strength,
			 evidence_ids, evidence_count, concepts, files, status,
			 schema_version, born_session_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.ID, row.CreatedAt, row.UpdatedAt, row.Project, row.Statement, row.Strength,
		string(evJSON), row.EvidenceCount, string(conJSON), string(filesJSON), row.Status,
		row.SchemaVersion, nullable(row.BornSessionID),
	)
	if err != nil {
		return fmt.Errorf("insert intuition: %w", err)
	}
	return nil
}

// ListActive returns active intuitions for a project (empty = all), strongest
// first, capped at limit. Used by the resident injection slot.
func (s *IntuitionStore) ListActive(project string, limit int) ([]IntuitionRow, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows *sql.Rows
	var err error
	if project != "" {
		rows, err = s.db.Query(intuitionSelect+` WHERE status = 'active' AND project = ? ORDER BY strength DESC LIMIT ?`, project, limit)
	} else {
		rows, err = s.db.Query(intuitionSelect+` WHERE status = 'active' ORDER BY strength DESC LIMIT ?`, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list active intuitions: %w", err)
	}
	defer rows.Close()
	return scanIntuitions(rows)
}

// ListAll returns intuitions of any status for a project (for the inspection UI).
func (s *IntuitionStore) ListAll(project string, limit int) ([]IntuitionRow, error) {
	if limit <= 0 {
		limit = 200
	}
	var rows *sql.Rows
	var err error
	if project != "" {
		rows, err = s.db.Query(intuitionSelect+` WHERE project = ? ORDER BY strength DESC LIMIT ?`, project, limit)
	} else {
		rows, err = s.db.Query(intuitionSelect+` ORDER BY strength DESC LIMIT ?`, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list intuitions: %w", err)
	}
	defer rows.Close()
	return scanIntuitions(rows)
}

// GetByID returns one intuition.
func (s *IntuitionStore) GetByID(id string) (*IntuitionRow, error) {
	rows, err := s.db.Query(intuitionSelect+` WHERE id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("get intuition: %w", err)
	}
	defer rows.Close()
	out, err := scanIntuitions(rows)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("intuition not found")
	}
	return &out[0], nil
}

// CountActive returns how many active intuitions a project has — used to enforce
// the hard quantity cap before rooting a new one.
func (s *IntuitionStore) CountActive(project string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM intuitions WHERE status = 'active' AND project = ?`, project).Scan(&n)
	return n, err
}

// WeakestActive returns the lowest-strength active intuition for a project, or
// nil if none. Used to make room when the cap is exceeded.
func (s *IntuitionStore) WeakestActive(project string) (*IntuitionRow, error) {
	rows, err := s.db.Query(intuitionSelect+` WHERE status = 'active' AND project = ? ORDER BY strength ASC LIMIT 1`, project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := scanIntuitions(rows)
	if err != nil || len(out) == 0 {
		return nil, err
	}
	return &out[0], nil
}

// RecordContradiction logs a contradiction (append-only) and lowers the
// intuition's strength by delta. When the strength crosses the floor it is
// demoted back to refined. Returns the new strength and whether it was demoted.
func (s *IntuitionStore) RecordContradiction(intuitionID, memoryID, detail string, delta, floor float64) (float64, bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, false, fmt.Errorf("begin contradiction tx: %w", err)
	}
	var strength float64
	if err := tx.QueryRow(`SELECT strength FROM intuitions WHERE id = ?`, intuitionID).Scan(&strength); err != nil {
		tx.Rollback()
		return 0, false, fmt.Errorf("load intuition strength: %w", err)
	}
	newStrength := strength - delta
	demoted := newStrength <= floor
	status := "active"
	if demoted {
		status = "demoted"
	}
	now := TimeToString(time.Now())
	if _, err := tx.Exec(`
		UPDATE intuitions
		   SET strength = ?, status = ?, updated_at = ?,
		       last_contradicted_at = ?, contradiction_count = contradiction_count + 1
		 WHERE id = ?`, newStrength, status, now, now, intuitionID); err != nil {
		tx.Rollback()
		return 0, false, fmt.Errorf("update intuition strength: %w", err)
	}
	if _, err := tx.Exec(`
		INSERT INTO intuition_contradictions (id, intuition_id, memory_id, detail, strength_delta)
		VALUES (?, ?, ?, ?, ?)`,
		ledgerID("contra"), intuitionID, memoryID, detail, delta); err != nil {
		tx.Rollback()
		return 0, false, fmt.Errorf("insert contradiction: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, false, fmt.Errorf("commit contradiction tx: %w", err)
	}
	return newStrength, demoted, nil
}

// ContradictionExists reports whether this (intuition, memory) pair was already
// logged — so a background pass never penalizes the same pair twice and never
// re-spends an LLM call re-judging it.
func (s *IntuitionStore) ContradictionExists(intuitionID, memoryID string) (bool, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM intuition_contradictions WHERE intuition_id = ? AND memory_id = ?`,
		intuitionID, memoryID).Scan(&n)
	return n > 0, err
}

// Contradictions returns the contradiction log for an intuition, newest first.
func (s *IntuitionStore) Contradictions(intuitionID string, limit int) ([]Contradiction, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT id, intuition_id, ts, memory_id, detail, strength_delta
		  FROM intuition_contradictions
		 WHERE intuition_id = ?
		 ORDER BY ts DESC LIMIT ?`, intuitionID, limit)
	if err != nil {
		return nil, fmt.Errorf("list contradictions: %w", err)
	}
	defer rows.Close()
	var out []Contradiction
	for rows.Next() {
		var c Contradiction
		if err := rows.Scan(&c.ID, &c.IntuitionID, &c.TS, &c.MemoryID, &c.Detail, &c.StrengthDelta); err != nil {
			return nil, fmt.Errorf("scan contradiction: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetStatus changes an intuition's status (manual demote/archive via UI).
func (s *IntuitionStore) SetStatus(id, status string) error {
	_, err := s.db.Exec(`UPDATE intuitions SET status = ?, updated_at = ? WHERE id = ?`,
		status, TimeToString(time.Now()), id)
	if err != nil {
		return fmt.Errorf("set intuition status: %w", err)
	}
	return nil
}

// Reinforce raises an intuition's strength (capped at 10), used when convergence
// re-confirms it. Caps prevent unbounded growth.
func (s *IntuitionStore) Reinforce(id string, delta float64) error {
	_, err := s.db.Exec(`
		UPDATE intuitions SET strength = MIN(10, strength + ?), updated_at = ? WHERE id = ?`,
		delta, TimeToString(time.Now()), id)
	return err
}

// HardDelete permanently removes an intuition and its contradiction log
// (A5 — apagar é apagar).
func (s *IntuitionStore) HardDelete(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM intuition_contradictions WHERE intuition_id = ?`, id); err != nil {
		tx.Rollback()
		return fmt.Errorf("delete contradictions: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM intuitions WHERE id = ?`, id); err != nil {
		tx.Rollback()
		return fmt.Errorf("delete intuition: %w", err)
	}
	return tx.Commit()
}

// DeleteByProject removes every intuition (and its log) for a repo (A5).
func (s *IntuitionStore) DeleteByProject(project string) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`
		DELETE FROM intuition_contradictions
		 WHERE intuition_id IN (SELECT id FROM intuitions WHERE project = ?)`, project); err != nil {
		tx.Rollback()
		return 0, err
	}
	res, err := tx.Exec(`DELETE FROM intuitions WHERE project = ?`, project)
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, tx.Commit()
}

const intuitionSelect = `
	SELECT id, created_at, updated_at, project, statement, strength,
	       COALESCE(evidence_ids,'[]'), evidence_count,
	       COALESCE(concepts,'[]'), COALESCE(files,'[]'),
	       last_contradicted_at, contradiction_count, status,
	       schema_version, COALESCE(born_session_id,'')
	  FROM intuitions`

func scanIntuitions(rows *sql.Rows) ([]IntuitionRow, error) {
	var out []IntuitionRow
	for rows.Next() {
		var r IntuitionRow
		var evJSON, conJSON, filesJSON string
		var lastContra sql.NullString
		if err := rows.Scan(
			&r.ID, &r.CreatedAt, &r.UpdatedAt, &r.Project, &r.Statement, &r.Strength,
			&evJSON, &r.EvidenceCount, &conJSON, &filesJSON,
			&lastContra, &r.ContradictionCount, &r.Status, &r.SchemaVersion, &r.BornSessionID,
		); err != nil {
			return nil, fmt.Errorf("scan intuition: %w", err)
		}
		_ = json.Unmarshal([]byte(evJSON), &r.EvidenceIDs)
		_ = json.Unmarshal([]byte(conJSON), &r.Concepts)
		_ = json.Unmarshal([]byte(filesJSON), &r.Files)
		if lastContra.Valid {
			r.LastContradictedAt = &lastContra.String
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
