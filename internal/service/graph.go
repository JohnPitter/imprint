package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"imprint/internal/pipeline"
	"imprint/internal/store"

	"github.com/google/uuid"
)

// GraphService manages knowledge graph extraction, storage, and querying.
type GraphService struct {
	c         *Container
	extractor *pipeline.GraphExtractor
}

// NewGraphService creates a new GraphService.
func NewGraphService(c *Container, extractor *pipeline.GraphExtractor) *GraphService {
	return &GraphService{c: c, extractor: extractor}
}

// ExtractAndStore extracts entities from a compressed observation and stores them.
func (s *GraphService) ExtractAndStore(ctx context.Context, obs *store.CompressedObservationRow) error {
	result, err := s.extractor.Extract(ctx, obs)
	if err != nil {
		return err
	}

	// Deduplicate and store nodes
	nodeIDs := map[string]string{} // name -> id
	for _, n := range result.Nodes {
		// Check if node already exists
		existing, err := s.c.Graph.FindNodeByName(n.Type, n.Name)
		if err == nil && existing != nil {
			nodeIDs[n.Name] = existing.ID
			continue
		}
		id := "gn_" + uuid.New().String()[:8]
		node := &store.GraphNodeRow{
			ID:                   id,
			Type:                 n.Type,
			Name:                 n.Name,
			SourceObservationIDs: json.RawMessage(`["` + obs.ID + `"]`),
		}
		if err := s.c.Graph.CreateNode(node); err != nil {
			continue
		}
		nodeIDs[n.Name] = id
	}

	// Store edges
	for _, e := range result.Edges {
		sourceID, ok1 := nodeIDs[e.Source]
		targetID, ok2 := nodeIDs[e.Target]
		if !ok1 || !ok2 {
			continue
		}
		edgeID := "ge_" + uuid.New().String()[:8]
		edge := &store.GraphEdgeRow{
			ID:                   edgeID,
			Type:                 e.Type,
			SourceNodeID:         sourceID,
			TargetNodeID:         targetID,
			Weight:               e.Weight,
			IsLatest:             1,
			Version:              1,
			SourceObservationIDs: json.RawMessage(`["` + obs.ID + `"]`),
		}
		s.c.Graph.CreateEdge(edge)
	}

	return nil
}

// GraphQueryResult holds the BFS traversal result.
type GraphQueryResult struct {
	Nodes []store.GraphNodeRow `json:"nodes"`
	Edges []store.GraphEdgeRow `json:"edges"`
}

// Query performs a BFS traversal from a start node up to maxDepth levels.
func (s *GraphService) Query(startNodeID string, maxDepth int) (*GraphQueryResult, error) {
	if maxDepth <= 0 {
		maxDepth = 2
	}

	visited := map[string]bool{}
	var allNodes []store.GraphNodeRow
	var allEdges []store.GraphEdgeRow

	queue := []string{startNodeID}
	visited[startNodeID] = true

	// Get start node
	startNode, err := s.c.Graph.GetNodeByID(startNodeID)
	if err != nil {
		return nil, fmt.Errorf("start node not found: %w", err)
	}
	allNodes = append(allNodes, *startNode)

	for depth := 0; depth < maxDepth && len(queue) > 0; depth++ {
		var nextQueue []string
		for _, nodeID := range queue {
			edges, _ := s.c.Graph.GetEdgesFrom(nodeID, 20, nil)
			for _, e := range edges {
				allEdges = append(allEdges, e)
				if !visited[e.TargetNodeID] {
					visited[e.TargetNodeID] = true
					if n, err := s.c.Graph.GetNodeByID(e.TargetNodeID); err == nil {
						allNodes = append(allNodes, *n)
						nextQueue = append(nextQueue, e.TargetNodeID)
					}
				}
			}
			edgesTo, _ := s.c.Graph.GetEdgesTo(nodeID, 20, nil)
			for _, e := range edgesTo {
				allEdges = append(allEdges, e)
				if !visited[e.SourceNodeID] {
					visited[e.SourceNodeID] = true
					if n, err := s.c.Graph.GetNodeByID(e.SourceNodeID); err == nil {
						allNodes = append(allNodes, *n)
						nextQueue = append(nextQueue, e.SourceNodeID)
					}
				}
			}
		}
		queue = nextQueue
	}

	return &GraphQueryResult{Nodes: allNodes, Edges: allEdges}, nil
}

// BlastRadius returns the file names structurally related to the given file
// within maxDepth hops in the knowledge graph — the set of files that matter
// *now* when this one is being edited (Phase 4). It is a relevance SIGNAL for
// injection selection, not a navigation feature: memories about these files get
// boosted. Pure-Go: it reuses the existing LLM-extracted graph rather than a
// tree-sitter parser (no CGO-free binding exists — see lesson). Returns nil when
// the file isn't in the graph yet (cold/graph-less repos degrade to no boost).
func (s *GraphService) BlastRadius(fileName string, maxDepth int) ([]string, error) {
	if fileName == "" {
		return nil, nil
	}
	node, err := s.c.Graph.FindNodeByName("file", fileName)
	if err != nil || node == nil {
		return nil, nil // not in graph — no blast radius, no boost
	}
	res, err := s.Query(node.ID, maxDepth)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(res.Nodes))
	for _, n := range res.Nodes {
		if n.Type == "file" && !strings.EqualFold(n.Name, fileName) {
			out = append(out, n.Name)
		}
	}
	return out, nil
}

// AllNodes returns all graph nodes (up to limit).
func (s *GraphService) AllNodes(limit int) ([]store.GraphNodeRow, error) {
	return s.c.Graph.ListNodes("", limit, 0)
}

// AllEdges returns all graph edges (up to limit).
func (s *GraphService) AllEdges(limit int) ([]store.GraphEdgeRow, error) {
	rows, err := s.c.Graph.ListEdges(limit)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// Stats returns node and edge counts by type.
func (s *GraphService) Stats() (map[string]any, error) {
	nodeCounts, err := s.c.Graph.CountNodes()
	if err != nil {
		return nil, err
	}
	edgeCounts, err := s.c.Graph.CountEdges()
	if err != nil {
		return nil, err
	}

	totalNodes := 0
	for _, c := range nodeCounts {
		totalNodes += c
	}
	totalEdges := 0
	for _, c := range edgeCounts {
		totalEdges += c
	}

	return map[string]any{
		"totalNodes":  totalNodes,
		"totalEdges":  totalEdges,
		"nodesByType": nodeCounts,
		"edgesByType": edgeCounts,
	}, nil
}

// CreateRelation manually creates a relation between two nodes.
func (s *GraphService) CreateRelation(sourceNodeID, targetNodeID, relType string, weight float64) (*store.GraphEdgeRow, error) {
	if weight <= 0 || weight > 1 {
		weight = 0.5
	}
	edgeID := "ge_" + uuid.New().String()[:8]
	edge := &store.GraphEdgeRow{
		ID:           edgeID,
		Type:         relType,
		SourceNodeID: sourceNodeID,
		TargetNodeID: targetNodeID,
		Weight:       weight,
		IsLatest:     1,
		Version:      1,
	}
	if err := s.c.Graph.CreateEdge(edge); err != nil {
		return nil, err
	}
	return edge, nil
}
