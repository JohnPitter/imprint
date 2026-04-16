package pipeline

import (
	"context"
	"testing"
	"time"

	"imprint/internal/store"
)

const mockGraphExtractResponse = `<graph>
  <nodes>
    <node><type>file</type><name>app.go</name></node>
    <node><type>function</type><name>main</name></node>
    <node><type>concept</type><name>startup</name></node>
  </nodes>
  <edges>
    <edge><source>app.go</source><target>main</target><type>uses</type><weight>0.9</weight></edge>
    <edge><source>main</source><target>startup</target><type>related_to</type><weight>0.7</weight></edge>
  </edges>
</graph>`

func TestGraphExtractor_Extract(t *testing.T) {
	mock := &mockLLMProvider{response: mockGraphExtractResponse}
	extractor := NewGraphExtractor(mock)

	narrative := "Application startup with database init."
	obs := &store.CompressedObservationRow{
		ID:        "cobs-graph-1",
		SessionID: "sess-001",
		Timestamp: time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC),
		Type:      "file_operation",
		Title:     "Edit app.go",
		Narrative: &narrative,
		Files:     []string{"app.go"},
		Concepts:  []string{"go", "startup"},
	}

	result, err := extractor.Extract(context.Background(), obs)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	// Verify nodes.
	if len(result.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(result.Nodes))
	}

	expectedNodes := []struct {
		Type string
		Name string
	}{
		{"file", "app.go"},
		{"function", "main"},
		{"concept", "startup"},
	}
	for i, en := range expectedNodes {
		if result.Nodes[i].Type != en.Type {
			t.Errorf("node %d: expected type %q, got %q", i, en.Type, result.Nodes[i].Type)
		}
		if result.Nodes[i].Name != en.Name {
			t.Errorf("node %d: expected name %q, got %q", i, en.Name, result.Nodes[i].Name)
		}
	}

	// Verify edges.
	if len(result.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(result.Edges))
	}

	if result.Edges[0].Source != "app.go" {
		t.Errorf("edge 0: expected source app.go, got %s", result.Edges[0].Source)
	}
	if result.Edges[0].Target != "main" {
		t.Errorf("edge 0: expected target main, got %s", result.Edges[0].Target)
	}
	if result.Edges[0].Type != "uses" {
		t.Errorf("edge 0: expected type uses, got %s", result.Edges[0].Type)
	}
	if result.Edges[0].Weight != 0.9 {
		t.Errorf("edge 0: expected weight 0.9, got %f", result.Edges[0].Weight)
	}

	if result.Edges[1].Source != "main" {
		t.Errorf("edge 1: expected source main, got %s", result.Edges[1].Source)
	}
	if result.Edges[1].Target != "startup" {
		t.Errorf("edge 1: expected target startup, got %s", result.Edges[1].Target)
	}
	if result.Edges[1].Type != "related_to" {
		t.Errorf("edge 1: expected type related_to, got %s", result.Edges[1].Type)
	}
	if result.Edges[1].Weight != 0.7 {
		t.Errorf("edge 1: expected weight 0.7, got %f", result.Edges[1].Weight)
	}

	if mock.calls.Load() != 1 {
		t.Errorf("expected 1 LLM call, got %d", mock.calls.Load())
	}
}

func TestGraphExtractor_Extract_EmptyResponse(t *testing.T) {
	mock := &mockLLMProvider{response: "No entities found in this observation."}
	extractor := NewGraphExtractor(mock)

	obs := &store.CompressedObservationRow{
		ID:        "cobs-graph-2",
		SessionID: "sess-002",
		Timestamp: time.Date(2026, 4, 13, 11, 0, 0, 0, time.UTC),
		Type:      "notification",
		Title:     "Build succeeded",
		Files:     []string{},
		Concepts:  []string{},
	}

	result, err := extractor.Extract(context.Background(), obs)
	if err != nil {
		t.Fatalf("Extract with empty response should not error: %v", err)
	}

	if len(result.Nodes) != 0 {
		t.Errorf("expected 0 nodes for empty response, got %d", len(result.Nodes))
	}
	if len(result.Edges) != 0 {
		t.Errorf("expected 0 edges for empty response, got %d", len(result.Edges))
	}

	if mock.calls.Load() != 1 {
		t.Errorf("expected 1 LLM call, got %d", mock.calls.Load())
	}
}
