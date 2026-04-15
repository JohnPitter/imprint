package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// CheckpointRow represents a row in the checkpoints table.
type CheckpointRow struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Status          string          `json:"status"`
	Type            string          `json:"type"`
	ActionID        *string         `json:"actionId"`
	Config          json.RawMessage `json:"config"`
	CreatedAt       string          `json:"createdAt"`
	ResolvedAt      *string         `json:"resolvedAt"`
	ResolvedBy      *string         `json:"resolvedBy"`
	Result          *string         `json:"result"`
	ExpiresAt       *string         `json:"expiresAt"`
	LinkedActionIDs json.RawMessage `json:"linkedActionIds"`
}

// CheckpointStore provides CRUD operations for the checkpoints table.
type CheckpointStore struct {
	db *DB
}

// NewCheckpointStore creates a new CheckpointStore.
func NewCheckpointStore(db *DB) *CheckpointStore {
	return &CheckpointStore{db: db}
}

// Create inserts a checkpoint.
func (s *CheckpointStore) Create(cp *CheckpointRow) error {
	if cp.CreatedAt == "" {
		cp.CreatedAt = TimeToString(time.Now())
	}
	if cp.Status == "" {
		cp.Status = "pending"
	}
	if cp.Type == "" {
		cp.Type = "approval"
	}
	if len(cp.Config) == 0 {
		cp.Config = json.RawMessage("{}")
	}
	if len(cp.LinkedActionIDs) == 0 {
		cp.LinkedActionIDs = json.RawMessage("[]")
	}

	_, err := s.db.Exec(
		`INSERT INTO checkpoints (id, name, description, status, type, action_id, config,
		    created_at, resolved_at, resolved_by, result, expires_at, linked_action_ids)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cp.ID, cp.Name, cp.Description, cp.Status, cp.Type,
		NullString(cp.ActionID), string(cp.Config),
		cp.CreatedAt, NullString(cp.ResolvedAt), NullString(cp.ResolvedBy),
		NullString(cp.Result), NullString(cp.ExpiresAt), string(cp.LinkedActionIDs),
	)
	if err != nil {
		return fmt.Errorf("insert checkpoint: %w", err)
	}
	return nil
}

// GetByID retrieves a checkpoint by ID.
func (s *CheckpointStore) GetByID(id string) (*CheckpointRow, error) {
	row := s.db.QueryRow(
		`SELECT id, name, description, status, type, action_id,
		        COALESCE(config,'{}'), created_at, resolved_at, resolved_by,
		        result, expires_at, COALESCE(linked_action_ids,'[]')
		 FROM checkpoints WHERE id = ?`, id)
	return s.scanCheckpoint(row)
}

// List returns checkpoints with optional status filter, ordered by most recent first.
func (s *CheckpointStore) List(status string, limit int) ([]CheckpointRow, error) {
	if limit <= 0 {
		limit = 100
	}

	var rows *sql.Rows
	var err error

	if status != "" {
		rows, err = s.db.Query(
			`SELECT id, name, description, status, type, action_id,
			        COALESCE(config,'{}'), created_at, resolved_at, resolved_by,
			        result, expires_at, COALESCE(linked_action_ids,'[]')
			 FROM checkpoints WHERE status = ?
			 ORDER BY created_at DESC LIMIT ?`, status, limit)
	} else {
		rows, err = s.db.Query(
			`SELECT id, name, description, status, type, action_id,
			        COALESCE(config,'{}'), created_at, resolved_at, resolved_by,
			        result, expires_at, COALESCE(linked_action_ids,'[]')
			 FROM checkpoints
			 ORDER BY created_at DESC LIMIT ?`, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list checkpoints: %w", err)
	}
	defer rows.Close()

	return s.scanCheckpoints(rows)
}

// Resolve marks a checkpoint as resolved.
func (s *CheckpointStore) Resolve(id, resolvedBy, result, status string) error {
	now := TimeToString(time.Now())

	_, err := s.db.Exec(
		`UPDATE checkpoints SET status = ?, resolved_at = ?, resolved_by = ?, result = ?
		 WHERE id = ?`,
		status, now, resolvedBy, result, id)
	if err != nil {
		return fmt.Errorf("resolve checkpoint: %w", err)
	}
	return nil
}

// --- Scan helpers ---

func (s *CheckpointStore) scanCheckpoint(row *sql.Row) (*CheckpointRow, error) {
	var cp CheckpointRow
	var config, linkedActionIDs string
	var actionID, resolvedAt, resolvedBy, result, expiresAt sql.NullString

	err := row.Scan(&cp.ID, &cp.Name, &cp.Description, &cp.Status, &cp.Type,
		&actionID, &config, &cp.CreatedAt, &resolvedAt, &resolvedBy,
		&result, &expiresAt, &linkedActionIDs)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("checkpoint not found")
		}
		return nil, fmt.Errorf("scan checkpoint: %w", err)
	}

	cp.Config = json.RawMessage(config)
	cp.LinkedActionIDs = json.RawMessage(linkedActionIDs)
	if actionID.Valid {
		cp.ActionID = &actionID.String
	}
	if resolvedAt.Valid {
		cp.ResolvedAt = &resolvedAt.String
	}
	if resolvedBy.Valid {
		cp.ResolvedBy = &resolvedBy.String
	}
	if result.Valid {
		cp.Result = &result.String
	}
	if expiresAt.Valid {
		cp.ExpiresAt = &expiresAt.String
	}
	return &cp, nil
}

func (s *CheckpointStore) scanCheckpoints(rows *sql.Rows) ([]CheckpointRow, error) {
	var results []CheckpointRow

	for rows.Next() {
		var cp CheckpointRow
		var config, linkedActionIDs string
		var actionID, resolvedAt, resolvedBy, result, expiresAt sql.NullString

		if err := rows.Scan(&cp.ID, &cp.Name, &cp.Description, &cp.Status, &cp.Type,
			&actionID, &config, &cp.CreatedAt, &resolvedAt, &resolvedBy,
			&result, &expiresAt, &linkedActionIDs); err != nil {
			return nil, fmt.Errorf("scan checkpoint row: %w", err)
		}

		cp.Config = json.RawMessage(config)
		cp.LinkedActionIDs = json.RawMessage(linkedActionIDs)
		if actionID.Valid {
			cp.ActionID = &actionID.String
		}
		if resolvedAt.Valid {
			cp.ResolvedAt = &resolvedAt.String
		}
		if resolvedBy.Valid {
			cp.ResolvedBy = &resolvedBy.String
		}
		if result.Valid {
			cp.Result = &result.String
		}
		if expiresAt.Valid {
			cp.ExpiresAt = &expiresAt.String
		}
		results = append(results, cp)
	}
	return results, rows.Err()
}
