package pipeline

import (
	"context"
	"testing"
	"time"

	"imprint/internal/extract"
	"imprint/internal/llm"
	"imprint/internal/store"
)

// countingProvider records whether Complete was called, so we can assert the
// Phase 3 gate skips the LLM on trivial observations.
type countingProvider struct{ calls int }

func (p *countingProvider) Name() string    { return "counting" }
func (p *countingProvider) Available() bool { return true }
func (p *countingProvider) Complete(_ context.Context, _ llm.CompletionRequest) (string, error) {
	p.calls++
	return "<type>fact</type><title>x</title><importance>5</importance>", nil
}

func ptr(s string) *string { return &s }

func TestImportance_TrivialSkipsLLM(t *testing.T) {
	prov := &countingProvider{}
	c := NewCompressor(prov, "hybrid")
	c.SetImportanceFilter(true, 4)

	// A tiny read-only navigation observation — clearly trivial.
	raw := &store.RawObservationRow{
		ID: "obs1", SessionID: "s1", Timestamp: time.Now(),
		ToolName:   ptr("Read"),
		ToolInput:  []byte(`"ls"`),
		ToolOutput: []byte(`"a.txt"`),
	}
	out, err := c.Compress(context.Background(), raw)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	if prov.calls != 0 {
		t.Errorf("expected 0 LLM calls for trivial obs, got %d", prov.calls)
	}
	if out == nil || out.SourceObservationID == nil || *out.SourceObservationID != "obs1" {
		t.Errorf("expected a deterministic base observation linked to source, got %+v", out)
	}
	if out.Importance >= 4 {
		t.Errorf("expected low importance for trivial obs, got %d", out.Importance)
	}
}

func TestImportance_HighSignalUsesLLM(t *testing.T) {
	prov := &countingProvider{}
	c := NewCompressor(prov, "hybrid")
	c.SetImportanceFilter(true, 4)

	// An edit with an error in the output — high signal, must reach the LLM.
	raw := &store.RawObservationRow{
		ID: "obs2", SessionID: "s1", Timestamp: time.Now(),
		ToolName:   ptr("Edit"),
		ToolInput:  []byte(`"internal/store/db.go"`),
		ToolOutput: []byte(`"panic: runtime error: nil pointer dereference at db.go:42"`),
	}
	if _, err := c.Compress(context.Background(), raw); err != nil {
		t.Fatalf("Compress: %v", err)
	}
	if prov.calls != 1 {
		t.Errorf("expected 1 LLM call for high-signal obs, got %d", prov.calls)
	}
}

func TestImportance_Scoring(t *testing.T) {
	// Errors + edit score high; read-only tiny output scores low.
	hi := ScoreImportance("Edit", "file.go", "error: boom", extract.Result{Errors: []string{"error: boom"}, Files: []string{"file.go"}})
	if hi < 7 {
		t.Errorf("expected high score for edit+error, got %d", hi)
	}
	lo := ScoreImportance("Read", "", "ok", extract.Result{})
	if lo >= 4 {
		t.Errorf("expected low score for trivial read, got %d", lo)
	}
}
