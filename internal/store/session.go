package store

import (
	"database/sql"
	"fmt"
	"time"

	"imprint/internal/types"
)

// SessionRow represents the sessions table row with all DB columns.
type SessionRow struct {
	ID               string
	Project          string
	Cwd              string
	StartedAt        time.Time
	EndedAt          *time.Time
	Status           types.SessionStatus
	ObservationCount int
	Model            *string
	Tags             []string
}

// SessionStore handles CRUD operations for sessions.
type SessionStore struct {
	db *DB
}

// NewSessionStore creates a new SessionStore backed by the given DB.
func NewSessionStore(db *DB) *SessionStore {
	return &SessionStore{db: db}
}

// Create inserts a new session.
func (s *SessionStore) Create(row *SessionRow) error {
	_, err := s.db.Exec(
		`INSERT INTO sessions (id, project, cwd, started_at, ended_at, status, observation_count, model, tags)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.ID,
		row.Project,
		row.Cwd,
		TimeToString(row.StartedAt),
		NullTime(row.EndedAt),
		string(row.Status),
		row.ObservationCount,
		NullString(row.Model),
		MarshalJSON(row.Tags),
	)
	if err != nil {
		return fmt.Errorf("session create: %w", err)
	}
	return nil
}

// GetByID retrieves a session by ID.
func (s *SessionStore) GetByID(id string) (*SessionRow, error) {
	row := s.db.QueryRow(
		`SELECT id, project, cwd, started_at, ended_at, status, observation_count, model, tags
		 FROM sessions WHERE id = ?`, id,
	)
	return scanSession(row)
}

// List returns sessions, optionally filtered by project, ordered by started_at DESC.
// Pass an empty project string to list all sessions.
// Supports limit and offset for pagination.
func (s *SessionStore) List(project string, limit, offset int) ([]SessionRow, error) {
	var rows *sql.Rows
	var err error

	if project != "" {
		rows, err = s.db.Query(
			`SELECT id, project, cwd, started_at, ended_at, status, observation_count, model, tags
			 FROM sessions WHERE project = ?
			 ORDER BY started_at DESC LIMIT ? OFFSET ?`,
			project, limit, offset,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, project, cwd, started_at, ended_at, status, observation_count, model, tags
			 FROM sessions ORDER BY started_at DESC LIMIT ? OFFSET ?`,
			limit, offset,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("session list: %w", err)
	}
	defer rows.Close()

	return scanSessions(rows)
}

// End marks a session as completed and sets ended_at to the current time.
func (s *SessionStore) End(id string) error {
	now := TimeToString(time.Now())
	res, err := s.db.Exec(
		`UPDATE sessions SET status = ?, ended_at = ? WHERE id = ?`,
		string(types.SessionCompleted), now, id,
	)
	if err != nil {
		return fmt.Errorf("session end: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("session end: session %s not found", id)
	}
	return nil
}

// IncrementObservationCount atomically increments observation_count by 1.
func (s *SessionStore) IncrementObservationCount(id string) error {
	res, err := s.db.Exec(
		`UPDATE sessions SET observation_count = observation_count + 1 WHERE id = ?`, id,
	)
	if err != nil {
		return fmt.Errorf("session increment observation count: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("session increment observation count: session %s not found", id)
	}
	return nil
}

// Count returns the total session count, optionally filtered by project.
// Pass an empty project string to count all sessions.
func (s *SessionStore) Count(project string) (int, error) {
	var count int
	var err error

	if project != "" {
		err = s.db.QueryRow(
			`SELECT COUNT(*) FROM sessions WHERE project = ?`, project,
		).Scan(&count)
	} else {
		err = s.db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&count)
	}
	if err != nil {
		return 0, fmt.Errorf("session count: %w", err)
	}
	return count, nil
}

// GetActive returns all sessions with status "active".
func (s *SessionStore) GetActive() ([]SessionRow, error) {
	rows, err := s.db.Query(
		`SELECT id, project, cwd, started_at, ended_at, status, observation_count, model, tags
		 FROM sessions WHERE status = ?
		 ORDER BY started_at DESC`,
		string(types.SessionActive),
	)
	if err != nil {
		return nil, fmt.Errorf("session get active: %w", err)
	}
	defer rows.Close()

	return scanSessions(rows)
}

// scanSession scans a single row into a SessionRow.
func scanSession(row *sql.Row) (*SessionRow, error) {
	var (
		sess      SessionRow
		startedAt string
		endedAt   sql.NullString
		status    string
		model     sql.NullString
		tags      string
	)

	err := row.Scan(
		&sess.ID,
		&sess.Project,
		&sess.Cwd,
		&startedAt,
		&endedAt,
		&status,
		&sess.ObservationCount,
		&model,
		&tags,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("session scan: %w", err)
	}

	sess.StartedAt = ParseTime(startedAt)
	sess.EndedAt = ParseNullTime(endedAt)
	sess.Status = types.SessionStatus(status)
	if model.Valid {
		sess.Model = &model.String
	}
	sess.Tags = UnmarshalStringSlice(tags)

	return &sess, nil
}

// scanSessions scans multiple rows into a slice of SessionRow.
func scanSessions(rows *sql.Rows) ([]SessionRow, error) {
	var sessions []SessionRow

	for rows.Next() {
		var (
			sess      SessionRow
			startedAt string
			endedAt   sql.NullString
			status    string
			model     sql.NullString
			tags      string
		)

		err := rows.Scan(
			&sess.ID,
			&sess.Project,
			&sess.Cwd,
			&startedAt,
			&endedAt,
			&status,
			&sess.ObservationCount,
			&model,
			&tags,
		)
		if err != nil {
			return nil, fmt.Errorf("session scan row: %w", err)
		}

		sess.StartedAt = ParseTime(startedAt)
		sess.EndedAt = ParseNullTime(endedAt)
		sess.Status = types.SessionStatus(status)
		if model.Valid {
			sess.Model = &model.String
		}
		sess.Tags = UnmarshalStringSlice(tags)

		sessions = append(sessions, sess)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("session scan rows: %w", err)
	}

	return sessions, nil
}
