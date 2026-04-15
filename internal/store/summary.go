package store

import (
	"database/sql"
	"fmt"
	"time"
)

// SummaryRow represents a row in the session_summaries table.
type SummaryRow struct {
	SessionID        string
	Project          string
	CreatedAt        string
	Title            string
	Narrative        string
	KeyDecisions     string
	FilesModified    string
	Concepts         string
	ObservationCount int
}

// SummaryStore provides CRUD operations for the session_summaries table.
type SummaryStore struct {
	db *DB
}

// NewSummaryStore creates a new SummaryStore.
func NewSummaryStore(db *DB) *SummaryStore {
	return &SummaryStore{db: db}
}

// Create inserts or replaces a session summary.
// Uses INSERT OR REPLACE since session_id is the primary key.
func (s *SummaryStore) Create(row *SummaryRow) error {
	if row.CreatedAt == "" {
		row.CreatedAt = TimeToString(time.Now())
	}
	if row.KeyDecisions == "" {
		row.KeyDecisions = "[]"
	}
	if row.FilesModified == "" {
		row.FilesModified = "[]"
	}
	if row.Concepts == "" {
		row.Concepts = "[]"
	}

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO session_summaries (
			session_id, project, created_at, title, narrative,
			key_decisions, files_modified, concepts, observation_count
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.SessionID, row.Project, row.CreatedAt, row.Title, row.Narrative,
		row.KeyDecisions, row.FilesModified, row.Concepts, row.ObservationCount,
	)
	if err != nil {
		return fmt.Errorf("insert session summary: %w", err)
	}
	return nil
}

// GetBySessionID returns the summary for a session.
func (s *SummaryStore) GetBySessionID(sessionID string) (*SummaryRow, error) {
	row := s.db.QueryRow(`
		SELECT session_id, project, created_at, title, narrative,
			COALESCE(key_decisions, '[]'), COALESCE(files_modified, '[]'),
			COALESCE(concepts, '[]'), observation_count
		FROM session_summaries
		WHERE session_id = ?`, sessionID)

	return s.scanRow(row)
}

// ListByProject returns summaries for a project, ordered by created_at DESC.
func (s *SummaryStore) ListByProject(project string, limit int) ([]SummaryRow, error) {
	rows, err := s.db.Query(`
		SELECT session_id, project, created_at, title, narrative,
			COALESCE(key_decisions, '[]'), COALESCE(files_modified, '[]'),
			COALESCE(concepts, '[]'), observation_count
		FROM session_summaries
		WHERE project = ?
		ORDER BY created_at DESC
		LIMIT ?`, project, limit)
	if err != nil {
		return nil, fmt.Errorf("list summaries by project: %w", err)
	}
	defer rows.Close()

	return s.scanRows(rows)
}

// ListRecent returns the most recent summaries across all projects.
func (s *SummaryStore) ListRecent(limit int) ([]SummaryRow, error) {
	rows, err := s.db.Query(`
		SELECT session_id, project, created_at, title, narrative,
			COALESCE(key_decisions, '[]'), COALESCE(files_modified, '[]'),
			COALESCE(concepts, '[]'), observation_count
		FROM session_summaries
		ORDER BY created_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent summaries: %w", err)
	}
	defer rows.Close()

	return s.scanRows(rows)
}

// Delete removes a summary by session ID.
func (s *SummaryStore) Delete(sessionID string) error {
	_, err := s.db.Exec(`DELETE FROM session_summaries WHERE session_id = ?`, sessionID)
	if err != nil {
		return fmt.Errorf("delete session summary: %w", err)
	}
	return nil
}

// scanRow scans a single row into a SummaryRow.
func (s *SummaryStore) scanRow(row *sql.Row) (*SummaryRow, error) {
	var sr SummaryRow
	err := row.Scan(
		&sr.SessionID, &sr.Project, &sr.CreatedAt, &sr.Title, &sr.Narrative,
		&sr.KeyDecisions, &sr.FilesModified, &sr.Concepts, &sr.ObservationCount,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session summary not found")
		}
		return nil, fmt.Errorf("scan session summary: %w", err)
	}
	return &sr, nil
}

// scanRows scans multiple rows into a slice of SummaryRow.
func (s *SummaryStore) scanRows(rows *sql.Rows) ([]SummaryRow, error) {
	var result []SummaryRow

	for rows.Next() {
		var sr SummaryRow
		err := rows.Scan(
			&sr.SessionID, &sr.Project, &sr.CreatedAt, &sr.Title, &sr.Narrative,
			&sr.KeyDecisions, &sr.FilesModified, &sr.Concepts, &sr.ObservationCount,
		)
		if err != nil {
			return nil, fmt.Errorf("scan summary row: %w", err)
		}
		result = append(result, sr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate summary rows: %w", err)
	}

	return result, nil
}
