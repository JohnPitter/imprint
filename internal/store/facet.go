package store

import (
	"database/sql"
	"fmt"
	"time"
)

// FacetRow represents a row in the facets table.
type FacetRow struct {
	ID         string `json:"id"`
	TargetID   string `json:"targetId"`
	TargetType string `json:"targetType"`
	Dimension  string `json:"dimension"`
	Value      string `json:"value"`
	CreatedAt  string `json:"createdAt"`
}

// FacetStore provides CRUD operations for the facets table.
type FacetStore struct {
	db *DB
}

// NewFacetStore creates a new FacetStore.
func NewFacetStore(db *DB) *FacetStore {
	return &FacetStore{db: db}
}

// Create inserts a new facet.
func (s *FacetStore) Create(row *FacetRow) error {
	if row.CreatedAt == "" {
		row.CreatedAt = TimeToString(time.Now())
	}

	_, err := s.db.Exec(`
		INSERT INTO facets (id, target_id, target_type, dimension, value, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		row.ID, row.TargetID, row.TargetType, row.Dimension, row.Value, row.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert facet: %w", err)
	}
	return nil
}

// GetByTarget returns all facets for a given target entity.
func (s *FacetStore) GetByTarget(targetID, targetType string) ([]FacetRow, error) {
	rows, err := s.db.Query(`
		SELECT id, target_id, target_type, dimension, value, created_at
		FROM facets
		WHERE target_id = ? AND target_type = ?
		ORDER BY created_at ASC`, targetID, targetType)
	if err != nil {
		return nil, fmt.Errorf("get facets by target: %w", err)
	}
	defer rows.Close()

	return s.scanRows(rows)
}

// Remove deletes a facet by ID.
func (s *FacetStore) Remove(id string) error {
	_, err := s.db.Exec(`DELETE FROM facets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("remove facet: %w", err)
	}
	return nil
}

// QueryByDimension returns facets matching a specific dimension and value.
func (s *FacetStore) QueryByDimension(dimension, value string, limit int) ([]FacetRow, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.Query(`
		SELECT id, target_id, target_type, dimension, value, created_at
		FROM facets
		WHERE dimension = ? AND value = ?
		ORDER BY created_at DESC LIMIT ?`, dimension, value, limit)
	if err != nil {
		return nil, fmt.Errorf("query facets by dimension: %w", err)
	}
	defer rows.Close()

	return s.scanRows(rows)
}

// Stats returns counts of facets grouped by dimension.
func (s *FacetStore) Stats() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT dimension, COUNT(*) FROM facets GROUP BY dimension`)
	if err != nil {
		return nil, fmt.Errorf("facet stats: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var dim string
		var c int
		if err := rows.Scan(&dim, &c); err != nil {
			return nil, err
		}
		counts[dim] = c
	}
	return counts, rows.Err()
}

// --- Scan helpers ---

func (s *FacetStore) scanRows(rows *sql.Rows) ([]FacetRow, error) {
	var result []FacetRow

	for rows.Next() {
		var f FacetRow
		if err := rows.Scan(&f.ID, &f.TargetID, &f.TargetType, &f.Dimension, &f.Value, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan facet row: %w", err)
		}
		result = append(result, f)
	}
	return result, rows.Err()
}
