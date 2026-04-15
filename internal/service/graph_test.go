package service

import (
	"testing"

	"imprint/internal/store"
)

// ---------------------------------------------------------------------------
// GraphService
// ---------------------------------------------------------------------------

func TestGraphService_Stats(t *testing.T) {
	c := setupTestContainer(t)

	// Create nodes and edges directly via the store.
	c.Graph.CreateNode(&store.GraphNodeRow{ID: "gn_s1", Type: "file", Name: "a.go"})
	c.Graph.CreateNode(&store.GraphNodeRow{ID: "gn_s2", Type: "file", Name: "b.go"})
	c.Graph.CreateNode(&store.GraphNodeRow{ID: "gn_s3", Type: "concept", Name: "testing"})

	c.Graph.CreateEdge(&store.GraphEdgeRow{
		ID: "ge_s1", Type: "imports", SourceNodeID: "gn_s1", TargetNodeID: "gn_s2",
		Weight: 0.8, IsLatest: 1, Version: 1,
	})
	c.Graph.CreateEdge(&store.GraphEdgeRow{
		ID: "ge_s2", Type: "related_to", SourceNodeID: "gn_s1", TargetNodeID: "gn_s3",
		Weight: 0.6, IsLatest: 1, Version: 1,
	})

	svc := NewGraphService(c, nil)
	stats, err := svc.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}

	if stats["totalNodes"].(int) != 3 {
		t.Errorf("expected totalNodes 3, got %v", stats["totalNodes"])
	}
	if stats["totalEdges"].(int) != 2 {
		t.Errorf("expected totalEdges 2, got %v", stats["totalEdges"])
	}

	nodesByType := stats["nodesByType"].(map[string]int)
	if nodesByType["file"] != 2 {
		t.Errorf("expected 2 file nodes, got %d", nodesByType["file"])
	}
	if nodesByType["concept"] != 1 {
		t.Errorf("expected 1 concept node, got %d", nodesByType["concept"])
	}

	edgesByType := stats["edgesByType"].(map[string]int)
	if edgesByType["imports"] != 1 {
		t.Errorf("expected 1 imports edge, got %d", edgesByType["imports"])
	}
	if edgesByType["related_to"] != 1 {
		t.Errorf("expected 1 related_to edge, got %d", edgesByType["related_to"])
	}
}

func TestGraphService_Query_BFS(t *testing.T) {
	c := setupTestContainer(t)

	// Create chain: A -> B -> C.
	c.Graph.CreateNode(&store.GraphNodeRow{ID: "gn_qa", Type: "file", Name: "a.go"})
	c.Graph.CreateNode(&store.GraphNodeRow{ID: "gn_qb", Type: "function", Name: "main"})
	c.Graph.CreateNode(&store.GraphNodeRow{ID: "gn_qc", Type: "concept", Name: "startup"})

	c.Graph.CreateEdge(&store.GraphEdgeRow{
		ID: "ge_qab", Type: "uses", SourceNodeID: "gn_qa", TargetNodeID: "gn_qb",
		Weight: 0.9, IsLatest: 1, Version: 1,
	})
	c.Graph.CreateEdge(&store.GraphEdgeRow{
		ID: "ge_qbc", Type: "related_to", SourceNodeID: "gn_qb", TargetNodeID: "gn_qc",
		Weight: 0.7, IsLatest: 1, Version: 1,
	})

	svc := NewGraphService(c, nil)
	result, err := svc.Query("gn_qa", 2)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	if len(result.Nodes) != 3 {
		t.Errorf("expected 3 nodes in BFS result, got %d", len(result.Nodes))
	}
	// BFS traverses both directions: A->B (from A), B->C and B<-A (from B) = 3 edges total.
	if len(result.Edges) != 3 {
		t.Errorf("expected 3 edges in BFS result, got %d", len(result.Edges))
	}

	// Verify all node IDs are present.
	nodeIDs := map[string]bool{}
	for _, n := range result.Nodes {
		nodeIDs[n.ID] = true
	}
	for _, id := range []string{"gn_qa", "gn_qb", "gn_qc"} {
		if !nodeIDs[id] {
			t.Errorf("expected node %s in BFS result", id)
		}
	}
}

func TestGraphService_CreateRelation(t *testing.T) {
	c := setupTestContainer(t)

	// Create two nodes.
	c.Graph.CreateNode(&store.GraphNodeRow{ID: "gn_ra", Type: "file", Name: "handler.go"})
	c.Graph.CreateNode(&store.GraphNodeRow{ID: "gn_rb", Type: "function", Name: "ServeHTTP"})

	svc := NewGraphService(c, nil)
	edge, err := svc.CreateRelation("gn_ra", "gn_rb", "uses", 0.85)
	if err != nil {
		t.Fatalf("CreateRelation: %v", err)
	}

	if edge.Type != "uses" {
		t.Errorf("expected edge type uses, got %s", edge.Type)
	}
	if edge.SourceNodeID != "gn_ra" {
		t.Errorf("expected source gn_ra, got %s", edge.SourceNodeID)
	}
	if edge.TargetNodeID != "gn_rb" {
		t.Errorf("expected target gn_rb, got %s", edge.TargetNodeID)
	}
	if edge.Weight != 0.85 {
		t.Errorf("expected weight 0.85, got %f", edge.Weight)
	}
	if edge.IsLatest != 1 {
		t.Errorf("expected isLatest 1, got %d", edge.IsLatest)
	}

	// Verify edge was persisted.
	got, err := c.Graph.GetEdgeByID(edge.ID)
	if err != nil {
		t.Fatalf("GetEdgeByID after CreateRelation: %v", err)
	}
	if got.Type != "uses" {
		t.Errorf("persisted edge type mismatch: %s", got.Type)
	}
}
