package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// AuditRow represents a row in the audit_log table.
type AuditRow struct {
	ID         string          `json:"id"`
	Action     string          `json:"action"`
	EntityID   string          `json:"entityId"`
	EntityType string          `json:"entityType"`
	AgentID    *string         `json:"agentId"`
	Meta       json.RawMessage `json:"meta"`
	Timestamp  string          `json:"timestamp"`
}

// AuditStore provides CRUD operations for the audit_log table.
type AuditStore struct {
	db *DB
}

// NewAuditStore creates a new AuditStore.
func NewAuditStore(db *DB) *AuditStore {
	return &AuditStore{db: db}
}

// Create inserts a new audit log entry.
func (s *AuditStore) Create(row *AuditRow) error {
	if row.Timestamp == "" {
		row.Timestamp = TimeToString(time.Now())
	}
	if len(row.Meta) == 0 {
		row.Meta = json.RawMessage("{}")
	}

	_, err := s.db.Exec(`
		INSERT INTO audit_log (id, action, entity_id, entity_type, agent_id, meta, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		row.ID, row.Action, row.EntityID, row.EntityType,
		NullString(row.AgentID), string(row.Meta), row.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

// List returns audit log entries filtered by action, ordered by timestamp DESC.
// Pass empty string for action to list all entries.
func (s *AuditStore) List(action string, limit, offset int) ([]AuditRow, error) {
	if limit <= 0 {
		limit = 100
	}

	var rows *sql.Rows
	var err error

	if action != "" {
		rows, err = s.db.Query(`
			SELECT id, action, entity_id, entity_type, agent_id,
				COALESCE(meta, '{}'), timestamp
			FROM audit_log
			WHERE action = ?
			ORDER BY timestamp DESC LIMIT ? OFFSET ?`, action, limit, offset)
	} else {
		rows, err = s.db.Query(`
			SELECT id, action, entity_id, entity_type, agent_id,
				COALESCE(meta, '{}'), timestamp
			FROM audit_log
			ORDER BY timestamp DESC LIMIT ? OFFSET ?`, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("list audit log: %w", err)
	}
	defer rows.Close()

	return s.scanRows(rows)
}

// Count returns the total number of audit log entries.
func (s *AuditStore) Count() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM audit_log`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count audit log: %w", err)
	}
	return count, nil
}

// --- Scan helpers ---

func (s *AuditStore) scanRows(rows *sql.Rows) ([]AuditRow, error) {
	var result []AuditRow

	for rows.Next() {
		var a AuditRow
		var meta string
		var agentID sql.NullString

		if err := rows.Scan(&a.ID, &a.Action, &a.EntityID, &a.EntityType, &agentID, &meta, &a.Timestamp); err != nil {
			return nil, fmt.Errorf("scan audit log row: %w", err)
		}

		a.Meta = json.RawMessage(meta)
		if agentID.Valid {
			a.AgentID = &agentID.String
		}
		result = append(result, a)
	}
	return result, rows.Err()
}
