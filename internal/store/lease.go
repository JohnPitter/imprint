package store

import (
	"database/sql"
	"fmt"
	"time"
)

// LeaseRow represents a row in the leases table.
type LeaseRow struct {
	ID         string  `json:"id"`
	ActionID   string  `json:"actionId"`
	AgentID    string  `json:"agentId"`
	AcquiredAt string  `json:"acquiredAt"`
	ExpiresAt  string  `json:"expiresAt"`
	Status     string  `json:"status"`
	Result     *string `json:"result"`
}

// LeaseStore provides CRUD operations for the leases table.
type LeaseStore struct {
	db *DB
}

// NewLeaseStore creates a new LeaseStore.
func NewLeaseStore(db *DB) *LeaseStore {
	return &LeaseStore{db: db}
}

// Acquire inserts a new lease.
func (s *LeaseStore) Acquire(row *LeaseRow) error {
	if row.AcquiredAt == "" {
		row.AcquiredAt = TimeToString(time.Now())
	}
	if row.Status == "" {
		row.Status = "active"
	}

	_, err := s.db.Exec(
		`INSERT INTO leases (id, action_id, agent_id, acquired_at, expires_at, status, result)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		row.ID, row.ActionID, row.AgentID, row.AcquiredAt, row.ExpiresAt,
		row.Status, NullString(row.Result),
	)
	if err != nil {
		return fmt.Errorf("acquire lease: %w", err)
	}
	return nil
}

// Release sets a lease status to "released" and records the result.
func (s *LeaseStore) Release(id, agentID string, result *string) error {
	_, err := s.db.Exec(
		`UPDATE leases SET status = 'released', result = ?
		 WHERE id = ? AND agent_id = ?`,
		NullString(result), id, agentID,
	)
	if err != nil {
		return fmt.Errorf("release lease: %w", err)
	}
	return nil
}

// Renew extends a lease's expiration time.
func (s *LeaseStore) Renew(id, agentID string, newExpiry string) error {
	_, err := s.db.Exec(
		`UPDATE leases SET expires_at = ?
		 WHERE id = ? AND agent_id = ? AND status = 'active'`,
		newExpiry, id, agentID,
	)
	if err != nil {
		return fmt.Errorf("renew lease: %w", err)
	}
	return nil
}

// GetByActionID retrieves the latest active lease for an action.
func (s *LeaseStore) GetByActionID(actionID string) (*LeaseRow, error) {
	row := s.db.QueryRow(
		`SELECT id, action_id, agent_id, acquired_at, expires_at, status, result
		 FROM leases
		 WHERE action_id = ? AND status = 'active'
		 ORDER BY acquired_at DESC LIMIT 1`, actionID)
	return s.scanLease(row)
}

// IsLocked checks if an active, non-expired lease exists for an action.
func (s *LeaseStore) IsLocked(actionID string) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM leases
		 WHERE action_id = ? AND status = 'active' AND expires_at > ?`,
		actionID, TimeToString(time.Now()),
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check lease lock: %w", err)
	}
	return count > 0, nil
}

// --- Scan helpers ---

func (s *LeaseStore) scanLease(row *sql.Row) (*LeaseRow, error) {
	var l LeaseRow
	var result sql.NullString

	err := row.Scan(&l.ID, &l.ActionID, &l.AgentID, &l.AcquiredAt, &l.ExpiresAt,
		&l.Status, &result)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("lease not found")
		}
		return nil, fmt.Errorf("scan lease: %w", err)
	}

	if result.Valid {
		l.Result = &result.String
	}
	return &l, nil
}
