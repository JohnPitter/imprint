package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ActionRow represents a row in the actions table.
type ActionRow struct {
	ID           string          `json:"id"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	Status       string          `json:"status"`
	Priority     int             `json:"priority"`
	Assignee     *string         `json:"assignee"`
	Project      *string         `json:"project"`
	Tags         json.RawMessage `json:"tags"`
	ParentID     *string         `json:"parentId"`
	SketchID     *string         `json:"sketchId"`
	Crystallized int             `json:"crystallized"`
	CreatedAt    string          `json:"createdAt"`
	UpdatedAt    string          `json:"updatedAt"`
}

// ActionEdgeRow represents a row in the action_edges table.
type ActionEdgeRow struct {
	ID        string `json:"id"`
	SourceID  string `json:"sourceId"`
	TargetID  string `json:"targetId"`
	Type      string `json:"type"`
	CreatedAt string `json:"createdAt"`
}

// ActionStore provides CRUD operations for the actions and action_edges tables.
type ActionStore struct {
	db *DB
}

// NewActionStore creates a new ActionStore.
func NewActionStore(db *DB) *ActionStore {
	return &ActionStore{db: db}
}

// --- Actions ---

// Create inserts an action.
func (s *ActionStore) Create(row *ActionRow) error {
	now := TimeToString(time.Now())
	if row.CreatedAt == "" {
		row.CreatedAt = now
	}
	if row.UpdatedAt == "" {
		row.UpdatedAt = now
	}
	if row.Status == "" {
		row.Status = "pending"
	}
	if row.Priority == 0 {
		row.Priority = 5
	}
	if len(row.Tags) == 0 {
		row.Tags = json.RawMessage("[]")
	}

	_, err := s.db.Exec(
		`INSERT INTO actions (id, title, description, status, priority, assignee, project,
		    tags, parent_id, sketch_id, crystallized, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.ID, row.Title, row.Description, row.Status, row.Priority,
		NullString(row.Assignee), NullString(row.Project),
		string(row.Tags),
		NullString(row.ParentID), NullString(row.SketchID),
		row.Crystallized, row.CreatedAt, row.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert action: %w", err)
	}
	return nil
}

// GetByID retrieves an action by ID.
func (s *ActionStore) GetByID(id string) (*ActionRow, error) {
	row := s.db.QueryRow(
		`SELECT id, title, description, status, priority, assignee, project,
		        COALESCE(tags,'[]'), parent_id, sketch_id, crystallized, created_at, updated_at
		 FROM actions WHERE id = ?`, id)
	return s.scanAction(row)
}

// List returns actions with optional status and project filters.
func (s *ActionStore) List(status, project string, limit, offset int) ([]ActionRow, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `SELECT id, title, description, status, priority, assignee, project,
	                  COALESCE(tags,'[]'), parent_id, sketch_id, crystallized, created_at, updated_at
	           FROM actions WHERE 1=1`
	args := []any{}

	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	if project != "" {
		query += ` AND project = ?`
		args = append(args, project)
	}

	query += ` ORDER BY priority DESC, created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list actions: %w", err)
	}
	defer rows.Close()

	return s.scanActions(rows)
}

// Update updates an existing action.
func (s *ActionStore) Update(row *ActionRow) error {
	row.UpdatedAt = TimeToString(time.Now())
	if len(row.Tags) == 0 {
		row.Tags = json.RawMessage("[]")
	}

	_, err := s.db.Exec(
		`UPDATE actions SET title = ?, description = ?, status = ?, priority = ?,
		    assignee = ?, project = ?, tags = ?, parent_id = ?, sketch_id = ?,
		    crystallized = ?, updated_at = ?
		 WHERE id = ?`,
		row.Title, row.Description, row.Status, row.Priority,
		NullString(row.Assignee), NullString(row.Project),
		string(row.Tags),
		NullString(row.ParentID), NullString(row.SketchID),
		row.Crystallized, row.UpdatedAt, row.ID,
	)
	if err != nil {
		return fmt.Errorf("update action: %w", err)
	}
	return nil
}

// Delete removes an action by ID.
func (s *ActionStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM actions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete action: %w", err)
	}
	return nil
}

// --- Action Edges ---

// CreateEdge inserts an action edge.
func (s *ActionStore) CreateEdge(edge *ActionEdgeRow) error {
	if edge.CreatedAt == "" {
		edge.CreatedAt = TimeToString(time.Now())
	}

	_, err := s.db.Exec(
		`INSERT INTO action_edges (id, source_id, target_id, type, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		edge.ID, edge.SourceID, edge.TargetID, edge.Type, edge.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert action edge: %w", err)
	}
	return nil
}

// GetFrontier returns pending actions that have no unsatisfied blocking edges.
func (s *ActionStore) GetFrontier() ([]ActionRow, error) {
	rows, err := s.db.Query(`
		SELECT a.id, a.title, a.description, a.status, a.priority, a.assignee, a.project,
		       COALESCE(a.tags,'[]'), a.parent_id, a.sketch_id, a.crystallized, a.created_at, a.updated_at
		FROM actions a
		WHERE a.status = 'pending'
		AND NOT EXISTS (
		    SELECT 1 FROM action_edges ae
		    JOIN actions blocker ON ae.source_id = blocker.id
		    WHERE ae.target_id = a.id AND ae.type = 'blocks' AND blocker.status != 'done'
		)
		ORDER BY a.priority DESC`)
	if err != nil {
		return nil, fmt.Errorf("get frontier: %w", err)
	}
	defer rows.Close()

	return s.scanActions(rows)
}

// GetNext returns the highest priority action from the frontier.
func (s *ActionStore) GetNext() (*ActionRow, error) {
	row := s.db.QueryRow(`
		SELECT a.id, a.title, a.description, a.status, a.priority, a.assignee, a.project,
		       COALESCE(a.tags,'[]'), a.parent_id, a.sketch_id, a.crystallized, a.created_at, a.updated_at
		FROM actions a
		WHERE a.status = 'pending'
		AND NOT EXISTS (
		    SELECT 1 FROM action_edges ae
		    JOIN actions blocker ON ae.source_id = blocker.id
		    WHERE ae.target_id = a.id AND ae.type = 'blocks' AND blocker.status != 'done'
		)
		ORDER BY a.priority DESC
		LIMIT 1`)
	return s.scanAction(row)
}

// --- Scan helpers ---

func (s *ActionStore) scanAction(row *sql.Row) (*ActionRow, error) {
	var a ActionRow
	var tags string
	var assignee, project, parentID, sketchID sql.NullString

	err := row.Scan(&a.ID, &a.Title, &a.Description, &a.Status, &a.Priority,
		&assignee, &project, &tags, &parentID, &sketchID,
		&a.Crystallized, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("action not found")
		}
		return nil, fmt.Errorf("scan action: %w", err)
	}

	a.Tags = json.RawMessage(tags)
	if assignee.Valid {
		a.Assignee = &assignee.String
	}
	if project.Valid {
		a.Project = &project.String
	}
	if parentID.Valid {
		a.ParentID = &parentID.String
	}
	if sketchID.Valid {
		a.SketchID = &sketchID.String
	}
	return &a, nil
}

func (s *ActionStore) scanActions(rows *sql.Rows) ([]ActionRow, error) {
	var result []ActionRow

	for rows.Next() {
		var a ActionRow
		var tags string
		var assignee, project, parentID, sketchID sql.NullString

		if err := rows.Scan(&a.ID, &a.Title, &a.Description, &a.Status, &a.Priority,
			&assignee, &project, &tags, &parentID, &sketchID,
			&a.Crystallized, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan action row: %w", err)
		}

		a.Tags = json.RawMessage(tags)
		if assignee.Valid {
			a.Assignee = &assignee.String
		}
		if project.Valid {
			a.Project = &project.String
		}
		if parentID.Valid {
			a.ParentID = &parentID.String
		}
		if sketchID.Valid {
			a.SketchID = &sketchID.String
		}
		result = append(result, a)
	}
	return result, rows.Err()
}
