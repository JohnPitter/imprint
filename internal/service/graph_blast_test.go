package service

import (
	"strings"
	"testing"

	"imprint/internal/store"
)

func TestBlastRadius_LazyInjectionPullsRelatedFileMemory(t *testing.T) {
	c := setupTestContainer(t)

	// Graph: a.go --imports--> b.go (file nodes + one edge).
	if err := c.Graph.CreateNode(&store.GraphNodeRow{ID: "n_a", Type: "file", Name: "a.go"}); err != nil {
		t.Fatal(err)
	}
	if err := c.Graph.CreateNode(&store.GraphNodeRow{ID: "n_b", Type: "file", Name: "b.go"}); err != nil {
		t.Fatal(err)
	}
	if err := c.Graph.CreateEdge(&store.GraphEdgeRow{
		ID: "e_ab", Type: "imports", SourceNodeID: "n_a", TargetNodeID: "n_b", IsLatest: 1, Version: 1,
	}); err != nil {
		t.Fatal(err)
	}

	// A refined memory about b.go only (no shared concept with a.go).
	if err := c.Memories.Create(&store.MemoryRow{
		ID: "m_b", Type: "architecture", Title: "b.go invariant", Content: "b.go must stay pure",
		Files: jsonArr("b.go"), Concepts: jsonArr("purity"), Strength: 8, IsLatest: 1,
	}); err != nil {
		t.Fatal(err)
	}

	graphSvc := NewGraphService(c, nil)
	ctxSvc := NewContextService(c, 2000)

	// Sanity: blast radius of a.go includes b.go.
	radius, err := graphSvc.BlastRadius("a.go", 2)
	if err != nil {
		t.Fatalf("BlastRadius: %v", err)
	}
	if len(radius) != 1 || radius[0] != "b.go" {
		t.Fatalf("expected blast radius [b.go], got %v", radius)
	}

	// Without the blast-radius signal, touching only a.go pulls nothing
	// (the memory is about b.go, no shared concept).
	blocks, _ := ctxSvc.LazyContext("s1", "p1", []string{"a.go"}, nil, 5)
	if len(blocks) != 0 {
		t.Fatalf("expected no lazy hit without blast radius, got %d blocks", len(blocks))
	}

	// With the blast-radius signal wired, touching a.go surfaces b.go's memory.
	ctxSvc.SetBlastRadius(func(file string, depth int) []string {
		r, _ := graphSvc.BlastRadius(file, depth)
		return r
	}, 2)
	blocks, err = ctxSvc.LazyContext("s1", "p1", []string{"a.go"}, nil, 5)
	if err != nil {
		t.Fatalf("LazyContext: %v", err)
	}
	if len(blocks) != 1 || !strings.Contains(blocks[0].Content, "b.go invariant") {
		t.Fatalf("expected b.go memory pulled via blast radius, got %+v", blocks)
	}
}
