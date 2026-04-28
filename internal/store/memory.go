package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// MemoryRow represents a row in the memories table.
type MemoryRow struct {
	ID                   string          `json:"id"`
	CreatedAt            string          `json:"createdAt"`
	UpdatedAt            string          `json:"updatedAt"`
	Type                 string          `json:"type"`
	Title                string          `json:"title"`
	Content              string          `json:"content"`
	Concepts             json.RawMessage `json:"concepts"`
	Files                json.RawMessage `json:"files"`
	SessionIDs           json.RawMessage `json:"sessionIds"`
	Strength             int             `json:"strength"`
	Version              int             `json:"version"`
	ParentID             *string         `json:"parentId"`
	Supersedes           json.RawMessage `json:"supersedes"`
	SourceObservationIDs json.RawMessage `json:"sourceObservationIds"`
	IsLatest             int             `json:"isLatest"`
	ForgetAfter          *string         `json:"forgetAfter"`
	TTLDays              *int            `json:"ttlDays"`
}

func defaultJSON(b json.RawMessage) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage("[]")
	}
	return b
}

// MemoryStore provides CRUD operations for the memories table.
type MemoryStore struct {
	db *DB
}

// NewMemoryStore creates a new MemoryStore.
func NewMemoryStore(db *DB) *MemoryStore {
	return &MemoryStore{db: db}
}

// Create inserts a new memory.
func (s *MemoryStore) Create(row *MemoryRow) error {
	now := TimeToString(time.Now())
	if row.CreatedAt == "" {
		row.CreatedAt = now
	}
	if row.UpdatedAt == "" {
		row.UpdatedAt = now
	}
	row.Concepts = defaultJSON(row.Concepts)
	row.Files = defaultJSON(row.Files)
	row.SessionIDs = defaultJSON(row.SessionIDs)
	row.Supersedes = defaultJSON(row.Supersedes)
	row.SourceObservationIDs = defaultJSON(row.SourceObservationIDs)
	if row.Version == 0 {
		row.Version = 1
	}

	_, err := s.db.Exec(`
		INSERT INTO memories (
			id, created_at, updated_at, type, title, content,
			concepts, files, session_ids, strength, version,
			parent_id, supersedes, source_observation_ids,
			is_latest, forget_after, ttl_days
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.ID, row.CreatedAt, row.UpdatedAt, row.Type, row.Title, row.Content,
		string(row.Concepts), string(row.Files), string(row.SessionIDs), row.Strength, row.Version,
		NullString(row.ParentID), string(row.Supersedes), string(row.SourceObservationIDs),
		row.IsLatest, NullString(row.ForgetAfter), row.TTLDays,
	)
	if err != nil {
		return fmt.Errorf("insert memory: %w", err)
	}
	return nil
}

// GetByID retrieves a memory by ID.
func (s *MemoryStore) GetByID(id string) (*MemoryRow, error) {
	row := s.db.QueryRow(`
		SELECT id, created_at, updated_at, type, title, content,
			COALESCE(concepts, '[]'), COALESCE(files, '[]'),
			COALESCE(session_ids, '[]'), strength, version,
			parent_id, COALESCE(supersedes, '[]'),
			COALESCE(source_observation_ids, '[]'),
			is_latest, forget_after, ttl_days
		FROM memories WHERE id = ?`, id)

	return s.scanRow(row)
}

// History walks the parent_id chain backward from the given memory and
// returns every version, oldest first. The starting row is included as the
// last element. Returns at most 100 versions to guard against accidental
// cycles in old data.
func (s *MemoryStore) History(id string) ([]MemoryRow, error) {
	const maxDepth = 100
	chain := make([]MemoryRow, 0, 4)
	cur := id
	seen := map[string]bool{}
	for i := 0; i < maxDepth && cur != ""; i++ {
		if seen[cur] {
			break // cycle guard
		}
		seen[cur] = true
		row, err := s.GetByID(cur)
		if err != nil {
			if i == 0 {
				return nil, err // first lookup must succeed
			}
			break // parent missing — stop quietly
		}
		chain = append(chain, *row)
		if row.ParentID == nil {
			break
		}
		cur = *row.ParentID
	}
	// Reverse to oldest-first.
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}

// List returns latest memories, optionally filtered by type.
// Pass empty string for memType to list all types.
func (s *MemoryStore) List(memType string, limit, offset int) ([]MemoryRow, error) {
	var rows *sql.Rows
	var err error

	if memType != "" {
		rows, err = s.db.Query(`
			SELECT id, created_at, updated_at, type, title, content,
				COALESCE(concepts, '[]'), COALESCE(files, '[]'),
				COALESCE(session_ids, '[]'), strength, version,
				parent_id, COALESCE(supersedes, '[]'),
				COALESCE(source_observation_ids, '[]'),
				is_latest, forget_after, ttl_days
			FROM memories
			WHERE is_latest = 1 AND type = ?
			ORDER BY updated_at DESC
			LIMIT ? OFFSET ?`, memType, limit, offset)
	} else {
		rows, err = s.db.Query(`
			SELECT id, created_at, updated_at, type, title, content,
				COALESCE(concepts, '[]'), COALESCE(files, '[]'),
				COALESCE(session_ids, '[]'), strength, version,
				parent_id, COALESCE(supersedes, '[]'),
				COALESCE(source_observation_ids, '[]'),
				is_latest, forget_after, ttl_days
			FROM memories
			WHERE is_latest = 1
			ORDER BY updated_at DESC
			LIMIT ? OFFSET ?`, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()

	return s.scanRows(rows)
}

// Update updates a memory's mutable fields.
func (s *MemoryStore) Update(row *MemoryRow) error {
	row.UpdatedAt = TimeToString(time.Now())

	_, err := s.db.Exec(`
		UPDATE memories SET
			updated_at = ?, type = ?, title = ?, content = ?,
			concepts = ?, files = ?, session_ids = ?,
			strength = ?, version = ?, supersedes = ?,
			source_observation_ids = ?, is_latest = ?,
			forget_after = ?, ttl_days = ?
		WHERE id = ?`,
		row.UpdatedAt, row.Type, row.Title, row.Content,
		string(row.Concepts), string(row.Files), string(row.SessionIDs),
		row.Strength, row.Version, string(row.Supersedes),
		string(row.SourceObservationIDs), row.IsLatest,
		NullString(row.ForgetAfter), row.TTLDays,
		row.ID,
	)
	if err != nil {
		return fmt.Errorf("update memory: %w", err)
	}
	return nil
}

// Delete soft-deletes a memory by setting is_latest = 0.
func (s *MemoryStore) Delete(id string) error {
	_, err := s.db.Exec(`UPDATE memories SET is_latest = 0, updated_at = ? WHERE id = ?`,
		TimeToString(time.Now()), id)
	if err != nil {
		return fmt.Errorf("soft-delete memory: %w", err)
	}
	return nil
}

// HardDelete permanently removes a memory from the database.
func (s *MemoryStore) HardDelete(id string) error {
	_, err := s.db.Exec(`DELETE FROM memories WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("hard-delete memory: %w", err)
	}
	return nil
}

// Supersede creates a new version of a memory, marking the old one as not latest.
// The new memory's parent_id is set to oldID and supersedes includes oldID.
func (s *MemoryStore) Supersede(oldID string, newMem *MemoryRow) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin supersede tx: %w", err)
	}

	// Mark old memory as no longer latest.
	_, err = tx.Exec(`UPDATE memories SET is_latest = 0, updated_at = ? WHERE id = ?`,
		TimeToString(time.Now()), oldID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("mark old memory: %w", err)
	}

	// Set lineage on new memory.
	newMem.ParentID = &oldID
	supersedes, _ := json.Marshal([]string{oldID})
	newMem.Supersedes = json.RawMessage(supersedes)
	newMem.IsLatest = 1

	now := TimeToString(time.Now())
	if newMem.CreatedAt == "" {
		newMem.CreatedAt = now
	}
	if newMem.UpdatedAt == "" {
		newMem.UpdatedAt = now
	}
	newMem.Concepts = defaultJSON(newMem.Concepts)
	newMem.Files = defaultJSON(newMem.Files)
	newMem.SessionIDs = defaultJSON(newMem.SessionIDs)
	newMem.SourceObservationIDs = defaultJSON(newMem.SourceObservationIDs)
	if newMem.Version == 0 {
		newMem.Version = 1
	}

	_, err = tx.Exec(`
		INSERT INTO memories (
			id, created_at, updated_at, type, title, content,
			concepts, files, session_ids, strength, version,
			parent_id, supersedes, source_observation_ids,
			is_latest, forget_after, ttl_days
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		newMem.ID, newMem.CreatedAt, newMem.UpdatedAt, newMem.Type, newMem.Title, newMem.Content,
		string(newMem.Concepts), string(newMem.Files), string(newMem.SessionIDs), newMem.Strength, newMem.Version,
		NullString(newMem.ParentID), string(newMem.Supersedes), string(newMem.SourceObservationIDs),
		newMem.IsLatest, NullString(newMem.ForgetAfter), newMem.TTLDays,
	)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("insert superseding memory: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit supersede tx: %w", err)
	}
	return nil
}

// ListByStrength returns latest memories with strength >= minStrength, ordered by strength DESC.
func (s *MemoryStore) ListByStrength(minStrength int, limit int) ([]MemoryRow, error) {
	rows, err := s.db.Query(`
		SELECT id, created_at, updated_at, type, title, content,
			COALESCE(concepts, '[]'), COALESCE(files, '[]'),
			COALESCE(session_ids, '[]'), strength, version,
			parent_id, COALESCE(supersedes, '[]'),
			COALESCE(source_observation_ids, '[]'),
			is_latest, forget_after, ttl_days
		FROM memories
		WHERE is_latest = 1 AND strength >= ?
		ORDER BY strength DESC
		LIMIT ?`, minStrength, limit)
	if err != nil {
		return nil, fmt.Errorf("list memories by strength: %w", err)
	}
	defer rows.Close()

	return s.scanRows(rows)
}

// ListByConcept returns latest memories that contain a concept (JSON LIKE search).
func (s *MemoryStore) ListByConcept(concept string, limit int) ([]MemoryRow, error) {
	rows, err := s.db.Query(`
		SELECT id, created_at, updated_at, type, title, content,
			COALESCE(concepts, '[]'), COALESCE(files, '[]'),
			COALESCE(session_ids, '[]'), strength, version,
			parent_id, COALESCE(supersedes, '[]'),
			COALESCE(source_observation_ids, '[]'),
			is_latest, forget_after, ttl_days
		FROM memories
		WHERE is_latest = 1 AND concepts LIKE '%"' || ? || '"%'
		ORDER BY strength DESC
		LIMIT ?`, concept, limit)
	if err != nil {
		return nil, fmt.Errorf("list memories by concept: %w", err)
	}
	defer rows.Close()

	return s.scanRows(rows)
}

// Count returns the total count of latest memories.
func (s *MemoryStore) Count() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM memories WHERE is_latest = 1`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count memories: %w", err)
	}
	return count, nil
}

// ConceptCount is the result of TopConcepts: a concept name and how many
// latest memories carry it.
type ConceptCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// TopConcepts returns the top N concepts across all latest memories,
// aggregated server-side via json_each so the client doesn't have to scan
// the full memories table to build a tag cloud.
func (s *MemoryStore) TopConcepts(limit int) ([]ConceptCount, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT je.value AS concept, COUNT(*) AS c
		FROM memories m, json_each(m.concepts) je
		WHERE m.is_latest = 1 AND je.value IS NOT NULL AND je.value != ''
		GROUP BY je.value
		ORDER BY c DESC, concept ASC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("top concepts: %w", err)
	}
	defer rows.Close()

	var result []ConceptCount
	for rows.Next() {
		var cc ConceptCount
		if err := rows.Scan(&cc.Name, &cc.Count); err != nil {
			return nil, fmt.Errorf("scan concept: %w", err)
		}
		result = append(result, cc)
	}
	return result, rows.Err()
}

// ListExpired returns latest memories past their forget_after date.
func (s *MemoryStore) ListExpired() ([]MemoryRow, error) {
	now := TimeToString(time.Now())
	rows, err := s.db.Query(`
		SELECT id, created_at, updated_at, type, title, content,
			COALESCE(concepts, '[]'), COALESCE(files, '[]'),
			COALESCE(session_ids, '[]'), strength, version,
			parent_id, COALESCE(supersedes, '[]'),
			COALESCE(source_observation_ids, '[]'),
			is_latest, forget_after, ttl_days
		FROM memories
		WHERE is_latest = 1 AND forget_after IS NOT NULL AND forget_after <= ?
		ORDER BY forget_after ASC`, now)
	if err != nil {
		return nil, fmt.Errorf("list expired memories: %w", err)
	}
	defer rows.Close()

	return s.scanRows(rows)
}

// scanMemoryFields scans JSON string columns into the MemoryRow's json.RawMessage fields.
func populateMemoryJSON(m *MemoryRow, concepts, files, sessionIDs, supersedes, sourceObsIDs string, parentID, forgetAfter sql.NullString, ttlDays sql.NullInt64) {
	m.Concepts = json.RawMessage(concepts)
	m.Files = json.RawMessage(files)
	m.SessionIDs = json.RawMessage(sessionIDs)
	m.Supersedes = json.RawMessage(supersedes)
	m.SourceObservationIDs = json.RawMessage(sourceObsIDs)
	if parentID.Valid {
		m.ParentID = &parentID.String
	}
	if forgetAfter.Valid {
		m.ForgetAfter = &forgetAfter.String
	}
	if ttlDays.Valid {
		v := int(ttlDays.Int64)
		m.TTLDays = &v
	}
}

// scanRow scans a single row into a MemoryRow.
func (s *MemoryStore) scanRow(row *sql.Row) (*MemoryRow, error) {
	var m MemoryRow
	var concepts, files, sessionIDs, supersedes, sourceObsIDs string
	var parentID, forgetAfter sql.NullString
	var ttlDays sql.NullInt64

	err := row.Scan(
		&m.ID, &m.CreatedAt, &m.UpdatedAt, &m.Type, &m.Title, &m.Content,
		&concepts, &files, &sessionIDs, &m.Strength, &m.Version,
		&parentID, &supersedes, &sourceObsIDs,
		&m.IsLatest, &forgetAfter, &ttlDays,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("memory not found")
		}
		return nil, fmt.Errorf("scan memory: %w", err)
	}

	populateMemoryJSON(&m, concepts, files, sessionIDs, supersedes, sourceObsIDs, parentID, forgetAfter, ttlDays)
	return &m, nil
}

// scanRows scans multiple rows into a slice of MemoryRow.
func (s *MemoryStore) scanRows(rows *sql.Rows) ([]MemoryRow, error) {
	var result []MemoryRow

	for rows.Next() {
		var m MemoryRow
		var concepts, files, sessionIDs, supersedes, sourceObsIDs string
		var parentID, forgetAfter sql.NullString
		var ttlDays sql.NullInt64

		err := rows.Scan(
			&m.ID, &m.CreatedAt, &m.UpdatedAt, &m.Type, &m.Title, &m.Content,
			&concepts, &files, &sessionIDs, &m.Strength, &m.Version,
			&parentID, &supersedes, &sourceObsIDs,
			&m.IsLatest, &forgetAfter, &ttlDays,
		)
		if err != nil {
			return nil, fmt.Errorf("scan memory row: %w", err)
		}

		populateMemoryJSON(&m, concepts, files, sessionIDs, supersedes, sourceObsIDs, parentID, forgetAfter, ttlDays)
		result = append(result, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memory rows: %w", err)
	}

	return result, nil
}
