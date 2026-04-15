package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// GraphNodeRow represents a row in the graph_nodes table.
type GraphNodeRow struct {
	ID                   string          `json:"id"`
	Type                 string          `json:"type"`
	Name                 string          `json:"name"`
	Properties           json.RawMessage `json:"properties"`
	Aliases              json.RawMessage `json:"aliases"`
	SourceObservationIDs json.RawMessage `json:"sourceObservationIds"`
	CreatedAt            string          `json:"createdAt"`
}

// GraphEdgeRow represents a row in the graph_edges table.
type GraphEdgeRow struct {
	ID                   string          `json:"id"`
	Type                 string          `json:"type"`
	SourceNodeID         string          `json:"sourceNodeId"`
	TargetNodeID         string          `json:"targetNodeId"`
	Weight               float64         `json:"weight"`
	SourceObservationIDs json.RawMessage `json:"sourceObservationIds"`
	CreatedAt            string          `json:"createdAt"`
	ValidFrom            *string         `json:"validFrom"`
	ValidTo              *string         `json:"validTo"`
	IsLatest             int             `json:"isLatest"`
	Version              int             `json:"version"`
	Context              json.RawMessage `json:"context"`
}

// GraphStore provides CRUD operations for the graph_nodes and graph_edges tables.
type GraphStore struct {
	db *DB
}

// NewGraphStore creates a new GraphStore.
func NewGraphStore(db *DB) *GraphStore {
	return &GraphStore{db: db}
}

// --- Nodes ---

// CreateNode inserts a graph node. Uses string() for JSON fields to store as TEXT.
func (s *GraphStore) CreateNode(node *GraphNodeRow) error {
	if node.CreatedAt == "" {
		node.CreatedAt = TimeToString(time.Now())
	}
	if len(node.Properties) == 0 {
		node.Properties = json.RawMessage("{}")
	}
	if len(node.Aliases) == 0 {
		node.Aliases = json.RawMessage("[]")
	}
	if len(node.SourceObservationIDs) == 0 {
		node.SourceObservationIDs = json.RawMessage("[]")
	}

	_, err := s.db.Exec(
		`INSERT INTO graph_nodes (id, type, name, properties, aliases, source_observation_ids, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		node.ID, node.Type, node.Name,
		string(node.Properties), string(node.Aliases), string(node.SourceObservationIDs),
		node.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert graph node: %w", err)
	}
	return nil
}

// GetNodeByID retrieves a node by ID.
func (s *GraphStore) GetNodeByID(id string) (*GraphNodeRow, error) {
	row := s.db.QueryRow(
		`SELECT id, type, name, COALESCE(properties,'{}'), COALESCE(aliases,'[]'),
		        COALESCE(source_observation_ids,'[]'), created_at
		 FROM graph_nodes WHERE id = ?`, id)
	return s.scanNode(row)
}

// FindNodeByName finds a node by type and name (case-insensitive).
func (s *GraphStore) FindNodeByName(nodeType, name string) (*GraphNodeRow, error) {
	row := s.db.QueryRow(
		`SELECT id, type, name, COALESCE(properties,'{}'), COALESCE(aliases,'[]'),
		        COALESCE(source_observation_ids,'[]'), created_at
		 FROM graph_nodes WHERE type = ? AND LOWER(name) = LOWER(?)`, nodeType, name)
	return s.scanNode(row)
}

// ListNodes returns nodes with optional type filter.
func (s *GraphStore) ListNodes(nodeType string, limit, offset int) ([]GraphNodeRow, error) {
	if limit <= 0 {
		limit = 100
	}

	var rows *sql.Rows
	var err error

	if nodeType != "" {
		rows, err = s.db.Query(
			`SELECT id, type, name, COALESCE(properties,'{}'), COALESCE(aliases,'[]'),
			        COALESCE(source_observation_ids,'[]'), created_at
			 FROM graph_nodes WHERE type = ?
			 ORDER BY created_at DESC LIMIT ? OFFSET ?`, nodeType, limit, offset)
	} else {
		rows, err = s.db.Query(
			`SELECT id, type, name, COALESCE(properties,'{}'), COALESCE(aliases,'[]'),
			        COALESCE(source_observation_ids,'[]'), created_at
			 FROM graph_nodes
			 ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("list graph nodes: %w", err)
	}
	defer rows.Close()

	return s.scanNodes(rows)
}

// DeleteNode deletes a node and its connected edges.
func (s *GraphStore) DeleteNode(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin delete node tx: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM graph_edges WHERE source_node_id = ? OR target_node_id = ?`, id, id); err != nil {
		tx.Rollback()
		return fmt.Errorf("delete node edges: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM graph_nodes WHERE id = ?`, id); err != nil {
		tx.Rollback()
		return fmt.Errorf("delete node: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete node: %w", err)
	}
	return nil
}

// CountNodes returns counts by type.
func (s *GraphStore) CountNodes() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT type, COUNT(*) FROM graph_nodes GROUP BY type`)
	if err != nil {
		return nil, fmt.Errorf("count nodes: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var t string
		var c int
		if err := rows.Scan(&t, &c); err != nil {
			return nil, err
		}
		counts[t] = c
	}
	return counts, rows.Err()
}

// --- Edges ---

// CreateEdge inserts a graph edge.
func (s *GraphStore) CreateEdge(edge *GraphEdgeRow) error {
	if edge.CreatedAt == "" {
		edge.CreatedAt = TimeToString(time.Now())
	}
	if len(edge.SourceObservationIDs) == 0 {
		edge.SourceObservationIDs = json.RawMessage("[]")
	}
	if len(edge.Context) == 0 {
		edge.Context = json.RawMessage("{}")
	}
	if edge.Weight == 0 {
		edge.Weight = 0.5
	}
	if edge.Version == 0 {
		edge.Version = 1
	}

	_, err := s.db.Exec(
		`INSERT INTO graph_edges (id, type, source_node_id, target_node_id, weight,
		    source_observation_ids, created_at, valid_from, valid_to, is_latest, version, context)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		edge.ID, edge.Type, edge.SourceNodeID, edge.TargetNodeID, edge.Weight,
		string(edge.SourceObservationIDs), edge.CreatedAt,
		NullString(edge.ValidFrom), NullString(edge.ValidTo),
		edge.IsLatest, edge.Version, string(edge.Context),
	)
	if err != nil {
		return fmt.Errorf("insert graph edge: %w", err)
	}
	return nil
}

// GetEdgeByID retrieves an edge by ID.
func (s *GraphStore) GetEdgeByID(id string) (*GraphEdgeRow, error) {
	row := s.db.QueryRow(
		`SELECT id, type, source_node_id, target_node_id, weight,
		        COALESCE(source_observation_ids,'[]'), created_at,
		        valid_from, valid_to, is_latest, version, COALESCE(context,'{}')
		 FROM graph_edges WHERE id = ?`, id)
	return s.scanEdge(row)
}

// GetEdgesFrom returns all edges originating from a node.
// If asOf is non-nil, only edges temporally valid at that time are returned.
func (s *GraphStore) GetEdgesFrom(nodeID string, limit int, asOf *time.Time) ([]GraphEdgeRow, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `SELECT id, type, source_node_id, target_node_id, weight,
	        COALESCE(source_observation_ids,'[]'), created_at,
	        valid_from, valid_to, is_latest, version, COALESCE(context,'{}')
	 FROM graph_edges
	 WHERE source_node_id = ? AND is_latest = 1`
	args := []any{nodeID}

	if asOf != nil {
		ts := TimeToString(*asOf)
		query += ` AND (valid_from IS NULL OR valid_from <= ?) AND (valid_to IS NULL OR valid_to > ?)`
		args = append(args, ts, ts)
	}
	query += ` ORDER BY weight DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get edges from: %w", err)
	}
	defer rows.Close()

	return s.scanEdges(rows)
}

// GetEdgesTo returns all edges pointing to a node.
// If asOf is non-nil, only edges temporally valid at that time are returned.
func (s *GraphStore) GetEdgesTo(nodeID string, limit int, asOf *time.Time) ([]GraphEdgeRow, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `SELECT id, type, source_node_id, target_node_id, weight,
	        COALESCE(source_observation_ids,'[]'), created_at,
	        valid_from, valid_to, is_latest, version, COALESCE(context,'{}')
	 FROM graph_edges
	 WHERE target_node_id = ? AND is_latest = 1`
	args := []any{nodeID}

	if asOf != nil {
		ts := TimeToString(*asOf)
		query += ` AND (valid_from IS NULL OR valid_from <= ?) AND (valid_to IS NULL OR valid_to > ?)`
		args = append(args, ts, ts)
	}
	query += ` ORDER BY weight DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get edges to: %w", err)
	}
	defer rows.Close()

	return s.scanEdges(rows)
}

// InvalidateEdge marks an edge as no longer valid by setting valid_to = now and is_latest = 0.
func (s *GraphStore) InvalidateEdge(id string) error {
	now := TimeToString(time.Now())
	result, err := s.db.Exec(
		`UPDATE graph_edges SET valid_to = ?, is_latest = 0 WHERE id = ?`,
		now, id,
	)
	if err != nil {
		return fmt.Errorf("invalidate edge: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("invalidate edge rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("edge not found: %s", id)
	}
	return nil
}

// GetNeighbors returns all nodes connected to a given node (both directions).
// If asOf is non-nil, only nodes connected via temporally valid edges are returned.
func (s *GraphStore) GetNeighbors(nodeID string, limit int, asOf *time.Time) ([]GraphNodeRow, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT DISTINCT n.id, n.type, n.name, COALESCE(n.properties,'{}'),
		       COALESCE(n.aliases,'[]'), COALESCE(n.source_observation_ids,'[]'), n.created_at
		FROM graph_nodes n
		JOIN graph_edges e ON (e.source_node_id = n.id OR e.target_node_id = n.id)
		WHERE (e.source_node_id = ? OR e.target_node_id = ?) AND n.id != ? AND e.is_latest = 1`
	args := []any{nodeID, nodeID, nodeID}

	if asOf != nil {
		ts := TimeToString(*asOf)
		query += ` AND (e.valid_from IS NULL OR e.valid_from <= ?) AND (e.valid_to IS NULL OR e.valid_to > ?)`
		args = append(args, ts, ts)
	}
	query += ` LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get neighbors: %w", err)
	}
	defer rows.Close()

	return s.scanNodes(rows)
}

// CountEdges returns counts by type.
func (s *GraphStore) CountEdges() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT type, COUNT(*) FROM graph_edges WHERE is_latest = 1 GROUP BY type`)
	if err != nil {
		return nil, fmt.Errorf("count edges: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var t string
		var c int
		if err := rows.Scan(&t, &c); err != nil {
			return nil, err
		}
		counts[t] = c
	}
	return counts, rows.Err()
}

// --- Scan helpers ---

func (s *GraphStore) scanNode(row *sql.Row) (*GraphNodeRow, error) {
	var n GraphNodeRow
	var props, aliases, sourceObs string

	err := row.Scan(&n.ID, &n.Type, &n.Name, &props, &aliases, &sourceObs, &n.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("graph node not found")
		}
		return nil, fmt.Errorf("scan graph node: %w", err)
	}

	n.Properties = json.RawMessage(props)
	n.Aliases = json.RawMessage(aliases)
	n.SourceObservationIDs = json.RawMessage(sourceObs)
	return &n, nil
}

func (s *GraphStore) scanNodes(rows *sql.Rows) ([]GraphNodeRow, error) {
	var result []GraphNodeRow

	for rows.Next() {
		var n GraphNodeRow
		var props, aliases, sourceObs string

		if err := rows.Scan(&n.ID, &n.Type, &n.Name, &props, &aliases, &sourceObs, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan graph node row: %w", err)
		}

		n.Properties = json.RawMessage(props)
		n.Aliases = json.RawMessage(aliases)
		n.SourceObservationIDs = json.RawMessage(sourceObs)
		result = append(result, n)
	}
	return result, rows.Err()
}

func (s *GraphStore) scanEdge(row *sql.Row) (*GraphEdgeRow, error) {
	var e GraphEdgeRow
	var sourceObs, ctx string
	var validFrom, validTo sql.NullString

	err := row.Scan(&e.ID, &e.Type, &e.SourceNodeID, &e.TargetNodeID, &e.Weight,
		&sourceObs, &e.CreatedAt, &validFrom, &validTo, &e.IsLatest, &e.Version, &ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("graph edge not found")
		}
		return nil, fmt.Errorf("scan graph edge: %w", err)
	}

	e.SourceObservationIDs = json.RawMessage(sourceObs)
	e.Context = json.RawMessage(ctx)
	if validFrom.Valid {
		e.ValidFrom = &validFrom.String
	}
	if validTo.Valid {
		e.ValidTo = &validTo.String
	}
	return &e, nil
}

func (s *GraphStore) scanEdges(rows *sql.Rows) ([]GraphEdgeRow, error) {
	var result []GraphEdgeRow

	for rows.Next() {
		var e GraphEdgeRow
		var sourceObs, ctx string
		var validFrom, validTo sql.NullString

		if err := rows.Scan(&e.ID, &e.Type, &e.SourceNodeID, &e.TargetNodeID, &e.Weight,
			&sourceObs, &e.CreatedAt, &validFrom, &validTo, &e.IsLatest, &e.Version, &ctx); err != nil {
			return nil, fmt.Errorf("scan graph edge row: %w", err)
		}

		e.SourceObservationIDs = json.RawMessage(sourceObs)
		e.Context = json.RawMessage(ctx)
		if validFrom.Valid {
			e.ValidFrom = &validFrom.String
		}
		if validTo.Valid {
			e.ValidTo = &validTo.String
		}
		result = append(result, e)
	}
	return result, rows.Err()
}
