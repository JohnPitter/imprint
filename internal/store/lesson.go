package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// LessonRow represents a row in the lessons table.
type LessonRow struct {
	ID               string          `json:"id"`
	Content          string          `json:"content"`
	Context          string          `json:"context"`
	Confidence       float64         `json:"confidence"`
	Reinforcements   int             `json:"reinforcements"`
	Source           string          `json:"source"`
	SourceIDs        json.RawMessage `json:"sourceIds"`
	Project          *string         `json:"project"`
	Tags             json.RawMessage `json:"tags"`
	CreatedAt        string          `json:"createdAt"`
	UpdatedAt        string          `json:"updatedAt"`
	LastReinforcedAt *string         `json:"lastReinforcedAt"`
	LastDecayedAt    *string         `json:"lastDecayedAt"`
	DecayRate        float64         `json:"decayRate"`
	Deleted          int             `json:"deleted"`
}

// LessonStore provides CRUD operations for the lessons table.
type LessonStore struct {
	db *DB
}

// NewLessonStore creates a new LessonStore.
func NewLessonStore(db *DB) *LessonStore {
	return &LessonStore{db: db}
}

// Create inserts a new lesson.
func (s *LessonStore) Create(row *LessonRow) error {
	now := TimeToString(time.Now())
	if row.CreatedAt == "" {
		row.CreatedAt = now
	}
	if row.UpdatedAt == "" {
		row.UpdatedAt = now
	}
	if row.Confidence == 0 {
		row.Confidence = 0.5
	}
	if row.Source == "" {
		row.Source = "manual"
	}
	if row.DecayRate == 0 {
		row.DecayRate = 0.01
	}
	row.SourceIDs = defaultJSON(row.SourceIDs)
	row.Tags = defaultJSON(row.Tags)

	_, err := s.db.Exec(`
		INSERT INTO lessons (
			id, content, context, confidence, reinforcements, source, source_ids,
			project, tags, created_at, updated_at, last_reinforced_at, last_decayed_at,
			decay_rate, deleted
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.ID, row.Content, row.Context, row.Confidence, row.Reinforcements, row.Source,
		string(row.SourceIDs), NullString(row.Project), string(row.Tags),
		row.CreatedAt, row.UpdatedAt,
		NullString(row.LastReinforcedAt), NullString(row.LastDecayedAt),
		row.DecayRate, row.Deleted,
	)
	if err != nil {
		return fmt.Errorf("insert lesson: %w", err)
	}
	return nil
}

// GetByID retrieves a lesson by ID.
func (s *LessonStore) GetByID(id string) (*LessonRow, error) {
	row := s.db.QueryRow(`
		SELECT id, content, context, confidence, reinforcements, source,
			COALESCE(source_ids, '[]'), project, COALESCE(tags, '[]'),
			created_at, updated_at, last_reinforced_at, last_decayed_at,
			decay_rate, deleted
		FROM lessons WHERE id = ?`, id)
	return s.scanRow(row)
}

// Count returns the number of non-deleted lessons, optionally filtered by project.
func (s *LessonStore) Count(project string) (int, error) {
	var (
		count int
		err   error
	)
	if project == "" {
		err = s.db.QueryRow(`SELECT COUNT(*) FROM lessons WHERE deleted = 0`).Scan(&count)
	} else {
		err = s.db.QueryRow(`SELECT COUNT(*) FROM lessons WHERE deleted = 0 AND project = ?`, project).Scan(&count)
	}
	if err != nil {
		return 0, fmt.Errorf("lesson count: %w", err)
	}
	return count, nil
}

// List returns lessons filtered by project, excluding soft-deleted, ordered by confidence DESC.
func (s *LessonStore) List(project string, limit, offset int) ([]LessonRow, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	var rows *sql.Rows
	var err error

	if project != "" {
		rows, err = s.db.Query(`
			SELECT id, content, context, confidence, reinforcements, source,
				COALESCE(source_ids, '[]'), project, COALESCE(tags, '[]'),
				created_at, updated_at, last_reinforced_at, last_decayed_at,
				decay_rate, deleted
			FROM lessons
			WHERE deleted = 0 AND project = ?
			ORDER BY confidence DESC LIMIT ? OFFSET ?`, project, limit, offset)
	} else {
		rows, err = s.db.Query(`
			SELECT id, content, context, confidence, reinforcements, source,
				COALESCE(source_ids, '[]'), project, COALESCE(tags, '[]'),
				created_at, updated_at, last_reinforced_at, last_decayed_at,
				decay_rate, deleted
			FROM lessons
			WHERE deleted = 0
			ORDER BY confidence DESC LIMIT ? OFFSET ?`, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("list lessons: %w", err)
	}
	defer rows.Close()

	return s.scanRows(rows)
}

// Search performs a LIKE search on lesson content, excluding soft-deleted.
func (s *LessonStore) Search(query string, limit int) ([]LessonRow, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.Query(`
		SELECT id, content, context, confidence, reinforcements, source,
			COALESCE(source_ids, '[]'), project, COALESCE(tags, '[]'),
			created_at, updated_at, last_reinforced_at, last_decayed_at,
			decay_rate, deleted
		FROM lessons
		WHERE deleted = 0 AND content LIKE '%' || ? || '%'
		ORDER BY confidence DESC LIMIT ?`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search lessons: %w", err)
	}
	defer rows.Close()

	return s.scanRows(rows)
}

// Strengthen increments reinforcements, boosts confidence, and updates lastReinforcedAt.
func (s *LessonStore) Strengthen(id string) error {
	now := TimeToString(time.Now())
	_, err := s.db.Exec(`
		UPDATE lessons SET
			reinforcements = reinforcements + 1,
			confidence = MIN(confidence + 0.1, 1.0),
			last_reinforced_at = ?,
			updated_at = ?
		WHERE id = ? AND deleted = 0`,
		now, now, id,
	)
	if err != nil {
		return fmt.Errorf("strengthen lesson: %w", err)
	}
	return nil
}

// --- Scan helpers ---

func populateLessonJSON(l *LessonRow, sourceIDs, tags string, project, lastReinforced, lastDecayed sql.NullString) {
	l.SourceIDs = json.RawMessage(sourceIDs)
	l.Tags = json.RawMessage(tags)
	if project.Valid {
		l.Project = &project.String
	}
	if lastReinforced.Valid {
		l.LastReinforcedAt = &lastReinforced.String
	}
	if lastDecayed.Valid {
		l.LastDecayedAt = &lastDecayed.String
	}
}

func (s *LessonStore) scanRow(row *sql.Row) (*LessonRow, error) {
	var l LessonRow
	var sourceIDs, tags string
	var project, lastReinforced, lastDecayed sql.NullString

	err := row.Scan(
		&l.ID, &l.Content, &l.Context, &l.Confidence, &l.Reinforcements, &l.Source,
		&sourceIDs, &project, &tags,
		&l.CreatedAt, &l.UpdatedAt, &lastReinforced, &lastDecayed,
		&l.DecayRate, &l.Deleted,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("lesson not found")
		}
		return nil, fmt.Errorf("scan lesson: %w", err)
	}

	populateLessonJSON(&l, sourceIDs, tags, project, lastReinforced, lastDecayed)
	return &l, nil
}

func (s *LessonStore) scanRows(rows *sql.Rows) ([]LessonRow, error) {
	var result []LessonRow

	for rows.Next() {
		var l LessonRow
		var sourceIDs, tags string
		var project, lastReinforced, lastDecayed sql.NullString

		err := rows.Scan(
			&l.ID, &l.Content, &l.Context, &l.Confidence, &l.Reinforcements, &l.Source,
			&sourceIDs, &project, &tags,
			&l.CreatedAt, &l.UpdatedAt, &lastReinforced, &lastDecayed,
			&l.DecayRate, &l.Deleted,
		)
		if err != nil {
			return nil, fmt.Errorf("scan lesson row: %w", err)
		}

		populateLessonJSON(&l, sourceIDs, tags, project, lastReinforced, lastDecayed)
		result = append(result, l)
	}
	return result, rows.Err()
}
