package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// SentinelRow represents a row in the sentinels table.
type SentinelRow struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Type            string          `json:"type"`
	Status          string          `json:"status"`
	Config          json.RawMessage `json:"config"`
	Result          *string         `json:"result"`
	CreatedAt       string          `json:"createdAt"`
	TriggeredAt     *string         `json:"triggeredAt"`
	ExpiresAt       *string         `json:"expiresAt"`
	LinkedActionIDs json.RawMessage `json:"linkedActionIds"`
	EscalatedAt     *string         `json:"escalatedAt"`
}

// SentinelStore provides CRUD operations for the sentinels table.
type SentinelStore struct {
	db *DB
}

// NewSentinelStore creates a new SentinelStore.
func NewSentinelStore(db *DB) *SentinelStore {
	return &SentinelStore{db: db}
}

// Create inserts a sentinel.
func (s *SentinelStore) Create(sen *SentinelRow) error {
	if sen.CreatedAt == "" {
		sen.CreatedAt = TimeToString(time.Now())
	}
	if sen.Status == "" {
		sen.Status = "watching"
	}
	if len(sen.Config) == 0 {
		sen.Config = json.RawMessage("{}")
	}
	if len(sen.LinkedActionIDs) == 0 {
		sen.LinkedActionIDs = json.RawMessage("[]")
	}

	_, err := s.db.Exec(
		`INSERT INTO sentinels (id, name, type, status, config, result,
		    created_at, triggered_at, expires_at, linked_action_ids, escalated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sen.ID, sen.Name, sen.Type, sen.Status, string(sen.Config),
		NullString(sen.Result), sen.CreatedAt,
		NullString(sen.TriggeredAt), NullString(sen.ExpiresAt),
		string(sen.LinkedActionIDs), NullString(sen.EscalatedAt),
	)
	if err != nil {
		return fmt.Errorf("insert sentinel: %w", err)
	}
	return nil
}

// GetByID retrieves a sentinel by ID.
func (s *SentinelStore) GetByID(id string) (*SentinelRow, error) {
	row := s.db.QueryRow(
		`SELECT id, name, type, status, COALESCE(config,'{}'), result,
		        created_at, triggered_at, expires_at,
		        COALESCE(linked_action_ids,'[]'), escalated_at
		 FROM sentinels WHERE id = ?`, id)
	return s.scanSentinel(row)
}

// List returns sentinels with optional status filter, ordered by most recent first.
func (s *SentinelStore) List(status string, limit int) ([]SentinelRow, error) {
	if limit <= 0 {
		limit = 100
	}

	var rows *sql.Rows
	var err error

	if status != "" {
		rows, err = s.db.Query(
			`SELECT id, name, type, status, COALESCE(config,'{}'), result,
			        created_at, triggered_at, expires_at,
			        COALESCE(linked_action_ids,'[]'), escalated_at
			 FROM sentinels WHERE status = ?
			 ORDER BY created_at DESC LIMIT ?`, status, limit)
	} else {
		rows, err = s.db.Query(
			`SELECT id, name, type, status, COALESCE(config,'{}'), result,
			        created_at, triggered_at, expires_at,
			        COALESCE(linked_action_ids,'[]'), escalated_at
			 FROM sentinels
			 ORDER BY created_at DESC LIMIT ?`, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list sentinels: %w", err)
	}
	defer rows.Close()

	return s.scanSentinels(rows)
}

// Trigger marks a sentinel as triggered with a result.
func (s *SentinelStore) Trigger(id, result string) error {
	now := TimeToString(time.Now())

	_, err := s.db.Exec(
		`UPDATE sentinels SET status = 'triggered', triggered_at = ?, result = ?
		 WHERE id = ?`,
		now, result, id)
	if err != nil {
		return fmt.Errorf("trigger sentinel: %w", err)
	}
	return nil
}

// Cancel marks a sentinel as cancelled.
func (s *SentinelStore) Cancel(id string) error {
	_, err := s.db.Exec(
		`UPDATE sentinels SET status = 'cancelled' WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("cancel sentinel: %w", err)
	}
	return nil
}

// Check retrieves a sentinel by ID (alias for GetByID for semantic clarity).
func (s *SentinelStore) Check(id string) (*SentinelRow, error) {
	return s.GetByID(id)
}

// --- Scan helpers ---

func (s *SentinelStore) scanSentinel(row *sql.Row) (*SentinelRow, error) {
	var sen SentinelRow
	var config, linkedActionIDs string
	var result, triggeredAt, expiresAt, escalatedAt sql.NullString

	err := row.Scan(&sen.ID, &sen.Name, &sen.Type, &sen.Status, &config, &result,
		&sen.CreatedAt, &triggeredAt, &expiresAt, &linkedActionIDs, &escalatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("sentinel not found")
		}
		return nil, fmt.Errorf("scan sentinel: %w", err)
	}

	sen.Config = json.RawMessage(config)
	sen.LinkedActionIDs = json.RawMessage(linkedActionIDs)
	if result.Valid {
		sen.Result = &result.String
	}
	if triggeredAt.Valid {
		sen.TriggeredAt = &triggeredAt.String
	}
	if expiresAt.Valid {
		sen.ExpiresAt = &expiresAt.String
	}
	if escalatedAt.Valid {
		sen.EscalatedAt = &escalatedAt.String
	}
	return &sen, nil
}

func (s *SentinelStore) scanSentinels(rows *sql.Rows) ([]SentinelRow, error) {
	var results []SentinelRow

	for rows.Next() {
		var sen SentinelRow
		var config, linkedActionIDs string
		var result, triggeredAt, expiresAt, escalatedAt sql.NullString

		if err := rows.Scan(&sen.ID, &sen.Name, &sen.Type, &sen.Status, &config, &result,
			&sen.CreatedAt, &triggeredAt, &expiresAt, &linkedActionIDs, &escalatedAt); err != nil {
			return nil, fmt.Errorf("scan sentinel row: %w", err)
		}

		sen.Config = json.RawMessage(config)
		sen.LinkedActionIDs = json.RawMessage(linkedActionIDs)
		if result.Valid {
			sen.Result = &result.String
		}
		if triggeredAt.Valid {
			sen.TriggeredAt = &triggeredAt.String
		}
		if expiresAt.Valid {
			sen.ExpiresAt = &expiresAt.String
		}
		if escalatedAt.Valid {
			sen.EscalatedAt = &escalatedAt.String
		}
		results = append(results, sen)
	}
	return results, rows.Err()
}
