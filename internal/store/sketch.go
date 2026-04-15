package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// SketchRow represents a row in the sketches table.
type SketchRow struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Status      string          `json:"status"`
	ActionIDs   json.RawMessage `json:"actionIds"`
	Project     *string         `json:"project"`
	CreatedAt   string          `json:"createdAt"`
	ExpiresAt   string          `json:"expiresAt"`
	PromotedAt  *string         `json:"promotedAt"`
	DiscardedAt *string         `json:"discardedAt"`
}

// SketchStore provides CRUD operations for the sketches table.
type SketchStore struct {
	db *DB
}

// NewSketchStore creates a new SketchStore.
func NewSketchStore(db *DB) *SketchStore {
	return &SketchStore{db: db}
}

// Create inserts a sketch.
func (s *SketchStore) Create(sk *SketchRow) error {
	if sk.CreatedAt == "" {
		sk.CreatedAt = TimeToString(time.Now())
	}
	if sk.Status == "" {
		sk.Status = "active"
	}
	if len(sk.ActionIDs) == 0 {
		sk.ActionIDs = json.RawMessage("[]")
	}

	_, err := s.db.Exec(
		`INSERT INTO sketches (id, title, description, status, action_ids, project,
		    created_at, expires_at, promoted_at, discarded_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sk.ID, sk.Title, sk.Description, sk.Status, string(sk.ActionIDs),
		NullString(sk.Project), sk.CreatedAt, sk.ExpiresAt,
		NullString(sk.PromotedAt), NullString(sk.DiscardedAt),
	)
	if err != nil {
		return fmt.Errorf("insert sketch: %w", err)
	}
	return nil
}

// GetByID retrieves a sketch by ID.
func (s *SketchStore) GetByID(id string) (*SketchRow, error) {
	row := s.db.QueryRow(
		`SELECT id, title, description, status, COALESCE(action_ids,'[]'),
		        project, created_at, COALESCE(expires_at,''), promoted_at, discarded_at
		 FROM sketches WHERE id = ?`, id)
	return s.scanSketch(row)
}

// List returns sketches with optional status filter, ordered by most recent first.
func (s *SketchStore) List(status string, limit int) ([]SketchRow, error) {
	if limit <= 0 {
		limit = 100
	}

	var rows *sql.Rows
	var err error

	if status != "" {
		rows, err = s.db.Query(
			`SELECT id, title, description, status, COALESCE(action_ids,'[]'),
			        project, created_at, COALESCE(expires_at,''), promoted_at, discarded_at
			 FROM sketches WHERE status = ?
			 ORDER BY created_at DESC LIMIT ?`, status, limit)
	} else {
		rows, err = s.db.Query(
			`SELECT id, title, description, status, COALESCE(action_ids,'[]'),
			        project, created_at, COALESCE(expires_at,''), promoted_at, discarded_at
			 FROM sketches
			 ORDER BY created_at DESC LIMIT ?`, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list sketches: %w", err)
	}
	defer rows.Close()

	return s.scanSketches(rows)
}

// AddAction appends an action ID to the sketch's action_ids array.
func (s *SketchStore) AddAction(sketchID, actionID string) error {
	row := s.db.QueryRow(
		`SELECT COALESCE(action_ids,'[]') FROM sketches WHERE id = ?`, sketchID)

	var actionIDsStr string
	if err := row.Scan(&actionIDsStr); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("sketch not found")
		}
		return fmt.Errorf("scan sketch action_ids: %w", err)
	}

	var actionIDs []string
	if err := json.Unmarshal([]byte(actionIDsStr), &actionIDs); err != nil {
		actionIDs = []string{}
	}

	// Check if already present.
	for _, a := range actionIDs {
		if a == actionID {
			return nil
		}
	}

	actionIDs = append(actionIDs, actionID)
	updated := MarshalJSON(actionIDs)

	_, err := s.db.Exec(`UPDATE sketches SET action_ids = ? WHERE id = ?`, updated, sketchID)
	if err != nil {
		return fmt.Errorf("add action to sketch: %w", err)
	}
	return nil
}

// Promote marks a sketch as promoted.
func (s *SketchStore) Promote(id string) error {
	now := TimeToString(time.Now())

	_, err := s.db.Exec(
		`UPDATE sketches SET status = 'promoted', promoted_at = ? WHERE id = ?`,
		now, id)
	if err != nil {
		return fmt.Errorf("promote sketch: %w", err)
	}
	return nil
}

// Discard marks a sketch as discarded.
func (s *SketchStore) Discard(id string) error {
	now := TimeToString(time.Now())

	_, err := s.db.Exec(
		`UPDATE sketches SET status = 'discarded', discarded_at = ? WHERE id = ?`,
		now, id)
	if err != nil {
		return fmt.Errorf("discard sketch: %w", err)
	}
	return nil
}

// GarbageCollect deletes expired sketches and returns the number of rows removed.
func (s *SketchStore) GarbageCollect() (int64, error) {
	now := TimeToString(time.Now())

	res, err := s.db.Exec(
		`DELETE FROM sketches WHERE expires_at != '' AND expires_at < ? AND status = 'active'`,
		now)
	if err != nil {
		return 0, fmt.Errorf("garbage collect sketches: %w", err)
	}

	count, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return count, nil
}

// --- Scan helpers ---

func (s *SketchStore) scanSketch(row *sql.Row) (*SketchRow, error) {
	var sk SketchRow
	var actionIDs string
	var project, promotedAt, discardedAt sql.NullString

	err := row.Scan(&sk.ID, &sk.Title, &sk.Description, &sk.Status, &actionIDs,
		&project, &sk.CreatedAt, &sk.ExpiresAt, &promotedAt, &discardedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("sketch not found")
		}
		return nil, fmt.Errorf("scan sketch: %w", err)
	}

	sk.ActionIDs = json.RawMessage(actionIDs)
	if project.Valid {
		sk.Project = &project.String
	}
	if promotedAt.Valid {
		sk.PromotedAt = &promotedAt.String
	}
	if discardedAt.Valid {
		sk.DiscardedAt = &discardedAt.String
	}
	return &sk, nil
}

func (s *SketchStore) scanSketches(rows *sql.Rows) ([]SketchRow, error) {
	var results []SketchRow

	for rows.Next() {
		var sk SketchRow
		var actionIDs string
		var project, promotedAt, discardedAt sql.NullString

		if err := rows.Scan(&sk.ID, &sk.Title, &sk.Description, &sk.Status, &actionIDs,
			&project, &sk.CreatedAt, &sk.ExpiresAt, &promotedAt, &discardedAt); err != nil {
			return nil, fmt.Errorf("scan sketch row: %w", err)
		}

		sk.ActionIDs = json.RawMessage(actionIDs)
		if project.Valid {
			sk.Project = &project.String
		}
		if promotedAt.Valid {
			sk.PromotedAt = &promotedAt.String
		}
		if discardedAt.Valid {
			sk.DiscardedAt = &discardedAt.String
		}
		results = append(results, sk)
	}
	return results, rows.Err()
}
