package store

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// WAL (wal.go)
// ---------------------------------------------------------------------------

func TestWAL_NewLogClose(t *testing.T) {
	dir := t.TempDir()

	w, err := NewWAL(dir)
	if err != nil {
		t.Fatalf("NewWAL: %v", err)
	}

	w.Log(WALEntry{
		Operation:  "create",
		Entity:     "memory",
		EntityID:   "mem-1",
		ContentLen: 42,
		Meta:       map[string]any{"tag": "test"},
	})
	// Entry with pre-filled timestamp.
	w.Log(WALEntry{
		Timestamp:  "2025-01-01T00:00:00Z",
		Operation:  "update",
		Entity:     "action",
		EntityID:   "act-1",
		ContentLen: 0,
	})

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Verify file exists and contains 2 valid JSON lines.
	path := filepath.Join(dir, "wal", "write_log.jsonl")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open wal file: %v", err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 wal lines, got %d", len(lines))
	}

	var first WALEntry
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("unmarshal line 0: %v", err)
	}
	if first.Operation != "create" || first.EntityID != "mem-1" {
		t.Errorf("first entry mismatch: %+v", first)
	}
	if first.Timestamp == "" {
		t.Error("expected auto-filled timestamp on first entry")
	}

	var second WALEntry
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatalf("unmarshal line 1: %v", err)
	}
	if second.Timestamp != "2025-01-01T00:00:00Z" {
		t.Errorf("second timestamp = %q, want preserved", second.Timestamp)
	}
}

func TestWAL_CloseNil(t *testing.T) {
	w := &WAL{}
	if err := w.Close(); err != nil {
		t.Errorf("Close on empty WAL: %v", err)
	}
}

// ---------------------------------------------------------------------------
// CrystalStore (crystal.go)
// ---------------------------------------------------------------------------

func TestCrystalStore_CreateGetList(t *testing.T) {
	db := setupTestDB(t)
	s := NewCrystalStore(db)

	proj := "imprint"
	sid := "sess-c1"
	row := &CrystalRow{
		ID:              "cry-1",
		Narrative:       "Refactored store package",
		KeyOutcomes:     json.RawMessage(`["better tests"]`),
		FilesAffected:   json.RawMessage(`["store/crystal.go"]`),
		Lessons:         json.RawMessage(`["ship tests"]`),
		SourceActionIDs: json.RawMessage(`["act-x"]`),
		Project:         &proj,
		SessionID:       &sid,
	}
	if err := s.Create(row); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Second crystal, different project, filled via defaults.
	if err := s.Create(&CrystalRow{ID: "cry-2", Narrative: "Another"}); err != nil {
		t.Fatalf("Create 2: %v", err)
	}

	got, err := s.GetByID("cry-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Narrative != "Refactored store package" {
		t.Errorf("narrative = %q", got.Narrative)
	}
	if got.Project == nil || *got.Project != "imprint" {
		t.Errorf("project mismatch: %v", got.Project)
	}
	if got.SessionID == nil || *got.SessionID != "sess-c1" {
		t.Errorf("session id mismatch: %v", got.SessionID)
	}

	byProj, err := s.List("imprint", 10)
	if err != nil {
		t.Fatalf("List project: %v", err)
	}
	if len(byProj) != 1 {
		t.Errorf("project list = %d, want 1", len(byProj))
	}

	all, err := s.List("", 0) // limit=0 → default 100
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("all list = %d, want 2", len(all))
	}
}

func TestCrystalStore_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	s := NewCrystalStore(db)
	if _, err := s.GetByID("nope"); err == nil {
		t.Error("expected error for missing crystal")
	}
}

func TestCrystalStore_AutoCrystallize(t *testing.T) {
	db := setupTestDB(t)
	cs := NewCrystalStore(db)
	as := NewActionStore(db)
	ss := NewSessionStore(db)

	// Seed session + two done actions in same project.
	proj := "imprint"
	if err := ss.Create(&SessionRow{
		ID:        "sess-ac",
		Project:   proj,
		Cwd:       "/tmp",
		StartedAt: time.Now(),
		Status:    "active",
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	as.Create(&ActionRow{ID: "act-d1", Title: "A", Status: "done", Project: &proj, Tags: json.RawMessage("[]")})
	as.Create(&ActionRow{ID: "act-d2", Title: "B", Status: "done", Project: &proj, Tags: json.RawMessage("[]")})
	as.Create(&ActionRow{ID: "act-p1", Title: "P", Status: "pending", Project: &proj, Tags: json.RawMessage("[]")})

	crystal, err := cs.AutoCrystallize("sess-ac")
	if err != nil {
		t.Fatalf("AutoCrystallize: %v", err)
	}
	if crystal.ID == "" {
		t.Error("expected generated ID")
	}
	if crystal.Project == nil || *crystal.Project != proj {
		t.Errorf("project mismatch: %v", crystal.Project)
	}

	// Re-running should fail since actions are now crystallized.
	if _, err := cs.AutoCrystallize("sess-ac"); err == nil {
		t.Error("expected error when no eligible actions")
	}

	// generateID determinism — just verifies output is non-empty.
	if id := generateID(); id == "" || !strings.HasPrefix(id, "cry_") {
		t.Errorf("generateID = %q", id)
	}
}

// ---------------------------------------------------------------------------
// InsightStore (insight.go)
// ---------------------------------------------------------------------------

func TestInsightStore_Full(t *testing.T) {
	db := setupTestDB(t)
	s := NewInsightStore(db)

	proj := "imprint"
	if err := s.Create(&InsightRow{
		ID:      "ins-1",
		Title:   "Prefer small PRs",
		Content: "Smaller PRs review faster",
		Project: &proj,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := s.Create(&InsightRow{
		ID:         "ins-2",
		Title:      "Cache invalidation is hard",
		Content:    "Beware of stale data",
		Confidence: 0.9,
		Tags:       json.RawMessage(`["cache"]`),
	}); err != nil {
		t.Fatalf("Create 2: %v", err)
	}

	got, err := s.GetByID("ins-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Title != "Prefer small PRs" {
		t.Errorf("title = %q", got.Title)
	}
	if got.Confidence != 0.5 {
		t.Errorf("default confidence = %f, want 0.5", got.Confidence)
	}
	if got.DecayRate != 0.01 {
		t.Errorf("default decay = %f, want 0.01", got.DecayRate)
	}

	if _, err := s.GetByID("missing"); err == nil {
		t.Error("expected not-found error")
	}

	byProj, err := s.List("imprint", 10, 0)
	if err != nil {
		t.Fatalf("List proj: %v", err)
	}
	if len(byProj) != 1 {
		t.Errorf("proj list = %d, want 1", len(byProj))
	}

	all, err := s.List("", 0, -1) // exercise limit=0 and negative offset defaults
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("all list = %d, want 2", len(all))
	}

	res, err := s.Search("cache", 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) != 1 || res[0].ID != "ins-2" {
		t.Errorf("search results = %+v", res)
	}

	// Search on title.
	res2, err := s.Search("small", 10)
	if err != nil {
		t.Fatalf("Search title: %v", err)
	}
	if len(res2) != 1 {
		t.Errorf("title search = %d, want 1", len(res2))
	}
}

// ---------------------------------------------------------------------------
// LeaseStore (lease.go) — Renew + IsLocked
// ---------------------------------------------------------------------------

func TestLeaseStore_RenewAndIsLocked(t *testing.T) {
	db := setupTestDB(t)
	as := NewActionStore(db)
	ls := NewLeaseStore(db)

	as.Create(&ActionRow{ID: "act-lk", Title: "Lock me", Tags: json.RawMessage("[]")})

	future := TimeToString(time.Now().Add(1 * time.Hour))
	if err := ls.Acquire(&LeaseRow{
		ID: "ls-lk", ActionID: "act-lk", AgentID: "agent-A", ExpiresAt: future,
	}); err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	locked, err := ls.IsLocked("act-lk")
	if err != nil {
		t.Fatalf("IsLocked: %v", err)
	}
	if !locked {
		t.Error("expected IsLocked=true for active future lease")
	}

	// Renew to even further future.
	farther := TimeToString(time.Now().Add(2 * time.Hour))
	if err := ls.Renew("ls-lk", "agent-A", farther); err != nil {
		t.Fatalf("Renew: %v", err)
	}

	got, err := ls.GetByActionID("act-lk")
	if err != nil {
		t.Fatalf("GetByActionID: %v", err)
	}
	if got.ExpiresAt != farther {
		t.Errorf("ExpiresAt = %q, want %q", got.ExpiresAt, farther)
	}

	// Unlocked action.
	locked2, err := ls.IsLocked("unknown-act")
	if err != nil {
		t.Fatalf("IsLocked unknown: %v", err)
	}
	if locked2 {
		t.Error("expected IsLocked=false for unknown action")
	}

	// Expired lease → not locked.
	as.Create(&ActionRow{ID: "act-exp", Title: "exp", Tags: json.RawMessage("[]")})
	past := TimeToString(time.Now().Add(-1 * time.Hour))
	ls.Acquire(&LeaseRow{ID: "ls-exp", ActionID: "act-exp", AgentID: "B", ExpiresAt: past})
	lockedExp, err := ls.IsLocked("act-exp")
	if err != nil {
		t.Fatalf("IsLocked exp: %v", err)
	}
	if lockedExp {
		t.Error("expected expired lease to not be locked")
	}
}

// ---------------------------------------------------------------------------
// FacetStore — GetByTarget, Remove, Stats
// ---------------------------------------------------------------------------

func TestFacetStore_GetByTargetRemoveStats(t *testing.T) {
	db := setupTestDB(t)
	s := NewFacetStore(db)

	s.Create(&FacetRow{ID: "f1", TargetID: "e1", TargetType: "action", Dimension: "priority", Value: "high"})
	s.Create(&FacetRow{ID: "f2", TargetID: "e1", TargetType: "action", Dimension: "status", Value: "open"})
	s.Create(&FacetRow{ID: "f3", TargetID: "e2", TargetType: "memory", Dimension: "priority", Value: "low"})

	byTarget, err := s.GetByTarget("e1", "action")
	if err != nil {
		t.Fatalf("GetByTarget: %v", err)
	}
	if len(byTarget) != 2 {
		t.Errorf("byTarget = %d, want 2", len(byTarget))
	}

	stats, err := s.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats["priority"] != 2 {
		t.Errorf("stats[priority] = %d, want 2", stats["priority"])
	}
	if stats["status"] != 1 {
		t.Errorf("stats[status] = %d, want 1", stats["status"])
	}

	if err := s.Remove("f1"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	afterRemove, err := s.GetByTarget("e1", "action")
	if err != nil {
		t.Fatalf("GetByTarget after remove: %v", err)
	}
	if len(afterRemove) != 1 {
		t.Errorf("after remove = %d, want 1", len(afterRemove))
	}

	// QueryByDimension with default limit.
	qd, err := s.QueryByDimension("priority", "low", 0)
	if err != nil {
		t.Fatalf("QueryByDimension: %v", err)
	}
	if len(qd) != 1 {
		t.Errorf("qd = %d, want 1", len(qd))
	}
}

// ---------------------------------------------------------------------------
// ActionStore — Delete, ExistsByTitle, CompleteInProgress
// ---------------------------------------------------------------------------

func TestActionStore_DeleteExistsCompleteInProgress(t *testing.T) {
	db := setupTestDB(t)
	s := NewActionStore(db)

	proj := "imprint"
	s.Create(&ActionRow{ID: "ad1", Title: "Do thing", Project: &proj, Status: "in_progress", Tags: json.RawMessage("[]")})
	s.Create(&ActionRow{ID: "ad2", Title: "Another", Project: &proj, Status: "in_progress", Tags: json.RawMessage("[]")})
	s.Create(&ActionRow{ID: "ad3", Title: "Pending", Project: &proj, Status: "pending", Tags: json.RawMessage("[]")})

	exists, err := s.ExistsByTitle("Do thing")
	if err != nil {
		t.Fatalf("ExistsByTitle: %v", err)
	}
	if !exists {
		t.Error("expected exists=true")
	}

	nope, err := s.ExistsByTitle("No Such Title")
	if err != nil {
		t.Fatalf("ExistsByTitle no: %v", err)
	}
	if nope {
		t.Error("expected exists=false")
	}

	n, err := s.CompleteInProgress(proj)
	if err != nil {
		t.Fatalf("CompleteInProgress: %v", err)
	}
	if n != 2 {
		t.Errorf("affected = %d, want 2", n)
	}

	a, err := s.GetByID("ad1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if a.Status != "done" {
		t.Errorf("status = %q, want done", a.Status)
	}

	if err := s.Delete("ad1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.GetByID("ad1"); err == nil {
		t.Error("expected not-found after delete")
	}
}

// ---------------------------------------------------------------------------
// CheckpointStore — List
// ---------------------------------------------------------------------------

func TestCheckpointStore_List(t *testing.T) {
	db := setupTestDB(t)
	s := NewCheckpointStore(db)

	actID := "act-1"
	s.Create(&CheckpointRow{ID: "cp1", Name: "A", Status: "pending", ActionID: &actID})
	s.Create(&CheckpointRow{ID: "cp2", Name: "B", Status: "pending"})
	s.Create(&CheckpointRow{ID: "cp3", Name: "C", Status: "approved"})

	pending, err := s.List("pending", 10)
	if err != nil {
		t.Fatalf("List pending: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("pending count = %d, want 2", len(pending))
	}

	all, err := s.List("", 0) // limit=0 exercises default
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("all count = %d, want 3", len(all))
	}

	// Exercise scanCheckpoint for not-found path.
	if _, err := s.GetByID("nope"); err == nil {
		t.Error("expected not-found error")
	}
}

// ---------------------------------------------------------------------------
// AuditStore — Count
// ---------------------------------------------------------------------------

func TestAuditStore_Count(t *testing.T) {
	db := setupTestDB(t)
	s := NewAuditStore(db)

	initial, err := s.Count()
	if err != nil {
		t.Fatalf("Count empty: %v", err)
	}
	if initial != 0 {
		t.Errorf("initial count = %d, want 0", initial)
	}

	for i, a := range []string{"create", "update", "delete"} {
		if err := s.Create(&AuditRow{
			ID:         "a" + string(rune('1'+i)),
			Action:     a,
			EntityID:   "e1",
			EntityType: "action",
		}); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	n, err := s.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 3 {
		t.Errorf("count = %d, want 3", n)
	}
}

// ---------------------------------------------------------------------------
// GraphStore — ListEdges, InvalidateEdge
// ---------------------------------------------------------------------------

func TestGraphStore_ListEdgesAndInvalidate(t *testing.T) {
	db := setupTestDB(t)
	s := NewGraphStore(db)

	s.CreateNode(&GraphNodeRow{ID: "n1", Type: "concept", Name: "A"})
	s.CreateNode(&GraphNodeRow{ID: "n2", Type: "concept", Name: "B"})

	if err := s.CreateEdge(&GraphEdgeRow{
		ID: "e1", Type: "relates_to", SourceNodeID: "n1", TargetNodeID: "n2", IsLatest: 1,
	}); err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}
	if err := s.CreateEdge(&GraphEdgeRow{
		ID: "e2", Type: "similar", SourceNodeID: "n1", TargetNodeID: "n2", IsLatest: 1,
	}); err != nil {
		t.Fatalf("CreateEdge 2: %v", err)
	}

	edges, err := s.ListEdges(0) // default
	if err != nil {
		t.Fatalf("ListEdges: %v", err)
	}
	if len(edges) != 2 {
		t.Errorf("edges = %d, want 2", len(edges))
	}

	if err := s.InvalidateEdge("e1"); err != nil {
		t.Fatalf("InvalidateEdge: %v", err)
	}

	edgesAfter, err := s.ListEdges(100)
	if err != nil {
		t.Fatalf("ListEdges after: %v", err)
	}
	if len(edgesAfter) != 1 {
		t.Errorf("edges after invalidate = %d, want 1", len(edgesAfter))
	}

	// Invalidate unknown edge should error.
	if err := s.InvalidateEdge("missing"); err == nil {
		t.Error("expected error invalidating missing edge")
	}
}

// ---------------------------------------------------------------------------
// RoutineStore — whole store
// ---------------------------------------------------------------------------

func TestRoutineStore_Full(t *testing.T) {
	db := setupTestDB(t)
	s := NewRoutineStore(db)

	r := &RoutineRow{
		ID:    "r1",
		Name:  "Daily standup",
		Steps: json.RawMessage(`["yesterday","today","blockers"]`),
		Tags:  json.RawMessage(`["meeting"]`),
	}
	if err := s.Create(r); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := s.Create(&RoutineRow{ID: "r2", Name: "Deploy"}); err != nil {
		t.Fatalf("Create 2: %v", err)
	}

	got, err := s.GetByID("r1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Daily standup" {
		t.Errorf("name = %q", got.Name)
	}

	if _, err := s.GetByID("missing"); err == nil {
		t.Error("expected not-found")
	}

	list, err := s.List(0, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("list = %d, want 2", len(list))
	}

	r.Name = "Weekly standup"
	r.Frozen = 1
	if err := s.Update(r); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got2, _ := s.GetByID("r1")
	if got2.Name != "Weekly standup" || got2.Frozen != 1 {
		t.Errorf("update mismatch: %+v", got2)
	}

	if err := s.Delete("r1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.GetByID("r1"); err == nil {
		t.Error("expected not-found after delete")
	}
}

// ---------------------------------------------------------------------------
// SentinelStore — whole store
// ---------------------------------------------------------------------------

func TestSentinelStore_Full(t *testing.T) {
	db := setupTestDB(t)
	s := NewSentinelStore(db)

	expires := TimeToString(time.Now().Add(1 * time.Hour))
	sen := &SentinelRow{
		ID:        "sn1",
		Name:      "CPU watcher",
		Type:      "metric",
		Config:    json.RawMessage(`{"threshold":0.9}`),
		ExpiresAt: &expires,
	}
	if err := s.Create(sen); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Create(&SentinelRow{ID: "sn2", Name: "Another", Type: "metric"}); err != nil {
		t.Fatalf("Create 2: %v", err)
	}

	got, err := s.GetByID("sn1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != "watching" {
		t.Errorf("default status = %q", got.Status)
	}

	if _, err := s.GetByID("missing"); err == nil {
		t.Error("expected not-found")
	}

	// Check is an alias for GetByID.
	c, err := s.Check("sn1")
	if err != nil || c.ID != "sn1" {
		t.Errorf("Check: %v / %+v", err, c)
	}

	watching, err := s.List("watching", 10)
	if err != nil {
		t.Fatalf("List watching: %v", err)
	}
	if len(watching) != 2 {
		t.Errorf("watching = %d, want 2", len(watching))
	}

	all, err := s.List("", 0)
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("all = %d, want 2", len(all))
	}

	if err := s.Trigger("sn1", "over threshold"); err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	got2, _ := s.GetByID("sn1")
	if got2.Status != "triggered" {
		t.Errorf("after trigger status = %q", got2.Status)
	}
	if got2.Result == nil || *got2.Result != "over threshold" {
		t.Errorf("result mismatch: %v", got2.Result)
	}
	if got2.TriggeredAt == nil {
		t.Error("expected TriggeredAt set")
	}

	if err := s.Cancel("sn2"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	got3, _ := s.GetByID("sn2")
	if got3.Status != "cancelled" {
		t.Errorf("after cancel status = %q", got3.Status)
	}
}

// ---------------------------------------------------------------------------
// SketchStore — whole store
// ---------------------------------------------------------------------------

func TestSketchStore_Full(t *testing.T) {
	db := setupTestDB(t)
	s := NewSketchStore(db)

	proj := "imprint"
	future := TimeToString(time.Now().Add(1 * time.Hour))
	sk := &SketchRow{
		ID:        "sk1",
		Title:     "Draft plan",
		Project:   &proj,
		ExpiresAt: future,
	}
	if err := s.Create(sk); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Create(&SketchRow{ID: "sk2", Title: "Other", ExpiresAt: future}); err != nil {
		t.Fatalf("Create 2: %v", err)
	}

	got, err := s.GetByID("sk1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != "active" {
		t.Errorf("status = %q, want active", got.Status)
	}

	if _, err := s.GetByID("missing"); err == nil {
		t.Error("expected not-found")
	}

	active, err := s.List("active", 10)
	if err != nil {
		t.Fatalf("List active: %v", err)
	}
	if len(active) != 2 {
		t.Errorf("active = %d, want 2", len(active))
	}

	all, err := s.List("", 0)
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("all = %d, want 2", len(all))
	}

	if err := s.AddAction("sk1", "act-x"); err != nil {
		t.Fatalf("AddAction: %v", err)
	}
	// Adding same action again should be no-op.
	if err := s.AddAction("sk1", "act-x"); err != nil {
		t.Fatalf("AddAction dup: %v", err)
	}
	if err := s.AddAction("sk1", "act-y"); err != nil {
		t.Fatalf("AddAction y: %v", err)
	}
	// AddAction to missing sketch.
	if err := s.AddAction("missing", "act-z"); err == nil {
		t.Error("expected error on missing sketch")
	}

	got2, _ := s.GetByID("sk1")
	var ids []string
	if err := json.Unmarshal(got2.ActionIDs, &ids); err != nil {
		t.Fatalf("unmarshal action_ids: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("actionIDs = %v, want 2 entries", ids)
	}

	if err := s.Promote("sk1"); err != nil {
		t.Fatalf("Promote: %v", err)
	}
	got3, _ := s.GetByID("sk1")
	if got3.Status != "promoted" || got3.PromotedAt == nil {
		t.Errorf("after promote: %+v", got3)
	}

	if err := s.Discard("sk2"); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	got4, _ := s.GetByID("sk2")
	if got4.Status != "discarded" || got4.DiscardedAt == nil {
		t.Errorf("after discard: %+v", got4)
	}

	// GarbageCollect: create expired active sketch, expect removal.
	past := TimeToString(time.Now().Add(-1 * time.Hour))
	s.Create(&SketchRow{ID: "sk-old", Title: "Old", ExpiresAt: past})
	n, err := s.GarbageCollect()
	if err != nil {
		t.Fatalf("GarbageCollect: %v", err)
	}
	if n != 1 {
		t.Errorf("GarbageCollect = %d, want 1", n)
	}
	if _, err := s.GetByID("sk-old"); err == nil {
		t.Error("expected sk-old deleted")
	}
}

// ---------------------------------------------------------------------------
// SignalStore — Send, MarkRead
// ---------------------------------------------------------------------------

func TestSignalStore_SendAndMarkRead(t *testing.T) {
	db := setupTestDB(t)
	s := NewSignalStore(db)

	// Send is alias for Create.
	if err := s.Send(&SignalRow{ID: "sig-A", FromAgent: "u1", ToAgent: "u2", Content: "hi"}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if err := s.MarkRead("sig-A", "u2"); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	// Second MarkRead with same agent should be no-op.
	if err := s.MarkRead("sig-A", "u2"); err != nil {
		t.Fatalf("MarkRead dup: %v", err)
	}
	if err := s.MarkRead("sig-A", "u3"); err != nil {
		t.Fatalf("MarkRead u3: %v", err)
	}
	if err := s.MarkRead("missing", "u2"); err == nil {
		t.Error("expected not-found for missing signal")
	}

	list, err := s.List("u2", 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list = %d, want 1", len(list))
	}
	var readers []string
	if err := json.Unmarshal(list[0].ReadBy, &readers); err != nil {
		t.Fatalf("unmarshal readBy: %v", err)
	}
	if len(readers) != 2 {
		t.Errorf("readers = %v, want 2", readers)
	}
}

// ---------------------------------------------------------------------------
// LessonStore — List
// ---------------------------------------------------------------------------

func TestLessonStore_List(t *testing.T) {
	db := setupTestDB(t)
	s := NewLessonStore(db)

	proj := "imprint"
	other := "other"
	s.Create(&LessonRow{ID: "l1", Content: "Write tests", Project: &proj, Confidence: 0.9})
	s.Create(&LessonRow{ID: "l2", Content: "Review code", Project: &proj, Confidence: 0.6})
	s.Create(&LessonRow{ID: "l3", Content: "Ship it", Project: &other, Confidence: 0.7})
	s.Create(&LessonRow{ID: "l4", Content: "Deleted", Deleted: 1})

	byProj, err := s.List("imprint", 10, 0)
	if err != nil {
		t.Fatalf("List proj: %v", err)
	}
	if len(byProj) != 2 {
		t.Errorf("proj list = %d, want 2", len(byProj))
	}
	// Order by confidence DESC.
	if byProj[0].ID != "l1" {
		t.Errorf("first ID = %q, want l1", byProj[0].ID)
	}

	all, err := s.List("", 0, -1)
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("all (non-deleted) = %d, want 3", len(all))
	}
}

// ---------------------------------------------------------------------------
// DB — DataDir
// ---------------------------------------------------------------------------

func TestDB_DataDir(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()
	if got := db.DataDir(); got != dir {
		t.Errorf("DataDir = %q, want %q", got, dir)
	}
}
