package store

import (
	"encoding/json"
	"testing"
)

// ---------------------------------------------------------------------------
// Node tests
// ---------------------------------------------------------------------------

func TestGraphStore_CreateNodeAndGetByID(t *testing.T) {
	db := setupTestDB(t)
	gs := NewGraphStore(db)

	node := &GraphNodeRow{
		ID:         "gn_test1",
		Type:       "file",
		Name:       "main.go",
		Properties: json.RawMessage(`{"lang":"go"}`),
		Aliases:    json.RawMessage(`["entry"]`),
	}

	if err := gs.CreateNode(node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	got, err := gs.GetNodeByID("gn_test1")
	if err != nil {
		t.Fatalf("GetNodeByID: %v", err)
	}

	if got.ID != "gn_test1" {
		t.Errorf("expected ID gn_test1, got %s", got.ID)
	}
	if got.Type != "file" {
		t.Errorf("expected type file, got %s", got.Type)
	}
	if got.Name != "main.go" {
		t.Errorf("expected name main.go, got %s", got.Name)
	}
	if string(got.Properties) != `{"lang":"go"}` {
		t.Errorf("unexpected properties: %s", got.Properties)
	}
	if string(got.Aliases) != `["entry"]` {
		t.Errorf("unexpected aliases: %s", got.Aliases)
	}
	if got.CreatedAt == "" {
		t.Error("expected CreatedAt to be set")
	}
}

func TestGraphStore_FindNodeByName(t *testing.T) {
	db := setupTestDB(t)
	gs := NewGraphStore(db)

	node := &GraphNodeRow{
		ID:   "gn_find1",
		Type: "function",
		Name: "HandleRequest",
	}
	if err := gs.CreateNode(node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Case-insensitive search.
	got, err := gs.FindNodeByName("function", "handlerequest")
	if err != nil {
		t.Fatalf("FindNodeByName: %v", err)
	}
	if got.ID != "gn_find1" {
		t.Errorf("expected ID gn_find1, got %s", got.ID)
	}
	if got.Name != "HandleRequest" {
		t.Errorf("expected original name HandleRequest, got %s", got.Name)
	}

	// Wrong type should not find.
	_, err = gs.FindNodeByName("file", "HandleRequest")
	if err == nil {
		t.Error("expected error when searching with wrong type")
	}
}

func TestGraphStore_ListNodes(t *testing.T) {
	db := setupTestDB(t)
	gs := NewGraphStore(db)

	nodes := []*GraphNodeRow{
		{ID: "gn_l1", Type: "file", Name: "a.go"},
		{ID: "gn_l2", Type: "function", Name: "main"},
		{ID: "gn_l3", Type: "file", Name: "b.go"},
	}
	for _, n := range nodes {
		if err := gs.CreateNode(n); err != nil {
			t.Fatalf("CreateNode %s: %v", n.ID, err)
		}
	}

	// List all.
	all, err := gs.ListNodes("", 100, 0)
	if err != nil {
		t.Fatalf("ListNodes all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(all))
	}

	// List by type.
	files, err := gs.ListNodes("file", 100, 0)
	if err != nil {
		t.Fatalf("ListNodes file: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 file nodes, got %d", len(files))
	}

	funcs, err := gs.ListNodes("function", 100, 0)
	if err != nil {
		t.Fatalf("ListNodes function: %v", err)
	}
	if len(funcs) != 1 {
		t.Errorf("expected 1 function node, got %d", len(funcs))
	}
}

func TestGraphStore_DeleteNode(t *testing.T) {
	db := setupTestDB(t)
	gs := NewGraphStore(db)

	// Create two nodes and an edge between them.
	nodeA := &GraphNodeRow{ID: "gn_da", Type: "file", Name: "a.go"}
	nodeB := &GraphNodeRow{ID: "gn_db", Type: "file", Name: "b.go"}
	if err := gs.CreateNode(nodeA); err != nil {
		t.Fatalf("CreateNode A: %v", err)
	}
	if err := gs.CreateNode(nodeB); err != nil {
		t.Fatalf("CreateNode B: %v", err)
	}

	edge := &GraphEdgeRow{
		ID:           "ge_d1",
		Type:         "imports",
		SourceNodeID: "gn_da",
		TargetNodeID: "gn_db",
		Weight:       0.8,
		IsLatest:     1,
		Version:      1,
	}
	if err := gs.CreateEdge(edge); err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}

	// Delete node A — should also remove the edge.
	if err := gs.DeleteNode("gn_da"); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}

	// Node A should be gone.
	_, err := gs.GetNodeByID("gn_da")
	if err == nil {
		t.Error("expected error getting deleted node")
	}

	// Edge should be gone too.
	_, err = gs.GetEdgeByID("ge_d1")
	if err == nil {
		t.Error("expected error getting edge after node deletion")
	}

	// Node B should still exist.
	got, err := gs.GetNodeByID("gn_db")
	if err != nil {
		t.Fatalf("GetNodeByID B after delete: %v", err)
	}
	if got.ID != "gn_db" {
		t.Errorf("expected node B to still exist, got %s", got.ID)
	}
}

func TestGraphStore_CountNodes(t *testing.T) {
	db := setupTestDB(t)
	gs := NewGraphStore(db)

	nodes := []*GraphNodeRow{
		{ID: "gn_c1", Type: "file", Name: "x.go"},
		{ID: "gn_c2", Type: "file", Name: "y.go"},
		{ID: "gn_c3", Type: "concept", Name: "concurrency"},
	}
	for _, n := range nodes {
		if err := gs.CreateNode(n); err != nil {
			t.Fatalf("CreateNode %s: %v", n.ID, err)
		}
	}

	counts, err := gs.CountNodes()
	if err != nil {
		t.Fatalf("CountNodes: %v", err)
	}

	if counts["file"] != 2 {
		t.Errorf("expected 2 file nodes, got %d", counts["file"])
	}
	if counts["concept"] != 1 {
		t.Errorf("expected 1 concept node, got %d", counts["concept"])
	}
}

// ---------------------------------------------------------------------------
// Edge tests
// ---------------------------------------------------------------------------

func TestGraphStore_CreateEdgeAndGetByID(t *testing.T) {
	db := setupTestDB(t)
	gs := NewGraphStore(db)

	// Create two nodes first.
	gs.CreateNode(&GraphNodeRow{ID: "gn_ea", Type: "file", Name: "a.go"})
	gs.CreateNode(&GraphNodeRow{ID: "gn_eb", Type: "function", Name: "main"})

	edge := &GraphEdgeRow{
		ID:           "ge_test1",
		Type:         "uses",
		SourceNodeID: "gn_ea",
		TargetNodeID: "gn_eb",
		Weight:       0.9,
		IsLatest:     1,
		Version:      1,
		Context:      json.RawMessage(`{"scope":"local"}`),
	}
	if err := gs.CreateEdge(edge); err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}

	got, err := gs.GetEdgeByID("ge_test1")
	if err != nil {
		t.Fatalf("GetEdgeByID: %v", err)
	}

	if got.ID != "ge_test1" {
		t.Errorf("expected ID ge_test1, got %s", got.ID)
	}
	if got.Type != "uses" {
		t.Errorf("expected type uses, got %s", got.Type)
	}
	if got.SourceNodeID != "gn_ea" {
		t.Errorf("expected source gn_ea, got %s", got.SourceNodeID)
	}
	if got.TargetNodeID != "gn_eb" {
		t.Errorf("expected target gn_eb, got %s", got.TargetNodeID)
	}
	if got.Weight != 0.9 {
		t.Errorf("expected weight 0.9, got %f", got.Weight)
	}
	if got.IsLatest != 1 {
		t.Errorf("expected isLatest 1, got %d", got.IsLatest)
	}
	if got.Version != 1 {
		t.Errorf("expected version 1, got %d", got.Version)
	}
	if string(got.Context) != `{"scope":"local"}` {
		t.Errorf("unexpected context: %s", got.Context)
	}
	if got.CreatedAt == "" {
		t.Error("expected CreatedAt to be set")
	}
}

func TestGraphStore_GetEdgesFromAndTo(t *testing.T) {
	db := setupTestDB(t)
	gs := NewGraphStore(db)

	// Create 3 nodes: A, B, C.
	gs.CreateNode(&GraphNodeRow{ID: "gn_a", Type: "file", Name: "a.go"})
	gs.CreateNode(&GraphNodeRow{ID: "gn_b", Type: "file", Name: "b.go"})
	gs.CreateNode(&GraphNodeRow{ID: "gn_c", Type: "file", Name: "c.go"})

	// Edges: A->B, A->C.
	gs.CreateEdge(&GraphEdgeRow{
		ID: "ge_ab", Type: "imports", SourceNodeID: "gn_a", TargetNodeID: "gn_b",
		Weight: 0.8, IsLatest: 1, Version: 1,
	})
	gs.CreateEdge(&GraphEdgeRow{
		ID: "ge_ac", Type: "imports", SourceNodeID: "gn_a", TargetNodeID: "gn_c",
		Weight: 0.7, IsLatest: 1, Version: 1,
	})

	// GetEdgesFrom(A) should return 2.
	fromA, err := gs.GetEdgesFrom("gn_a", 100, nil)
	if err != nil {
		t.Fatalf("GetEdgesFrom: %v", err)
	}
	if len(fromA) != 2 {
		t.Errorf("expected 2 edges from A, got %d", len(fromA))
	}

	// GetEdgesTo(B) should return 1.
	toB, err := gs.GetEdgesTo("gn_b", 100, nil)
	if err != nil {
		t.Fatalf("GetEdgesTo: %v", err)
	}
	if len(toB) != 1 {
		t.Errorf("expected 1 edge to B, got %d", len(toB))
	}
	if toB[0].SourceNodeID != "gn_a" {
		t.Errorf("expected edge source gn_a, got %s", toB[0].SourceNodeID)
	}

	// GetEdgesFrom(B) should return 0 (no outgoing edges from B).
	fromB, err := gs.GetEdgesFrom("gn_b", 100, nil)
	if err != nil {
		t.Fatalf("GetEdgesFrom B: %v", err)
	}
	if len(fromB) != 0 {
		t.Errorf("expected 0 edges from B, got %d", len(fromB))
	}
}

func TestGraphStore_GetNeighbors(t *testing.T) {
	db := setupTestDB(t)
	gs := NewGraphStore(db)

	// A->B, C->A.
	gs.CreateNode(&GraphNodeRow{ID: "gn_na", Type: "file", Name: "a.go"})
	gs.CreateNode(&GraphNodeRow{ID: "gn_nb", Type: "function", Name: "main"})
	gs.CreateNode(&GraphNodeRow{ID: "gn_nc", Type: "concept", Name: "startup"})

	gs.CreateEdge(&GraphEdgeRow{
		ID: "ge_nab", Type: "uses", SourceNodeID: "gn_na", TargetNodeID: "gn_nb",
		Weight: 0.9, IsLatest: 1, Version: 1,
	})
	gs.CreateEdge(&GraphEdgeRow{
		ID: "ge_nca", Type: "related_to", SourceNodeID: "gn_nc", TargetNodeID: "gn_na",
		Weight: 0.7, IsLatest: 1, Version: 1,
	})

	neighbors, err := gs.GetNeighbors("gn_na", 50, nil)
	if err != nil {
		t.Fatalf("GetNeighbors: %v", err)
	}
	if len(neighbors) != 2 {
		t.Fatalf("expected 2 neighbors of A, got %d", len(neighbors))
	}

	// Verify both B and C are returned.
	ids := map[string]bool{}
	for _, n := range neighbors {
		ids[n.ID] = true
	}
	if !ids["gn_nb"] {
		t.Error("expected neighbor B (gn_nb)")
	}
	if !ids["gn_nc"] {
		t.Error("expected neighbor C (gn_nc)")
	}
}

func TestGraphStore_CountEdges(t *testing.T) {
	db := setupTestDB(t)
	gs := NewGraphStore(db)

	gs.CreateNode(&GraphNodeRow{ID: "gn_ce1", Type: "file", Name: "x.go"})
	gs.CreateNode(&GraphNodeRow{ID: "gn_ce2", Type: "file", Name: "y.go"})
	gs.CreateNode(&GraphNodeRow{ID: "gn_ce3", Type: "function", Name: "foo"})

	gs.CreateEdge(&GraphEdgeRow{
		ID: "ge_ce1", Type: "imports", SourceNodeID: "gn_ce1", TargetNodeID: "gn_ce2",
		Weight: 0.5, IsLatest: 1, Version: 1,
	})
	gs.CreateEdge(&GraphEdgeRow{
		ID: "ge_ce2", Type: "uses", SourceNodeID: "gn_ce1", TargetNodeID: "gn_ce3",
		Weight: 0.8, IsLatest: 1, Version: 1,
	})
	gs.CreateEdge(&GraphEdgeRow{
		ID: "ge_ce3", Type: "uses", SourceNodeID: "gn_ce2", TargetNodeID: "gn_ce3",
		Weight: 0.6, IsLatest: 1, Version: 1,
	})

	counts, err := gs.CountEdges()
	if err != nil {
		t.Fatalf("CountEdges: %v", err)
	}

	if counts["imports"] != 1 {
		t.Errorf("expected 1 imports edge, got %d", counts["imports"])
	}
	if counts["uses"] != 2 {
		t.Errorf("expected 2 uses edges, got %d", counts["uses"])
	}
}
