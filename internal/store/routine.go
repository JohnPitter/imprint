package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// RoutineRow represents a row in the routines table.
type RoutineRow struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Steps     json.RawMessage `json:"steps"`
	Tags      json.RawMessage `json:"tags"`
	Frozen    int             `json:"frozen"`
	CreatedAt string          `json:"createdAt"`
	UpdatedAt string          `json:"updatedAt"`
}

// RoutineStore provides CRUD operations for the routines table.
type RoutineStore struct {
	db *DB
}

// NewRoutineStore creates a new RoutineStore.
func NewRoutineStore(db *DB) *RoutineStore {
	return &RoutineStore{db: db}
}

// Create inserts a routine.
func (s *RoutineStore) Create(row *RoutineRow) error {
	now := TimeToString(time.Now())
	if row.CreatedAt == "" {
		row.CreatedAt = now
	}
	if row.UpdatedAt == "" {
		row.UpdatedAt = now
	}
	if len(row.Steps) == 0 {
		row.Steps = json.RawMessage("[]")
	}
	if len(row.Tags) == 0 {
		row.Tags = json.RawMessage("[]")
	}

	_, err := s.db.Exec(
		`INSERT INTO routines (id, name, steps, tags, frozen, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		row.ID, row.Name, string(row.Steps), string(row.Tags),
		row.Frozen, row.CreatedAt, row.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert routine: %w", err)
	}
	return nil
}

// GetByID retrieves a routine by ID.
func (s *RoutineStore) GetByID(id string) (*RoutineRow, error) {
	row := s.db.QueryRow(
		`SELECT id, name, COALESCE(steps,'[]'), COALESCE(tags,'[]'), frozen, created_at, updated_at
		 FROM routines WHERE id = ?`, id)
	return s.scanRoutine(row)
}

// List returns routines with pagination.
func (s *RoutineStore) List(limit, offset int) ([]RoutineRow, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.Query(
		`SELECT id, name, COALESCE(steps,'[]'), COALESCE(tags,'[]'), frozen, created_at, updated_at
		 FROM routines
		 ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list routines: %w", err)
	}
	defer rows.Close()

	return s.scanRoutines(rows)
}

// Update updates an existing routine.
func (s *RoutineStore) Update(row *RoutineRow) error {
	row.UpdatedAt = TimeToString(time.Now())
	if len(row.Steps) == 0 {
		row.Steps = json.RawMessage("[]")
	}
	if len(row.Tags) == 0 {
		row.Tags = json.RawMessage("[]")
	}

	_, err := s.db.Exec(
		`UPDATE routines SET name = ?, steps = ?, tags = ?, frozen = ?, updated_at = ?
		 WHERE id = ?`,
		row.Name, string(row.Steps), string(row.Tags),
		row.Frozen, row.UpdatedAt, row.ID,
	)
	if err != nil {
		return fmt.Errorf("update routine: %w", err)
	}
	return nil
}

// Delete removes a routine by ID.
func (s *RoutineStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM routines WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete routine: %w", err)
	}
	return nil
}

// --- Scan helpers ---

func (s *RoutineStore) scanRoutine(row *sql.Row) (*RoutineRow, error) {
	var r RoutineRow
	var steps, tags string

	err := row.Scan(&r.ID, &r.Name, &steps, &tags, &r.Frozen, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("routine not found")
		}
		return nil, fmt.Errorf("scan routine: %w", err)
	}

	r.Steps = json.RawMessage(steps)
	r.Tags = json.RawMessage(tags)
	return &r, nil
}

func (s *RoutineStore) scanRoutines(rows *sql.Rows) ([]RoutineRow, error) {
	var result []RoutineRow

	for rows.Next() {
		var r RoutineRow
		var steps, tags string

		if err := rows.Scan(&r.ID, &r.Name, &steps, &tags, &r.Frozen, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan routine row: %w", err)
		}

		r.Steps = json.RawMessage(steps)
		r.Tags = json.RawMessage(tags)
		result = append(result, r)
	}
	return result, rows.Err()
}
