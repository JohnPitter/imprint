package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// InsightRow represents a row in the insights table.
type InsightRow struct {
	ID                   string          `json:"id"`
	Title                string          `json:"title"`
	Content              string          `json:"content"`
	Confidence           float64         `json:"confidence"`
	Reinforcements       int             `json:"reinforcements"`
	SourceConceptCluster json.RawMessage `json:"sourceConceptCluster"`
	SourceMemoryIDs      json.RawMessage `json:"sourceMemoryIds"`
	SourceLessonIDs      json.RawMessage `json:"sourceLessonIds"`
	SourceCrystalIDs     json.RawMessage `json:"sourceCrystalIds"`
	Project              *string         `json:"project"`
	Tags                 json.RawMessage `json:"tags"`
	CreatedAt            string          `json:"createdAt"`
	UpdatedAt            string          `json:"updatedAt"`
	LastReinforcedAt     *string         `json:"lastReinforcedAt"`
	LastDecayedAt        *string         `json:"lastDecayedAt"`
	DecayRate            float64         `json:"decayRate"`
	Deleted              int             `json:"deleted"`
}

// InsightStore provides CRUD operations for the insights table.
type InsightStore struct {
	db *DB
}

// NewInsightStore creates a new InsightStore.
func NewInsightStore(db *DB) *InsightStore {
	return &InsightStore{db: db}
}

// Create inserts a new insight.
func (s *InsightStore) Create(row *InsightRow) error {
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
	if row.DecayRate == 0 {
		row.DecayRate = 0.01
	}
	row.SourceConceptCluster = defaultJSON(row.SourceConceptCluster)
	row.SourceMemoryIDs = defaultJSON(row.SourceMemoryIDs)
	row.SourceLessonIDs = defaultJSON(row.SourceLessonIDs)
	row.SourceCrystalIDs = defaultJSON(row.SourceCrystalIDs)
	row.Tags = defaultJSON(row.Tags)

	_, err := s.db.Exec(`
		INSERT INTO insights (
			id, title, content, confidence, reinforcements,
			source_concept_cluster, source_memory_ids, source_lesson_ids, source_crystal_ids,
			project, tags, created_at, updated_at,
			last_reinforced_at, last_decayed_at, decay_rate, deleted
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.ID, row.Title, row.Content, row.Confidence, row.Reinforcements,
		string(row.SourceConceptCluster), string(row.SourceMemoryIDs),
		string(row.SourceLessonIDs), string(row.SourceCrystalIDs),
		NullString(row.Project), string(row.Tags),
		row.CreatedAt, row.UpdatedAt,
		NullString(row.LastReinforcedAt), NullString(row.LastDecayedAt),
		row.DecayRate, row.Deleted,
	)
	if err != nil {
		return fmt.Errorf("insert insight: %w", err)
	}
	return nil
}

// GetByID retrieves an insight by ID.
func (s *InsightStore) GetByID(id string) (*InsightRow, error) {
	row := s.db.QueryRow(`
		SELECT id, title, content, confidence, reinforcements,
			COALESCE(source_concept_cluster, '[]'), COALESCE(source_memory_ids, '[]'),
			COALESCE(source_lesson_ids, '[]'), COALESCE(source_crystal_ids, '[]'),
			project, COALESCE(tags, '[]'),
			created_at, updated_at, last_reinforced_at, last_decayed_at,
			decay_rate, deleted
		FROM insights WHERE id = ?`, id)
	return s.scanRow(row)
}

// List returns insights filtered by project, excluding soft-deleted, ordered by confidence DESC.
func (s *InsightStore) List(project string, limit, offset int) ([]InsightRow, error) {
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
			SELECT id, title, content, confidence, reinforcements,
				COALESCE(source_concept_cluster, '[]'), COALESCE(source_memory_ids, '[]'),
				COALESCE(source_lesson_ids, '[]'), COALESCE(source_crystal_ids, '[]'),
				project, COALESCE(tags, '[]'),
				created_at, updated_at, last_reinforced_at, last_decayed_at,
				decay_rate, deleted
			FROM insights
			WHERE deleted = 0 AND project = ?
			ORDER BY confidence DESC LIMIT ? OFFSET ?`, project, limit, offset)
	} else {
		rows, err = s.db.Query(`
			SELECT id, title, content, confidence, reinforcements,
				COALESCE(source_concept_cluster, '[]'), COALESCE(source_memory_ids, '[]'),
				COALESCE(source_lesson_ids, '[]'), COALESCE(source_crystal_ids, '[]'),
				project, COALESCE(tags, '[]'),
				created_at, updated_at, last_reinforced_at, last_decayed_at,
				decay_rate, deleted
			FROM insights
			WHERE deleted = 0
			ORDER BY confidence DESC LIMIT ? OFFSET ?`, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("list insights: %w", err)
	}
	defer rows.Close()

	return s.scanRows(rows)
}

// Count returns the total number of non-deleted insights, optionally filtered by project.
func (s *InsightStore) Count(project string) (int, error) {
	var count int
	var err error
	if project != "" {
		err = s.db.QueryRow(`SELECT COUNT(*) FROM insights WHERE deleted = 0 AND project = ?`, project).Scan(&count)
	} else {
		err = s.db.QueryRow(`SELECT COUNT(*) FROM insights WHERE deleted = 0`).Scan(&count)
	}
	if err != nil {
		return 0, fmt.Errorf("count insights: %w", err)
	}
	return count, nil
}

// Search performs a LIKE search on insight content and title, excluding soft-deleted.
func (s *InsightStore) Search(query string, limit int) ([]InsightRow, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.Query(`
		SELECT id, title, content, confidence, reinforcements,
			COALESCE(source_concept_cluster, '[]'), COALESCE(source_memory_ids, '[]'),
			COALESCE(source_lesson_ids, '[]'), COALESCE(source_crystal_ids, '[]'),
			project, COALESCE(tags, '[]'),
			created_at, updated_at, last_reinforced_at, last_decayed_at,
			decay_rate, deleted
		FROM insights
		WHERE deleted = 0 AND (content LIKE '%' || ? || '%' OR title LIKE '%' || ? || '%')
		ORDER BY confidence DESC LIMIT ?`, query, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search insights: %w", err)
	}
	defer rows.Close()

	return s.scanRows(rows)
}

// --- Scan helpers ---

func populateInsightJSON(i *InsightRow, srcCluster, srcMemIDs, srcLessonIDs, srcCrystalIDs, tags string, project, lastReinforced, lastDecayed sql.NullString) {
	i.SourceConceptCluster = json.RawMessage(srcCluster)
	i.SourceMemoryIDs = json.RawMessage(srcMemIDs)
	i.SourceLessonIDs = json.RawMessage(srcLessonIDs)
	i.SourceCrystalIDs = json.RawMessage(srcCrystalIDs)
	i.Tags = json.RawMessage(tags)
	if project.Valid {
		i.Project = &project.String
	}
	if lastReinforced.Valid {
		i.LastReinforcedAt = &lastReinforced.String
	}
	if lastDecayed.Valid {
		i.LastDecayedAt = &lastDecayed.String
	}
}

func (s *InsightStore) scanRow(row *sql.Row) (*InsightRow, error) {
	var i InsightRow
	var srcCluster, srcMemIDs, srcLessonIDs, srcCrystalIDs, tags string
	var project, lastReinforced, lastDecayed sql.NullString

	err := row.Scan(
		&i.ID, &i.Title, &i.Content, &i.Confidence, &i.Reinforcements,
		&srcCluster, &srcMemIDs, &srcLessonIDs, &srcCrystalIDs,
		&project, &tags,
		&i.CreatedAt, &i.UpdatedAt, &lastReinforced, &lastDecayed,
		&i.DecayRate, &i.Deleted,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("insight not found")
		}
		return nil, fmt.Errorf("scan insight: %w", err)
	}

	populateInsightJSON(&i, srcCluster, srcMemIDs, srcLessonIDs, srcCrystalIDs, tags, project, lastReinforced, lastDecayed)
	return &i, nil
}

func (s *InsightStore) scanRows(rows *sql.Rows) ([]InsightRow, error) {
	var result []InsightRow

	for rows.Next() {
		var i InsightRow
		var srcCluster, srcMemIDs, srcLessonIDs, srcCrystalIDs, tags string
		var project, lastReinforced, lastDecayed sql.NullString

		err := rows.Scan(
			&i.ID, &i.Title, &i.Content, &i.Confidence, &i.Reinforcements,
			&srcCluster, &srcMemIDs, &srcLessonIDs, &srcCrystalIDs,
			&project, &tags,
			&i.CreatedAt, &i.UpdatedAt, &lastReinforced, &lastDecayed,
			&i.DecayRate, &i.Deleted,
		)
		if err != nil {
			return nil, fmt.Errorf("scan insight row: %w", err)
		}

		populateInsightJSON(&i, srcCluster, srcMemIDs, srcLessonIDs, srcCrystalIDs, tags, project, lastReinforced, lastDecayed)
		result = append(result, i)
	}
	return result, rows.Err()
}
