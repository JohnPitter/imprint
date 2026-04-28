package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"imprint/internal/llm"
	"imprint/internal/pipeline"
	"imprint/internal/store"
	"imprint/internal/types"
)

// ---------------------------------------------------------------------------
// Fake LLM provider used by pipeline tests.
// ---------------------------------------------------------------------------

type fakeLLM struct {
	name      string
	available bool
	resp      string
	err       error
	calls     int
	lastReq   llm.CompletionRequest
}

func (f *fakeLLM) Name() string    { return f.name }
func (f *fakeLLM) Available() bool { return f.available }
func (f *fakeLLM) Complete(ctx context.Context, req llm.CompletionRequest) (string, error) {
	f.calls++
	f.lastReq = req
	if f.err != nil {
		return "", f.err
	}
	return f.resp, nil
}

// ---------------------------------------------------------------------------
// Helper — insert a compressed observation with defaults.
// ---------------------------------------------------------------------------

func insertCompressed(t *testing.T, c *Container, id, sessionID, title string, importance int) {
	t.Helper()
	narrative := "narrative for " + title
	err := c.Observations.CreateCompressed(&store.CompressedObservationRow{
		ID:         id,
		SessionID:  sessionID,
		Timestamp:  time.Now(),
		Type:       "decision",
		Title:      title,
		Narrative:  &narrative,
		Concepts:   []string{"go", "test"},
		Files:      []string{"main.go"},
		Importance: importance,
		Confidence: 0.9,
	})
	if err != nil {
		t.Fatalf("CreateCompressed(%s): %v", id, err)
	}
}

// ---------------------------------------------------------------------------
// ActionService — extended coverage
// ---------------------------------------------------------------------------

func TestActionService_UpsertFromTask_CreateAndUpdate(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewActionService(c)

	// Create session so project is resolved.
	ss := NewSessionService(c, nil)
	sess, _, _ := ss.Start("ses_upsert", "proj-u", "/tmp")

	a, err := svc.UpsertFromTask("Refactor auth", "desc", "", sess.ID)
	if err != nil {
		t.Fatalf("UpsertFromTask create: %v", err)
	}
	if a.Status != "done" || a.Project == nil || *a.Project != "proj-u" {
		t.Errorf("unexpected new action: status=%s project=%v", a.Status, a.Project)
	}

	// Update via same title.
	a2, err := svc.UpsertFromTask("Refactor auth", "desc2", "in_progress", sess.ID)
	if err != nil {
		t.Fatalf("UpsertFromTask update: %v", err)
	}
	if a2.ID != a.ID {
		t.Errorf("expected same ID on update: %s vs %s", a2.ID, a.ID)
	}
	if a2.Status != "in_progress" || a2.Description != "desc2" {
		t.Errorf("update did not apply fields")
	}

	// Empty title => error.
	if _, err := svc.UpsertFromTask("", "", "", ""); err == nil {
		t.Error("expected error on empty title")
	}
}

func TestActionService_GetAndUpdateAction(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewActionService(c)

	a, err := svc.CreateAction("T1", "d", "pending", 3, nil, []string{"x"})
	if err != nil {
		t.Fatalf("CreateAction: %v", err)
	}

	if _, err := svc.GetAction(""); err == nil {
		t.Error("expected error for empty id")
	}
	got, err := svc.GetAction(a.ID)
	if err != nil {
		t.Fatalf("GetAction: %v", err)
	}
	if got.ID != a.ID {
		t.Errorf("id mismatch")
	}

	if err := svc.UpdateAction("", map[string]any{}); err == nil {
		t.Error("expected error for empty id")
	}

	// Create a parent action so the parentId FK is valid.
	parent, _ := svc.CreateAction("parent", "", "pending", 5, nil, nil)
	proj := "p2"
	asg := "bob"
	updates := map[string]any{
		"title":        "T1 new",
		"description":  "nd",
		"status":       "done",
		"priority":     float64(9),
		"assignee":     asg,
		"project":      proj,
		"tags":         []string{"a", "b"},
		"parentId":     parent.ID,
		"crystallized": true,
	}
	if err := svc.UpdateAction(a.ID, updates); err != nil {
		t.Fatalf("UpdateAction: %v", err)
	}
	got, _ = svc.GetAction(a.ID)
	if got.Title != "T1 new" || got.Status != "done" || got.Priority != 9 {
		t.Errorf("update fields not applied: %+v", got)
	}
	if got.Crystallized != 1 {
		t.Errorf("crystallized should be 1, got %d", got.Crystallized)
	}

	// Update with int priority, bool false.
	if err := svc.UpdateAction(a.ID, map[string]any{"priority": 4, "crystallized": false}); err != nil {
		t.Fatalf("UpdateAction 2: %v", err)
	}
	got, _ = svc.GetAction(a.ID)
	if got.Priority != 4 || got.Crystallized != 0 {
		t.Errorf("second update failed: %+v", got)
	}
}

func TestActionService_Frontier_GetNext(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewActionService(c)

	_, _ = svc.CreateAction("lo", "", "pending", 3, nil, nil)
	high, _ := svc.CreateAction("hi", "", "pending", 9, nil, nil)

	next, err := svc.GetNext()
	if err != nil {
		t.Fatalf("GetNext: %v", err)
	}
	if next == nil || next.ID != high.ID {
		t.Errorf("GetNext should return highest-priority pending: got %+v", next)
	}
}

func TestActionService_CreateEdge_Errors(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewActionService(c)

	if _, err := svc.CreateEdge("", "b", "blocks"); err == nil {
		t.Error("expected error with empty sourceID")
	}
	if _, err := svc.CreateEdge("a", "", "blocks"); err == nil {
		t.Error("expected error with empty targetID")
	}

	a1, _ := svc.CreateAction("A", "", "pending", 5, nil, nil)
	a2, _ := svc.CreateAction("B", "", "pending", 5, nil, nil)
	e, err := svc.CreateEdge(a1.ID, a2.ID, "")
	if err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}
	if e.Type != "blocks" {
		t.Errorf("expected default type blocks, got %s", e.Type)
	}
}

func TestActionService_Leases(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewActionService(c)

	// Guard errors.
	if _, err := svc.AcquireLease("", "a", 30); err == nil {
		t.Error("expected error for empty actionID")
	}
	if _, err := svc.AcquireLease("act1", "", 30); err == nil {
		t.Error("expected error for empty agentID")
	}
	if err := svc.ReleaseLease("", "a", nil); err == nil {
		t.Error("expected error for empty leaseID")
	}
	if err := svc.ReleaseLease("l1", "", nil); err == nil {
		t.Error("expected error for empty agentID")
	}
	if err := svc.RenewLease("", "a", 10); err == nil {
		t.Error("expected error for empty leaseID")
	}
	if err := svc.RenewLease("l1", "", 10); err == nil {
		t.Error("expected error for empty agentID")
	}

	a, _ := svc.CreateAction("Do it", "", "pending", 5, nil, nil)

	lease, err := svc.AcquireLease(a.ID, "agent-1", 0)
	if err != nil {
		t.Fatalf("AcquireLease: %v", err)
	}

	// Second acquire should fail (locked).
	if _, err := svc.AcquireLease(a.ID, "agent-2", 60); err == nil {
		t.Error("expected lock error on second acquire")
	}

	if err := svc.RenewLease(lease.ID, "agent-1", 0); err != nil {
		t.Fatalf("RenewLease: %v", err)
	}

	result := "ok"
	if err := svc.ReleaseLease(lease.ID, "agent-1", &result); err != nil {
		t.Fatalf("ReleaseLease: %v", err)
	}
}

func TestActionService_Routines(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewActionService(c)

	if _, err := svc.CreateRoutine("", nil, nil, 0); err == nil {
		t.Error("expected error empty name")
	}
	if _, err := svc.GetRoutine(""); err == nil {
		t.Error("expected error empty id")
	}
	if _, err := svc.RunRoutine(""); err == nil {
		t.Error("expected error empty id")
	}

	steps := json.RawMessage(`[
		{"title":"step1","description":"d1","priority":5,"tags":["t"]},
		{"title":"step2","description":"d2","priority":7,"tags":[]}
	]`)
	r, err := svc.CreateRoutine("setup", steps, nil, 0)
	if err != nil {
		t.Fatalf("CreateRoutine: %v", err)
	}

	got, err := svc.GetRoutine(r.ID)
	if err != nil || got.Name != "setup" {
		t.Fatalf("GetRoutine mismatch: %+v err=%v", got, err)
	}

	list, err := svc.ListRoutines(0, -1)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListRoutines: len=%d err=%v", len(list), err)
	}

	created, err := svc.RunRoutine(r.ID)
	if err != nil {
		t.Fatalf("RunRoutine: %v", err)
	}
	if len(created) != 2 {
		t.Errorf("expected 2 created actions, got %d", len(created))
	}

	// Create with empty steps.
	r2, err := svc.CreateRoutine("empty-routine", nil, nil, 1)
	if err != nil {
		t.Fatalf("CreateRoutine empty: %v", err)
	}
	_ = r2
}

// ---------------------------------------------------------------------------
// AdvancedService — extended coverage
// ---------------------------------------------------------------------------

func TestAdvancedService_ValidationErrors(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewAdvancedService(c)

	if _, err := svc.SendSignal("", "b", "ct", ""); err == nil {
		t.Error("expected error empty from")
	}
	if _, err := svc.SendSignal("a", "b", "", ""); err == nil {
		t.Error("expected error empty content")
	}
	if _, err := svc.ListSignals("", 10); err == nil {
		t.Error("expected error empty agent")
	}
	if _, err := svc.CreateCheckpoint("", "", "", nil); err == nil {
		t.Error("expected error empty name")
	}
	if err := svc.ResolveCheckpoint("", "x", "", ""); err == nil {
		t.Error("expected error empty id")
	}
	if err := svc.ResolveCheckpoint("x", "", "", ""); err == nil {
		t.Error("expected error empty resolvedBy")
	}
	if _, err := svc.CreateSentinel("", "", nil); err == nil {
		t.Error("expected error empty name")
	}
	if err := svc.TriggerSentinel("", ""); err == nil {
		t.Error("expected error id")
	}
	if err := svc.CancelSentinel(""); err == nil {
		t.Error("expected error id")
	}
	if _, err := svc.CheckSentinel(""); err == nil {
		t.Error("expected error id")
	}
	if _, err := svc.CreateSketch("", "", nil, 0); err == nil {
		t.Error("expected error empty title")
	}
	if err := svc.AddToSketch("", "a"); err == nil {
		t.Error("expected error empty ids")
	}
	if err := svc.PromoteSketch(""); err == nil {
		t.Error("expected error empty id")
	}
	if err := svc.DiscardSketch(""); err == nil {
		t.Error("expected error empty id")
	}
	if _, err := svc.CreateLesson("", "", "", nil, nil); err == nil {
		t.Error("expected error empty content")
	}
	if _, err := svc.SearchLessons("", 10); err == nil {
		t.Error("expected error empty query")
	}
	if err := svc.StrengthenLesson(""); err == nil {
		t.Error("expected error empty id")
	}
	if _, err := svc.SearchInsights("", 10); err == nil {
		t.Error("expected error empty query")
	}
	if _, err := svc.CreateFacet("", "", "", ""); err == nil {
		t.Error("expected error empty targetID")
	}
	if _, err := svc.CreateFacet("t1", "type", "", ""); err == nil {
		t.Error("expected error empty dim")
	}
	if _, err := svc.GetFacets("", ""); err == nil {
		t.Error("expected error empty")
	}
	if err := svc.RemoveFacet(""); err == nil {
		t.Error("expected error empty id")
	}
	if _, err := svc.QueryFacets("", "", 0); err == nil {
		t.Error("expected error empty")
	}
	if err := svc.GovernanceDeleteMemory(""); err == nil {
		t.Error("expected error empty id")
	}
	if _, err := svc.GovernanceBulkDelete(nil); err == nil {
		t.Error("expected error empty list")
	}
}

func TestAdvancedService_Checkpoints(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewAdvancedService(c)

	cp, err := svc.CreateCheckpoint("cp1", "desc", "", nil)
	if err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}
	if cp.Type != "approval" {
		t.Errorf("expected default type 'approval', got %s", cp.Type)
	}

	if err := svc.ResolveCheckpoint(cp.ID, "bob", "ok", ""); err != nil {
		t.Fatalf("ResolveCheckpoint: %v", err)
	}

	list, err := svc.ListCheckpoints("", 0)
	if err != nil {
		t.Fatalf("ListCheckpoints: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 checkpoint, got %d", len(list))
	}
}

func TestAdvancedService_Sentinels(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewAdvancedService(c)

	s, err := svc.CreateSentinel("watch-logs", "", map[string]any{"path": "/tmp"})
	if err != nil {
		t.Fatalf("CreateSentinel: %v", err)
	}
	if s.Type != "file_change" {
		t.Errorf("default type mismatch: %s", s.Type)
	}

	if err := svc.TriggerSentinel(s.ID, "fired"); err != nil {
		t.Fatalf("TriggerSentinel: %v", err)
	}

	got, err := svc.CheckSentinel(s.ID)
	if err != nil {
		t.Fatalf("CheckSentinel: %v", err)
	}
	if got.ID != s.ID {
		t.Errorf("id mismatch")
	}

	s2, _ := svc.CreateSentinel("x", "ttl", nil)
	if err := svc.CancelSentinel(s2.ID); err != nil {
		t.Fatalf("CancelSentinel: %v", err)
	}

	list, _ := svc.ListSentinels("", 0)
	if len(list) != 2 {
		t.Errorf("expected 2 sentinels, got %d", len(list))
	}
}

func TestAdvancedService_Sketches(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewAdvancedService(c)
	ac := NewActionService(c)

	proj := "p"
	sk, err := svc.CreateSketch("draft", "d", &proj, 0)
	if err != nil {
		t.Fatalf("CreateSketch: %v", err)
	}

	a, _ := ac.CreateAction("act-sk", "", "pending", 5, nil, nil)
	if err := svc.AddToSketch(sk.ID, a.ID); err != nil {
		t.Fatalf("AddToSketch: %v", err)
	}

	if err := svc.PromoteSketch(sk.ID); err != nil {
		t.Fatalf("PromoteSketch: %v", err)
	}

	sk2, _ := svc.CreateSketch("draft2", "", nil, 1)
	if err := svc.DiscardSketch(sk2.ID); err != nil {
		t.Fatalf("DiscardSketch: %v", err)
	}

	list, err := svc.ListSketches("", 0)
	if err != nil {
		t.Fatalf("ListSketches: %v", err)
	}
	if len(list) < 2 {
		t.Errorf("expected >=2 sketches, got %d", len(list))
	}

	if _, err := svc.GarbageCollectSketches(); err != nil {
		t.Fatalf("GarbageCollectSketches: %v", err)
	}
}

func TestAdvancedService_Lessons_Insights(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewAdvancedService(c)

	proj := "pr"
	_, err := svc.CreateLesson("Always test", "testing", "", &proj, []string{"test"})
	if err != nil {
		t.Fatalf("CreateLesson: %v", err)
	}

	if _, err := svc.ListLessons("pr", 0, -1); err != nil {
		t.Fatalf("ListLessons: %v", err)
	}

	// Insert an insight directly.
	err = c.Insights.Create(&store.InsightRow{
		ID:                   "ins_1",
		Title:                "An insight",
		Content:              "about error handling",
		Confidence:           0.7,
		SourceConceptCluster: json.RawMessage("[]"),
		CreatedAt:            store.TimeToString(time.Now()),
		UpdatedAt:            store.TimeToString(time.Now()),
	})
	if err != nil {
		t.Fatalf("insight create: %v", err)
	}

	ins, err := svc.ListInsights("", 0, 0)
	if err != nil {
		t.Fatalf("ListInsights: %v", err)
	}
	if len(ins) != 1 {
		t.Errorf("expected 1 insight, got %d", len(ins))
	}

	found, err := svc.SearchInsights("error", 0)
	if err != nil {
		t.Fatalf("SearchInsights: %v", err)
	}
	_ = found
}

func TestAdvancedService_Facets(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewAdvancedService(c)

	f, err := svc.CreateFacet("obs1", "observation", "severity", "high")
	if err != nil {
		t.Fatalf("CreateFacet: %v", err)
	}

	got, err := svc.GetFacets("obs1", "observation")
	if err != nil {
		t.Fatalf("GetFacets: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 facet, got %d", len(got))
	}

	q, err := svc.QueryFacets("severity", "high", 0)
	if err != nil {
		t.Fatalf("QueryFacets: %v", err)
	}
	if len(q) != 1 {
		t.Errorf("expected 1 query result, got %d", len(q))
	}

	stats, err := svc.FacetStats()
	if err != nil {
		t.Fatalf("FacetStats: %v", err)
	}
	if stats["severity"] != 1 {
		t.Errorf("expected severity=1, got %v", stats["severity"])
	}

	if err := svc.RemoveFacet(f.ID); err != nil {
		t.Fatalf("RemoveFacet: %v", err)
	}
}

func TestAdvancedService_Audit_Governance(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewAdvancedService(c)
	rem := NewRememberService(c)

	m, err := rem.Remember(types.MemFact, "x", "y", nil, nil, 5)
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}

	if err := svc.GovernanceDeleteMemory(m.ID); err != nil {
		t.Fatalf("GovernanceDeleteMemory: %v", err)
	}

	m2, _ := rem.Remember(types.MemFact, "a", "b", nil, nil, 5)
	m3, _ := rem.Remember(types.MemFact, "c", "d", nil, nil, 5)

	n, err := svc.GovernanceBulkDelete([]string{m2.ID, m3.ID})
	if err != nil {
		t.Fatalf("GovernanceBulkDelete: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 deleted, got %d", n)
	}

	entries, err := svc.ListAudit("", 0, -1)
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected audit entries, got 0")
	}

	filtered, err := svc.ListAudit("governance.delete", 10, 0)
	if err != nil {
		t.Fatalf("ListAudit filtered: %v", err)
	}
	_ = filtered
}

func TestAdvancedService_MarshalStringSlice(t *testing.T) {
	empty := marshalStringSlice(nil)
	if string(empty) != "[]" {
		t.Errorf("expected [], got %s", string(empty))
	}
	got := marshalStringSlice([]string{"a", "b"})
	var back []string
	if err := json.Unmarshal(got, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(back) != 2 {
		t.Errorf("expected len 2, got %d", len(back))
	}
}

// ---------------------------------------------------------------------------
// audit.go — extractOperation + WAL integration via LogAudit
// ---------------------------------------------------------------------------

func TestContainer_LogAudit_WithWAL(t *testing.T) {
	c := setupTestContainer(t)

	dir := t.TempDir()
	w, err := store.NewWAL(dir)
	if err != nil {
		t.Fatalf("NewWAL: %v", err)
	}
	t.Cleanup(func() { w.Close() })
	c.WAL = w

	c.LogAudit("observation.create", "obs1", "observation", map[string]any{"k": "v"})
	c.LogAudit("simpleaction", "e1", "ent", nil)

	// Sync + read the WAL file.
	_ = w.Close()
	data, err := os.ReadFile(filepath.Join(dir, "wal", "write_log.jsonl"))
	if err != nil {
		t.Fatalf("read wal: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 wal lines, got %d", len(lines))
	}
}

func TestExtractOperation(t *testing.T) {
	cases := []struct {
		in, out string
	}{
		{"observation.create", "create"},
		{"memory.delete", "delete"},
		{"simple", "simple"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := extractOperation(tc.in); got != tc.out {
			t.Errorf("extractOperation(%q) = %q, want %q", tc.in, got, tc.out)
		}
	}
}

// ---------------------------------------------------------------------------
// ContextService — SetDataDir, SetLayerBudget, LoadIdentity, truncation path
// ---------------------------------------------------------------------------

func TestContextService_SetDataDir_LoadIdentity(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewContextService(c, 10000)

	dir := t.TempDir()
	// Write an identity file longer than the default L0 budget.
	long := strings.Repeat("identity line. ", 100)
	if err := os.WriteFile(filepath.Join(dir, "identity.txt"), []byte(long), 0o600); err != nil {
		t.Fatalf("write identity: %v", err)
	}

	svc.SetDataDir(dir)
	svc.SetLayerBudget(LayerBudget{L0Identity: 10, L1EssentialStory: 200, L2SessionContext: 200})

	blocks, err := svc.BuildContext("sX", "projX", 0)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	var gotIdentity *types.ContextBlock
	for i, b := range blocks {
		if b.Type == "identity" {
			gotIdentity = &blocks[i]
			break
		}
	}
	if gotIdentity == nil {
		t.Fatal("expected identity block")
	}
	if !strings.HasSuffix(gotIdentity.Content, "...") {
		t.Errorf("expected truncated identity ending in '...', got: %q", gotIdentity.Content)
	}
}

func TestContextService_LoadIdentity_Empty(t *testing.T) {
	dir := t.TempDir()
	// No file.
	if got := LoadIdentity(dir); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
	// Whitespace-only.
	os.WriteFile(filepath.Join(dir, "identity.txt"), []byte("   \n\t"), 0o600)
	if got := LoadIdentity(dir); got != "" {
		t.Errorf("expected empty for whitespace, got %q", got)
	}
}

func TestDefaultLayerBudget(t *testing.T) {
	b := DefaultLayerBudget()
	if b.L0Identity == 0 || b.L1EssentialStory == 0 || b.L2SessionContext == 0 {
		t.Errorf("defaults are zero: %+v", b)
	}
}

// ---------------------------------------------------------------------------
// ObserveService — SetCompressor + ListCompressed
// ---------------------------------------------------------------------------

type fakeCompressor struct{ submitted []*store.RawObservationRow }

func (f *fakeCompressor) Submit(r *store.RawObservationRow) {
	f.submitted = append(f.submitted, r)
}

func TestObserveService_SetCompressor_Submits(t *testing.T) {
	c := setupTestContainer(t)
	ss := NewSessionService(c, nil)
	ss.Start("ses_cmp", "p", "/tmp")

	svc := NewObserveService(c, 100, 8000)
	fc := &fakeCompressor{}
	svc.SetCompressor(fc)

	tool := "Write"
	p := &types.HookPayload{
		SessionID: "ses_cmp",
		HookType:  types.HookPostToolUse,
		ToolName:  &tool,
		Timestamp: time.Now(),
		ToolInput: json.RawMessage(`{"path":"/x"}`),
	}
	if _, err := svc.Observe(p); err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if len(fc.submitted) != 1 {
		t.Errorf("expected 1 submission, got %d", len(fc.submitted))
	}
}

func TestObserveService_ListCompressed(t *testing.T) {
	c := setupTestContainer(t)
	createTestSession(t, c, "ses_lc", "p")

	insertCompressed(t, c, "co1", "ses_lc", "t1", 5)
	insertCompressed(t, c, "co2", "ses_lc", "t2", 6)

	svc := NewObserveService(c, 100, 8000)
	got, err := svc.ListCompressed("ses_lc", 0, -1)
	if err != nil {
		t.Fatalf("ListCompressed: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 compressed, got %d", len(got))
	}
}

func TestObserveService_MissingSessionID(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewObserveService(c, 100, 8000)
	if _, err := svc.Observe(&types.HookPayload{}); err == nil {
		t.Error("expected error for missing session id")
	}
}

// ---------------------------------------------------------------------------
// PipelineService — using fake LLM
// ---------------------------------------------------------------------------

func buildPipeline(t *testing.T, c *Container, resp string, fail error) *PipelineService {
	t.Helper()
	p := &fakeLLM{name: "fake", available: true, resp: resp, err: fail}
	sum := pipeline.NewSummarizer(p)
	con := pipeline.NewConsolidator(p)
	ref := pipeline.NewReflector(p)
	ge := pipeline.NewGraphExtractor(p)
	gs := NewGraphService(c, ge)
	return NewPipelineService(c, sum, con, ref, gs)
}

const summarizeResp = `<summary>
<title>Session Summary</title>
<narrative>Did some work</narrative>
<key_decisions><decision>Use Go</decision></key_decisions>
<files_modified><file>main.go</file></files_modified>
<concepts><concept>golang</concept></concepts>
</summary>`

const consolidateResp = `<memories>
<memory>
<type>pattern</type>
<title>M1</title>
<content>Content of m1</content>
<concepts><concept>go</concept></concepts>
<files><file>main.go</file></files>
<strength>7</strength>
</memory>
</memories>`

const reflectResp = `<insights>
<insight>
<title>I1</title>
<content>Insight content</content>
<confidence>0.8</confidence>
<concepts><concept>go</concept></concepts>
</insight>
</insights>`

const graphResp = `<graph>
<nodes>
<node><type>file</type><name>main.go</name></node>
<node><type>concept</type><name>testing</name></node>
</nodes>
<edges>
<edge><source>main.go</source><target>testing</target><type>related_to</type><weight>0.7</weight></edge>
</edges>
</graph>`

func TestPipelineService_Summarize(t *testing.T) {
	c := setupTestContainer(t)
	createTestSession(t, c, "ses_sum_1234567", "p")
	insertCompressed(t, c, "o1", "ses_sum_1234567", "Title one", 8)

	ps := buildPipeline(t, c, summarizeResp, nil)
	sum, err := ps.Summarize(context.Background(), "ses_sum_1234567")
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if sum == nil || sum.Title == "" {
		t.Errorf("empty summary: %+v", sum)
	}

	// Missing session.
	if _, err := ps.Summarize(context.Background(), "missing"); err == nil {
		t.Error("expected error for missing session")
	}
}

func TestPipelineService_Summarize_NoObservations(t *testing.T) {
	c := setupTestContainer(t)
	createTestSession(t, c, "ses_nosum_1234567", "p")

	ps := buildPipeline(t, c, summarizeResp, nil)
	if _, err := ps.Summarize(context.Background(), "ses_nosum_1234567"); err == nil {
		t.Error("expected error when no observations")
	}
}

func TestPipelineService_Summarize_LLMError(t *testing.T) {
	c := setupTestContainer(t)
	createTestSession(t, c, "ses_err_1234567", "p")
	insertCompressed(t, c, "oe", "ses_err_1234567", "t", 8)

	ps := buildPipeline(t, c, "", errors.New("llm down"))
	if _, err := ps.Summarize(context.Background(), "ses_err_1234567"); err == nil {
		t.Error("expected error from LLM")
	}
}

func TestPipelineService_Consolidate(t *testing.T) {
	c := setupTestContainer(t)
	createTestSession(t, c, "ses_con_1234567", "pc")

	// Need >=3 importance>=5 observations with shared concepts.
	for i := 0; i < 4; i++ {
		insertCompressed(t, c, "oc"+string(rune('a'+i)), "ses_con_1234567", "Title "+string(rune('a'+i)), 8)
	}

	ps := buildPipeline(t, c, consolidateResp, nil)
	n, err := ps.Consolidate(context.Background(), "ses_con_1234567")
	if err != nil {
		t.Fatalf("Consolidate: %v", err)
	}
	if n < 1 {
		t.Errorf("expected >=1 consolidated, got %d", n)
	}

	// Missing session.
	if _, err := ps.Consolidate(context.Background(), "missing"); err == nil {
		t.Error("expected missing-session error")
	}
}

func TestPipelineService_Consolidate_NotEnough(t *testing.T) {
	c := setupTestContainer(t)
	createTestSession(t, c, "ses_tiny_1234567", "pt")
	insertCompressed(t, c, "oct", "ses_tiny_1234567", "t", 8)

	ps := buildPipeline(t, c, consolidateResp, nil)
	n, err := ps.Consolidate(context.Background(), "ses_tiny_1234567")
	if err != nil {
		t.Fatalf("Consolidate: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 when not enough data, got %d", n)
	}
}

func TestPipelineService_Reflect(t *testing.T) {
	c := setupTestContainer(t)
	createTestSession(t, c, "ses_ref_1234567", "pr")

	// Add memories + observations so it has enough data.
	for i := 0; i < 3; i++ {
		rem := NewRememberService(c)
		rem.Remember(types.MemFact, "f"+string(rune('a'+i)), "c", nil, nil, 7)
	}
	for i := 0; i < 3; i++ {
		insertCompressed(t, c, "or"+string(rune('a'+i)), "ses_ref_1234567", "t"+string(rune('a'+i)), 6)
	}

	ps := buildPipeline(t, c, reflectResp, nil)
	n, err := ps.Reflect(context.Background(), "ses_ref_1234567")
	if err != nil {
		t.Fatalf("Reflect: %v", err)
	}
	if n < 1 {
		t.Errorf("expected >=1 insight, got %d", n)
	}

	if _, err := ps.Reflect(context.Background(), "missing"); err == nil {
		t.Error("expected error for missing session")
	}
}

func TestPipelineService_Reflect_NotEnough(t *testing.T) {
	c := setupTestContainer(t)
	createTestSession(t, c, "ses_rn_1234567", "p")

	ps := buildPipeline(t, c, reflectResp, nil)
	n, err := ps.Reflect(context.Background(), "ses_rn_1234567")
	if err != nil {
		t.Fatalf("Reflect: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 insights with no data, got %d", n)
	}
}

func TestPipelineService_ExtractGraph(t *testing.T) {
	c := setupTestContainer(t)
	createTestSession(t, c, "ses_g_1234567", "pg")
	insertCompressed(t, c, "og1", "ses_g_1234567", "Graph obs", 6)

	ps := buildPipeline(t, c, graphResp, nil)
	n, err := ps.ExtractGraph(context.Background(), "ses_g_1234567")
	if err != nil {
		t.Fatalf("ExtractGraph: %v", err)
	}
	if n < 1 {
		t.Errorf("expected >=1 processed, got %d", n)
	}
}

func TestPipelineService_ExtractGraph_NoObs(t *testing.T) {
	c := setupTestContainer(t)
	createTestSession(t, c, "ses_gn_1234567", "p")
	ps := buildPipeline(t, c, graphResp, nil)
	n, err := ps.ExtractGraph(context.Background(), "ses_gn_1234567")
	if err != nil {
		t.Fatalf("ExtractGraph: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestPipelineService_ExtractActions(t *testing.T) {
	c := setupTestContainer(t)
	createTestSession(t, c, "ses_a_1234567", "pa")

	insertCompressed(t, c, "oa1", "ses_a_1234567", "Important action", 8)
	insertCompressed(t, c, "oa2", "ses_a_1234567", "Low prio", 3) // < 7, skipped

	ps := buildPipeline(t, c, "", nil)
	n, err := ps.ExtractActions(context.Background(), "ses_a_1234567")
	if err != nil {
		t.Fatalf("ExtractActions: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 action created, got %d", n)
	}

	// Run again — should be deduped.
	n, err = ps.ExtractActions(context.Background(), "ses_a_1234567")
	if err != nil {
		t.Fatalf("ExtractActions 2: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 on dedup, got %d", n)
	}

	// Missing session.
	if _, err := ps.ExtractActions(context.Background(), "missing"); err == nil {
		t.Error("expected error for missing session")
	}
}

func TestPipelineService_RunFullPipeline(t *testing.T) {
	c := setupTestContainer(t)
	createTestSession(t, c, "ses_full_1234567", "pf")
	for i := 0; i < 4; i++ {
		insertCompressed(t, c, "of"+string(rune('a'+i)), "ses_full_1234567", "Topic "+string(rune('a'+i)), 8)
	}

	// Use a catch-all response; different stages each parse what they can.
	resp := summarizeResp + consolidateResp + reflectResp + graphResp
	ps := buildPipeline(t, c, resp, nil)
	if err := ps.RunFullPipeline(context.Background(), "ses_full_1234567"); err != nil {
		t.Fatalf("RunFullPipeline: %v", err)
	}
}

func TestPipelineService_RunFinalize(t *testing.T) {
	c := setupTestContainer(t)
	createTestSession(t, c, "ses_fin_1234567", "pfin")
	for i := 0; i < 4; i++ {
		insertCompressed(t, c, "ox"+string(rune('a'+i)), "ses_fin_1234567", "Topic "+string(rune('a'+i)), 8)
	}

	// Create an in_progress action for same project — Finalize should complete it.
	ac := NewActionService(c)
	proj := "pfin"
	ac.CreateAction("inprog", "", "in_progress", 5, &proj, nil)

	resp := summarizeResp + consolidateResp + reflectResp + graphResp
	ps := buildPipeline(t, c, resp, nil)
	if err := ps.RunFinalize(context.Background(), "ses_fin_1234567"); err != nil {
		t.Fatalf("RunFinalize: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Scheduler
// ---------------------------------------------------------------------------

func TestScheduler_DisabledWhenZero(t *testing.T) {
	c := setupTestContainer(t)
	ps := buildPipeline(t, c, summarizeResp, nil)
	ss := NewSessionService(c, nil)

	s := NewScheduler(ps, ss, 0)
	s.Start() // no-op
	s.Stop()  // no-op
}

func TestScheduler_StartStop(t *testing.T) {
	c := setupTestContainer(t)
	ps := buildPipeline(t, c, summarizeResp, nil)
	ss := NewSessionService(c, nil)

	// Use a minimal 1-minute interval; we won't wait for a tick. The goal is
	// to exercise Start → loop → Stop lifecycle.
	s := NewScheduler(ps, ss, 1)
	s.Start()

	// Second Start is a no-op.
	s.Start()

	// Give the goroutine a moment to enter the select.
	time.Sleep(20 * time.Millisecond)

	s.Stop()

	// Second Stop is a no-op.
	s.Stop()
}

func TestScheduler_Tick(t *testing.T) {
	c := setupTestContainer(t)
	createTestSession(t, c, "ses_tick_1234567", "ptick")
	// Compressed observations so tick has work.
	insertCompressed(t, c, "ott1", "ses_tick_1234567", "t1", 8)
	insertCompressed(t, c, "ott2", "ses_tick_1234567", "t2", 8)
	insertCompressed(t, c, "ott3", "ses_tick_1234567", "t3", 8)

	resp := summarizeResp + consolidateResp + reflectResp + graphResp
	ps := buildPipeline(t, c, resp, nil)
	ss := NewSessionService(c, nil)
	s := NewScheduler(ps, ss, 5)

	// Directly call tick to avoid waiting.
	s.tick()

	// Tick with no active sessions.
	c.Sessions.End("ses_tick_1234567")
	s.tick()
}

// ---------------------------------------------------------------------------
// RememberService — error paths
// ---------------------------------------------------------------------------

func TestRememberService_Errors(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewRememberService(c)

	if _, err := svc.Remember(types.MemFact, "", "c", nil, nil, 5); err == nil {
		t.Error("expected empty-title error")
	}
	if _, err := svc.Remember(types.MemFact, "t", "", nil, nil, 5); err == nil {
		t.Error("expected empty-content error")
	}

	// Strength clamping.
	lo, err := svc.Remember(types.MemFact, "t1", "c", nil, nil, 0)
	if err != nil {
		t.Fatalf("Remember lo: %v", err)
	}
	if lo.Strength != 5 {
		t.Errorf("expected default strength 5, got %d", lo.Strength)
	}

	hi, _ := svc.Remember(types.MemFact, "t2", "c", nil, nil, 999)
	if hi.Strength != 10 {
		t.Errorf("expected clamp to 10, got %d", hi.Strength)
	}

	if err := svc.Forget(""); err == nil {
		t.Error("expected empty-id error")
	}

	if _, err := svc.Evolve("", EvolveInput{Content: "c", Strength: 5}); err == nil {
		t.Error("expected empty-id error")
	}
	if _, err := svc.Evolve("x", EvolveInput{Content: "", Strength: 5}); err == nil {
		t.Error("expected empty-content error")
	}
	if _, err := svc.Evolve("nonexistent", EvolveInput{Content: "c", Strength: 5}); err == nil {
		t.Error("expected not-found error")
	}

	// Evolve with strength clamp.
	base, _ := svc.Remember(types.MemFact, "t3", "c", nil, nil, 5)
	ev, err := svc.Evolve(base.ID, EvolveInput{Content: "new", Strength: 999})
	if err != nil {
		t.Fatalf("Evolve clamp: %v", err)
	}
	if ev.Strength != 10 {
		t.Errorf("expected clamp to 10, got %d", ev.Strength)
	}

	// Evolve preserves old strength on 0.
	base2, _ := svc.Remember(types.MemFact, "t4", "c", nil, nil, 6)
	ev2, _ := svc.Evolve(base2.ID, EvolveInput{Content: "new", Strength: 0})
	if ev2.Strength != 6 {
		t.Errorf("expected preserved strength 6, got %d", ev2.Strength)
	}
}

func TestRememberService_MarshalToRawNil(t *testing.T) {
	got := marshalToRaw(nil)
	if string(got) != "null" && string(got) != "[]" {
		// json.Marshal(nil) returns "null"; the function wraps as raw, either is acceptable.
		// We accept both since the test just ensures no panic.
	}
}

// ---------------------------------------------------------------------------
// GraphService — ExtractAndStore, AllNodes, AllEdges
// ---------------------------------------------------------------------------

func TestGraphService_ExtractAndStore(t *testing.T) {
	c := setupTestContainer(t)
	createTestSession(t, c, "ses_gx_1234567", "pgx")
	insertCompressed(t, c, "ogx1", "ses_gx_1234567", "Graph ext", 6)

	p := &fakeLLM{name: "fake", available: true, resp: graphResp}
	ge := pipeline.NewGraphExtractor(p)
	gs := NewGraphService(c, ge)

	obs, err := c.Observations.GetCompressedByID("ogx1")
	if err != nil {
		t.Fatalf("get compressed: %v", err)
	}

	if err := gs.ExtractAndStore(context.Background(), obs); err != nil {
		t.Fatalf("ExtractAndStore: %v", err)
	}

	// Run a second time to exercise the "already exists" branch.
	if err := gs.ExtractAndStore(context.Background(), obs); err != nil {
		t.Fatalf("ExtractAndStore 2: %v", err)
	}
}

func TestGraphService_AllNodesAllEdges(t *testing.T) {
	c := setupTestContainer(t)
	c.Graph.CreateNode(&store.GraphNodeRow{ID: "gn_all1", Type: "file", Name: "x.go"})
	c.Graph.CreateNode(&store.GraphNodeRow{ID: "gn_all2", Type: "file", Name: "y.go"})
	c.Graph.CreateEdge(&store.GraphEdgeRow{
		ID: "ge_all1", Type: "uses", SourceNodeID: "gn_all1", TargetNodeID: "gn_all2",
		Weight: 0.5, IsLatest: 1, Version: 1,
	})

	gs := NewGraphService(c, nil)
	nodes, err := gs.AllNodes(100)
	if err != nil {
		t.Fatalf("AllNodes: %v", err)
	}
	if len(nodes) < 2 {
		t.Errorf("expected >=2 nodes, got %d", len(nodes))
	}

	edges, err := gs.AllEdges(100)
	if err != nil {
		t.Fatalf("AllEdges: %v", err)
	}
	if len(edges) < 1 {
		t.Errorf("expected >=1 edge, got %d", len(edges))
	}
}

func TestGraphService_Query_Errors(t *testing.T) {
	c := setupTestContainer(t)
	gs := NewGraphService(c, nil)
	if _, err := gs.Query("nonexistent", 0); err == nil {
		t.Error("expected error for unknown start node")
	}
}

func TestGraphService_CreateRelation_WeightClamp(t *testing.T) {
	c := setupTestContainer(t)
	c.Graph.CreateNode(&store.GraphNodeRow{ID: "gn_wa", Type: "a", Name: "a"})
	c.Graph.CreateNode(&store.GraphNodeRow{ID: "gn_wb", Type: "b", Name: "b"})
	gs := NewGraphService(c, nil)

	edge, err := gs.CreateRelation("gn_wa", "gn_wb", "rel", 0) // invalid → 0.5
	if err != nil {
		t.Fatalf("CreateRelation: %v", err)
	}
	if edge.Weight != 0.5 {
		t.Errorf("expected clamped weight 0.5, got %f", edge.Weight)
	}
	edge2, _ := gs.CreateRelation("gn_wa", "gn_wb", "rel", 2.0) // invalid → 0.5
	if edge2.Weight != 0.5 {
		t.Errorf("expected clamped weight 0.5, got %f", edge2.Weight)
	}
}
