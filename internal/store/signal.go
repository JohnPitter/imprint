package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// SignalRow represents a row in the signals table.
type SignalRow struct {
	ID        string          `json:"id"`
	FromAgent string          `json:"from"`
	ToAgent   string          `json:"to"`
	Content   string          `json:"content"`
	Type      string          `json:"type"`
	ThreadID  *string         `json:"threadId"`
	ParentID  *string         `json:"parentId"`
	CreatedAt string          `json:"createdAt"`
	ExpiresAt *string         `json:"expiresAt"`
	ReadBy    json.RawMessage `json:"readBy"`
}

// SignalStore provides CRUD operations for the signals table.
type SignalStore struct {
	db *DB
}

// NewSignalStore creates a new SignalStore.
func NewSignalStore(db *DB) *SignalStore {
	return &SignalStore{db: db}
}

// Create inserts a signal.
func (s *SignalStore) Create(sig *SignalRow) error {
	if sig.CreatedAt == "" {
		sig.CreatedAt = TimeToString(time.Now())
	}
	if sig.Type == "" {
		sig.Type = "info"
	}
	if len(sig.ReadBy) == 0 {
		sig.ReadBy = json.RawMessage("[]")
	}

	_, err := s.db.Exec(
		`INSERT INTO signals (id, from_agent, to_agent, content, type, thread_id, parent_id, created_at, expires_at, read_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sig.ID, sig.FromAgent, sig.ToAgent, sig.Content, sig.Type,
		NullString(sig.ThreadID), NullString(sig.ParentID),
		sig.CreatedAt, NullString(sig.ExpiresAt), string(sig.ReadBy),
	)
	if err != nil {
		return fmt.Errorf("insert signal: %w", err)
	}
	return nil
}

// Send is an alias for Create.
func (s *SignalStore) Send(sig *SignalRow) error {
	return s.Create(sig)
}

// List returns signals for a given agent, ordered by most recent first.
func (s *SignalStore) List(agentID string, limit int) ([]SignalRow, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.Query(
		`SELECT id, from_agent, to_agent, content, type,
		        thread_id, parent_id, created_at, expires_at,
		        COALESCE(read_by,'[]')
		 FROM signals
		 WHERE to_agent = ?
		 ORDER BY created_at DESC LIMIT ?`, agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("list signals: %w", err)
	}
	defer rows.Close()

	return s.scanSignals(rows)
}

// MarkRead adds agentID to the read_by array of a signal.
func (s *SignalStore) MarkRead(id, agentID string) error {
	row := s.db.QueryRow(
		`SELECT COALESCE(read_by,'[]') FROM signals WHERE id = ?`, id)

	var readByStr string
	if err := row.Scan(&readByStr); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("signal not found")
		}
		return fmt.Errorf("scan signal read_by: %w", err)
	}

	var readBy []string
	if err := json.Unmarshal([]byte(readByStr), &readBy); err != nil {
		readBy = []string{}
	}

	// Check if already read by this agent.
	for _, a := range readBy {
		if a == agentID {
			return nil
		}
	}

	readBy = append(readBy, agentID)
	updated := MarshalJSON(readBy)

	_, err := s.db.Exec(`UPDATE signals SET read_by = ? WHERE id = ?`, updated, id)
	if err != nil {
		return fmt.Errorf("mark signal read: %w", err)
	}
	return nil
}

// --- Scan helpers ---

func (s *SignalStore) scanSignal(row *sql.Row) (*SignalRow, error) {
	var sig SignalRow
	var readBy string
	var threadID, parentID, expiresAt sql.NullString

	err := row.Scan(&sig.ID, &sig.FromAgent, &sig.ToAgent, &sig.Content, &sig.Type,
		&threadID, &parentID, &sig.CreatedAt, &expiresAt, &readBy)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("signal not found")
		}
		return nil, fmt.Errorf("scan signal: %w", err)
	}

	sig.ReadBy = json.RawMessage(readBy)
	if threadID.Valid {
		sig.ThreadID = &threadID.String
	}
	if parentID.Valid {
		sig.ParentID = &parentID.String
	}
	if expiresAt.Valid {
		sig.ExpiresAt = &expiresAt.String
	}
	return &sig, nil
}

func (s *SignalStore) scanSignals(rows *sql.Rows) ([]SignalRow, error) {
	var result []SignalRow

	for rows.Next() {
		var sig SignalRow
		var readBy string
		var threadID, parentID, expiresAt sql.NullString

		if err := rows.Scan(&sig.ID, &sig.FromAgent, &sig.ToAgent, &sig.Content, &sig.Type,
			&threadID, &parentID, &sig.CreatedAt, &expiresAt, &readBy); err != nil {
			return nil, fmt.Errorf("scan signal row: %w", err)
		}

		sig.ReadBy = json.RawMessage(readBy)
		if threadID.Valid {
			sig.ThreadID = &threadID.String
		}
		if parentID.Valid {
			sig.ParentID = &parentID.String
		}
		if expiresAt.Valid {
			sig.ExpiresAt = &expiresAt.String
		}
		result = append(result, sig)
	}
	return result, rows.Err()
}
