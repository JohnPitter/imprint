package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"imprint/internal/pipeline"
	"imprint/internal/store"
	"imprint/internal/types"
)

// fakeRooter is a deterministic stand-in for the LLM-backed Rooter so the
// convergence/contradiction policy can be tested without a provider.
type fakeRooter struct {
	converge   bool
	contradict bool
}

func (f *fakeRooter) Synthesize(_ context.Context, _, _ string, _ []store.MemoryRow) (*pipeline.RootedDraft, error) {
	if !f.converge {
		return nil, nil
	}
	return &pipeline.RootedDraft{Statement: "simplicity beats sophisticated patterns", Concepts: []string{"simplicity"}}, nil
}

func (f *fakeRooter) Contradicts(_ context.Context, _, _ string, _ store.MemoryRow) (bool, string, error) {
	return f.contradict, "observed the opposite", nil
}

func jsonArr(items ...string) json.RawMessage {
	b, _ := json.Marshal(items)
	return b
}

func seedConvergentMemories(t *testing.T, c *Container) {
	t.Helper()
	// Two distinct sessions in the same project — episodic dispersion.
	for _, sid := range []string{"s1", "s2"} {
		if err := c.Sessions.Create(&store.SessionRow{
			ID: sid, Project: "p1", Cwd: "/tmp", StartedAt: time.Now(), Status: types.SessionStatus("active"),
		}); err != nil {
			t.Fatalf("create session: %v", err)
		}
	}
	// Four refined memories converging on the "simplicity" concept.
	mems := []struct {
		id, sid string
	}{{"m1", "s1"}, {"m2", "s1"}, {"m3", "s2"}, {"m4", "s2"}}
	for _, m := range mems {
		if err := c.Memories.Create(&store.MemoryRow{
			ID: m.id, Type: "architecture", Title: "keep it simple " + m.id,
			Content:    "chose the simple option",
			Concepts:   jsonArr("simplicity"),
			SessionIDs: jsonArr(m.sid),
			Strength:   7, IsLatest: 1,
		}); err != nil {
			t.Fatalf("create memory: %v", err)
		}
	}
}

func intuitionCfg() IntuitionConfig {
	return IntuitionConfig{MinStrength: 6, MinConvergence: 4, MinSessions: 2, MaxActive: 5, ContradictionHit: 2, DemoteFloor: 3}
}

func TestIntuition_RootsOnConvergence(t *testing.T) {
	c := setupTestContainer(t)
	seedConvergentMemories(t, c)
	svc := NewIntuitionService(c, &fakeRooter{converge: true}, intuitionCfg())

	created, err := svc.DetectAndRoot(context.Background(), "p1")
	if err != nil {
		t.Fatalf("DetectAndRoot: %v", err)
	}
	if len(created) != 1 {
		t.Fatalf("expected 1 rooted intuition, got %d", len(created))
	}
	if created[0].EvidenceCount != 4 {
		t.Errorf("expected 4 evidence memories, got %d", created[0].EvidenceCount)
	}
	active, _ := c.Intuitions.ListActive("p1", 10)
	if len(active) != 1 {
		t.Fatalf("expected 1 active intuition, got %d", len(active))
	}
}

func TestIntuition_DoesNotRootWithoutConvergence(t *testing.T) {
	c := setupTestContainer(t)
	seedConvergentMemories(t, c)
	// Rooter says it does not converge → nothing is rooted (high bar).
	svc := NewIntuitionService(c, &fakeRooter{converge: false}, intuitionCfg())
	created, err := svc.DetectAndRoot(context.Background(), "p1")
	if err != nil {
		t.Fatalf("DetectAndRoot: %v", err)
	}
	if len(created) != 0 {
		t.Fatalf("expected 0 rooted (no convergence), got %d", len(created))
	}
}

func TestIntuition_BelowBarDoesNotRoot(t *testing.T) {
	c := setupTestContainer(t)
	// Only two memories — below MinConvergence=4.
	if err := c.Sessions.Create(&store.SessionRow{ID: "s1", Project: "p1", StartedAt: time.Now(), Status: "active"}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"m1", "m2"} {
		_ = c.Memories.Create(&store.MemoryRow{ID: id, Type: "architecture", Title: id, Concepts: jsonArr("simplicity"), SessionIDs: jsonArr("s1"), Strength: 7, IsLatest: 1})
	}
	svc := NewIntuitionService(c, &fakeRooter{converge: true}, intuitionCfg())
	created, _ := svc.DetectAndRoot(context.Background(), "p1")
	if len(created) != 0 {
		t.Fatalf("expected 0 rooted (below convergence bar), got %d", len(created))
	}
}

func TestIntuition_ContradictionWeakensAndDemotes(t *testing.T) {
	c := setupTestContainer(t)
	seedConvergentMemories(t, c)
	svc := NewIntuitionService(c, &fakeRooter{converge: true, contradict: true}, intuitionCfg())
	if _, err := svc.DetectAndRoot(context.Background(), "p1"); err != nil {
		t.Fatal(err)
	}

	mems, _ := c.Memories.ListRefinedByProject("p1", 6, 10)
	// First pass: one contradiction (strength 6 → 4), logged for that pair.
	if err := svc.CheckContradictions(context.Background(), "p1", mems); err != nil {
		t.Fatalf("CheckContradictions: %v", err)
	}
	active, _ := c.Intuitions.ListAll("p1", 10)
	if len(active) != 1 || active[0].Strength != 4 {
		t.Fatalf("expected strength 4 after one contradiction, got %+v", active)
	}
	if active[0].ContradictionCount != 1 {
		t.Errorf("expected contradiction_count 1, got %d", active[0].ContradictionCount)
	}

	// Second pass: a different (unjudged) memory contradicts → 4 → 2 ≤ floor → demoted.
	if err := svc.CheckContradictions(context.Background(), "p1", mems); err != nil {
		t.Fatalf("CheckContradictions 2: %v", err)
	}
	all, _ := c.Intuitions.ListAll("p1", 10)
	if all[0].Status != "demoted" {
		t.Errorf("expected demoted after crossing floor, got status=%q strength=%.1f", all[0].Status, all[0].Strength)
	}
	// Demoted intuition no longer injected.
	if act, _ := c.Intuitions.ListActive("p1", 10); len(act) != 0 {
		t.Errorf("expected 0 active after demotion, got %d", len(act))
	}
}
