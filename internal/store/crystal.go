package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// CrystalRow represents a row in the crystals table.
type CrystalRow struct {
	ID              string          `json:"id"`
	Narrative       string          `json:"narrative"`
	KeyOutcomes     json.RawMessage `json:"keyOutcomes"`
	FilesAffected   json.RawMessage `json:"filesAffected"`
	Lessons         json.RawMessage `json:"lessons"`
	SourceActionIDs json.RawMessage `json:"sourceActionIds"`
	SessionID       *string         `json:"sessionId"`
	Project         *string         `json:"project"`
	CreatedAt       string          `json:"createdAt"`
}

// CrystalStore provides CRUD operations for the crystals table.
type CrystalStore struct {
	db *DB
}

// NewCrystalStore creates a new CrystalStore.
func NewCrystalStore(db *DB) *CrystalStore {
	return &CrystalStore{db: db}
}

// Create inserts a new crystal.
func (s *CrystalStore) Create(row *CrystalRow) error {
	if row.CreatedAt == "" {
		row.CreatedAt = TimeToString(time.Now())
	}
	row.KeyOutcomes = defaultJSON(row.KeyOutcomes)
	row.FilesAffected = defaultJSON(row.FilesAffected)
	row.Lessons = defaultJSON(row.Lessons)
	row.SourceActionIDs = defaultJSON(row.SourceActionIDs)

	_, err := s.db.Exec(`
		INSERT INTO crystals (id, narrative, key_outcomes, files_affected, lessons, source_action_ids, session_id, project, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.ID, row.Narrative,
		string(row.KeyOutcomes), string(row.FilesAffected), string(row.Lessons), string(row.SourceActionIDs),
		NullString(row.SessionID), NullString(row.Project),
		row.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert crystal: %w", err)
	}
	return nil
}

// GetByID retrieves a crystal by ID.
func (s *CrystalStore) GetByID(id string) (*CrystalRow, error) {
	row := s.db.QueryRow(`
		SELECT id, narrative,
			COALESCE(key_outcomes, '[]'), COALESCE(files_affected, '[]'),
			COALESCE(lessons, '[]'), COALESCE(source_action_ids, '[]'),
			session_id, project, created_at
		FROM crystals WHERE id = ?`, id)
	return s.scanRow(row)
}

// List returns crystals filtered by project, ordered by created_at DESC.
func (s *CrystalStore) List(project string, limit int) ([]CrystalRow, error) {
	if limit <= 0 {
		limit = 100
	}

	var rows *sql.Rows
	var err error

	if project != "" {
		rows, err = s.db.Query(`
			SELECT id, narrative,
				COALESCE(key_outcomes, '[]'), COALESCE(files_affected, '[]'),
				COALESCE(lessons, '[]'), COALESCE(source_action_ids, '[]'),
				session_id, project, created_at
			FROM crystals
			WHERE project = ?
			ORDER BY created_at DESC LIMIT ?`, project, limit)
	} else {
		rows, err = s.db.Query(`
			SELECT id, narrative,
				COALESCE(key_outcomes, '[]'), COALESCE(files_affected, '[]'),
				COALESCE(lessons, '[]'), COALESCE(source_action_ids, '[]'),
				session_id, project, created_at
			FROM crystals
			ORDER BY created_at DESC LIMIT ?`, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list crystals: %w", err)
	}
	defer rows.Close()

	return s.scanRows(rows)
}

// AutoCrystallize creates a crystal from all completed actions in the given session.
func (s *CrystalStore) AutoCrystallize(sessionID string) (*CrystalRow, error) {
	// Gather all done actions for this session's project.
	var actionIDs []string
	rows, err := s.db.Query(`
		SELECT a.id FROM actions a
		JOIN sessions s ON a.project = s.project
		WHERE s.id = ? AND a.status = 'done' AND a.crystallized = 0
		ORDER BY a.updated_at ASC`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("auto-crystallize query actions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("auto-crystallize scan action id: %w", err)
		}
		actionIDs = append(actionIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auto-crystallize iterate actions: %w", err)
	}

	if len(actionIDs) == 0 {
		return nil, fmt.Errorf("no completed actions to crystallize for session %s", sessionID)
	}

	sourceIDs, _ := json.Marshal(actionIDs)

	crystal := &CrystalRow{
		ID:              generateID(),
		Narrative:       fmt.Sprintf("Auto-crystallized from %d completed actions in session %s", len(actionIDs), sessionID),
		SourceActionIDs: json.RawMessage(sourceIDs),
		SessionID:       &sessionID,
		CreatedAt:       TimeToString(time.Now()),
	}

	// Look up the project from the session.
	var project sql.NullString
	if err := s.db.QueryRow(`SELECT project FROM sessions WHERE id = ?`, sessionID).Scan(&project); err == nil && project.Valid {
		crystal.Project = &project.String
	}

	if err := s.Create(crystal); err != nil {
		return nil, err
	}

	// Mark actions as crystallized.
	for _, aid := range actionIDs {
		if _, err := s.db.Exec(`UPDATE actions SET crystallized = 1 WHERE id = ?`, aid); err != nil {
			return nil, fmt.Errorf("mark action crystallized: %w", err)
		}
	}

	return crystal, nil
}

// generateID creates a simple unique ID. Defers to the caller for custom IDs.
func generateID() string {
	return fmt.Sprintf("cry_%d", time.Now().UnixNano())
}

// --- Scan helpers ---

func (s *CrystalStore) scanRow(row *sql.Row) (*CrystalRow, error) {
	var c CrystalRow
	var keyOutcomes, filesAffected, lessons, sourceActionIDs string
	var sessionID, project sql.NullString

	err := row.Scan(&c.ID, &c.Narrative,
		&keyOutcomes, &filesAffected, &lessons, &sourceActionIDs,
		&sessionID, &project, &c.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("crystal not found")
		}
		return nil, fmt.Errorf("scan crystal: %w", err)
	}

	c.KeyOutcomes = json.RawMessage(keyOutcomes)
	c.FilesAffected = json.RawMessage(filesAffected)
	c.Lessons = json.RawMessage(lessons)
	c.SourceActionIDs = json.RawMessage(sourceActionIDs)
	if sessionID.Valid {
		c.SessionID = &sessionID.String
	}
	if project.Valid {
		c.Project = &project.String
	}
	return &c, nil
}

func (s *CrystalStore) scanRows(rows *sql.Rows) ([]CrystalRow, error) {
	var result []CrystalRow

	for rows.Next() {
		var c CrystalRow
		var keyOutcomes, filesAffected, lessons, sourceActionIDs string
		var sessionID, project sql.NullString

		if err := rows.Scan(&c.ID, &c.Narrative,
			&keyOutcomes, &filesAffected, &lessons, &sourceActionIDs,
			&sessionID, &project, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan crystal row: %w", err)
		}

		c.KeyOutcomes = json.RawMessage(keyOutcomes)
		c.FilesAffected = json.RawMessage(filesAffected)
		c.Lessons = json.RawMessage(lessons)
		c.SourceActionIDs = json.RawMessage(sourceActionIDs)
		if sessionID.Valid {
			c.SessionID = &sessionID.String
		}
		if project.Valid {
			c.Project = &project.String
		}
		result = append(result, c)
	}
	return result, rows.Err()
}
