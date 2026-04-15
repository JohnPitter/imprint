package store

import (
	"encoding/json"
	"testing"
)

// ---------------------------------------------------------------------------
// Actions
// ---------------------------------------------------------------------------

func TestActionStore_CreateAndGetByID(t *testing.T) {
	db := setupTestDB(t)
	s := NewActionStore(db)

	row := &ActionRow{
		ID:          "act-001",
		Title:       "Implement feature X",
		Description: "Full implementation of feature X",
		Status:      "pending",
		Priority:    7,
		Tags:        json.RawMessage("[]"),
	}
	if err := s.Create(row); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := s.GetByID("act-001")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Title != "Implement feature X" {
		t.Errorf("Title = %q, want %q", got.Title, "Implement feature X")
	}
	if got.Priority != 7 {
		t.Errorf("Priority = %d, want 7", got.Priority)
	}
}

func TestActionStore_List(t *testing.T) {
	db := setupTestDB(t)
	s := NewActionStore(db)

	for i, st := range []string{"pending", "pending", "done"} {
		row := &ActionRow{
			ID:     "act-l" + string(rune('1'+i)),
			Title:  "Action " + string(rune('A'+i)),
			Status: st,
			Tags:   json.RawMessage("[]"),
		}
		if err := s.Create(row); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}

	pending, err := s.List("pending", "", 10, 0)
	if err != nil {
		t.Fatalf("List pending: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("pending count = %d, want 2", len(pending))
	}

	all, err := s.List("", "", 10, 0)
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("all count = %d, want 3", len(all))
	}
}

func TestActionStore_Update(t *testing.T) {
	db := setupTestDB(t)
	s := NewActionStore(db)

	row := &ActionRow{
		ID:    "act-upd",
		Title: "Original",
		Tags:  json.RawMessage("[]"),
	}
	if err := s.Create(row); err != nil {
		t.Fatalf("Create: %v", err)
	}

	row.Title = "Updated"
	row.Status = "in_progress"
	if err := s.Update(row); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := s.GetByID("act-upd")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Title != "Updated" {
		t.Errorf("Title = %q, want %q", got.Title, "Updated")
	}
	if got.Status != "in_progress" {
		t.Errorf("Status = %q, want %q", got.Status, "in_progress")
	}
}

func TestActionStore_GetFrontier(t *testing.T) {
	db := setupTestDB(t)
	s := NewActionStore(db)

	// A is pending, no blockers.
	s.Create(&ActionRow{ID: "act-A", Title: "A", Status: "pending", Priority: 5, Tags: json.RawMessage("[]")})
	// B is pending, blocked by A.
	s.Create(&ActionRow{ID: "act-B", Title: "B", Status: "pending", Priority: 5, Tags: json.RawMessage("[]")})
	s.CreateEdge(&ActionEdgeRow{ID: "edge-1", SourceID: "act-A", TargetID: "act-B", Type: "blocks"})

	frontier, err := s.GetFrontier()
	if err != nil {
		t.Fatalf("GetFrontier: %v", err)
	}
	if len(frontier) != 1 {
		t.Fatalf("frontier len = %d, want 1", len(frontier))
	}
	if frontier[0].ID != "act-A" {
		t.Errorf("frontier[0].ID = %q, want act-A", frontier[0].ID)
	}
}

func TestActionStore_GetNext(t *testing.T) {
	db := setupTestDB(t)
	s := NewActionStore(db)

	s.Create(&ActionRow{ID: "act-lo", Title: "Low", Status: "pending", Priority: 3, Tags: json.RawMessage("[]")})
	s.Create(&ActionRow{ID: "act-hi", Title: "High", Status: "pending", Priority: 9, Tags: json.RawMessage("[]")})

	next, err := s.GetNext()
	if err != nil {
		t.Fatalf("GetNext: %v", err)
	}
	if next.ID != "act-hi" {
		t.Errorf("GetNext ID = %q, want act-hi", next.ID)
	}
}

// ---------------------------------------------------------------------------
// Leases
// ---------------------------------------------------------------------------

func TestLeaseStore_AcquireAndRelease(t *testing.T) {
	db := setupTestDB(t)
	as := NewActionStore(db)
	ls := NewLeaseStore(db)

	as.Create(&ActionRow{ID: "act-lease", Title: "Lease target", Tags: json.RawMessage("[]")})

	lease := &LeaseRow{
		ID:        "ls-001",
		ActionID:  "act-lease",
		AgentID:   "agent-1",
		ExpiresAt: "2099-12-31T23:59:59Z",
		Status:    "active",
	}
	if err := ls.Acquire(lease); err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	got, err := ls.GetByActionID("act-lease")
	if err != nil {
		t.Fatalf("GetByActionID: %v", err)
	}
	if got.Status != "active" {
		t.Errorf("Status = %q, want active", got.Status)
	}

	if err := ls.Release("ls-001", "agent-1", nil); err != nil {
		t.Fatalf("Release: %v", err)
	}

	// After release, GetByActionID for active lease should fail.
	_, err = ls.GetByActionID("act-lease")
	if err == nil {
		t.Error("expected error after release, got nil")
	}
}

// ---------------------------------------------------------------------------
// Signals
// ---------------------------------------------------------------------------

func TestSignalStore_CreateAndList(t *testing.T) {
	db := setupTestDB(t)
	s := NewSignalStore(db)

	s.Create(&SignalRow{ID: "sig-1", FromAgent: "a1", ToAgent: "a2", Content: "hello", Type: "info", ReadBy: json.RawMessage("[]")})
	s.Create(&SignalRow{ID: "sig-2", FromAgent: "a1", ToAgent: "a2", Content: "world", Type: "info", ReadBy: json.RawMessage("[]")})

	list, err := s.List("a2", 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("len = %d, want 2", len(list))
	}
}

// ---------------------------------------------------------------------------
// Checkpoints
// ---------------------------------------------------------------------------

func TestCheckpointStore_CreateAndResolve(t *testing.T) {
	db := setupTestDB(t)
	s := NewCheckpointStore(db)

	cp := &CheckpointRow{
		ID:          "cp-001",
		Name:        "Review gate",
		Description: "Needs human review",
		Config:      json.RawMessage("{}"),
	}
	if err := s.Create(cp); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := s.Resolve("cp-001", "human", "approved", "approved"); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	got, err := s.GetByID("cp-001")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != "approved" {
		t.Errorf("Status = %q, want approved", got.Status)
	}
}

// ---------------------------------------------------------------------------
// Lessons
// ---------------------------------------------------------------------------

func TestLessonStore_CreateAndSearch(t *testing.T) {
	db := setupTestDB(t)
	s := NewLessonStore(db)

	s.Create(&LessonRow{
		ID:      "les-001",
		Content: "Always validate input before processing",
		Context: "security",
		Source:  "manual",
	})

	results, err := s.Search("validate", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("search results = %d, want 1", len(results))
	}
	if results[0].ID != "les-001" {
		t.Errorf("ID = %q, want les-001", results[0].ID)
	}
}

func TestLessonStore_Strengthen(t *testing.T) {
	db := setupTestDB(t)
	s := NewLessonStore(db)

	s.Create(&LessonRow{
		ID:      "les-str",
		Content: "Use parameterized queries",
		Context: "sql",
		Source:  "manual",
	})

	if err := s.Strengthen("les-str"); err != nil {
		t.Fatalf("Strengthen: %v", err)
	}

	got, err := s.GetByID("les-str")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Reinforcements != 1 {
		t.Errorf("Reinforcements = %d, want 1", got.Reinforcements)
	}
	if got.Confidence <= 0.5 {
		t.Errorf("Confidence = %f, want > 0.5", got.Confidence)
	}
}

// ---------------------------------------------------------------------------
// Facets
// ---------------------------------------------------------------------------

func TestFacetStore_CreateAndQuery(t *testing.T) {
	db := setupTestDB(t)
	s := NewFacetStore(db)

	s.Create(&FacetRow{ID: "fct-1", TargetID: "act-1", TargetType: "action", Dimension: "priority", Value: "high"})
	s.Create(&FacetRow{ID: "fct-2", TargetID: "act-2", TargetType: "action", Dimension: "priority", Value: "high"})
	s.Create(&FacetRow{ID: "fct-3", TargetID: "act-3", TargetType: "action", Dimension: "priority", Value: "low"})

	results, err := s.QueryByDimension("priority", "high", 10)
	if err != nil {
		t.Fatalf("QueryByDimension: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("query results = %d, want 2", len(results))
	}
}

// ---------------------------------------------------------------------------
// Audit
// ---------------------------------------------------------------------------

func TestAuditStore_CreateAndList(t *testing.T) {
	db := setupTestDB(t)
	s := NewAuditStore(db)

	s.Create(&AuditRow{ID: "aud-1", Action: "create", EntityID: "e1", EntityType: "action", Meta: json.RawMessage("{}")})
	s.Create(&AuditRow{ID: "aud-2", Action: "delete", EntityID: "e2", EntityType: "action", Meta: json.RawMessage("{}")})
	s.Create(&AuditRow{ID: "aud-3", Action: "create", EntityID: "e3", EntityType: "memory", Meta: json.RawMessage("{}")})

	creates, err := s.List("create", 10, 0)
	if err != nil {
		t.Fatalf("List create: %v", err)
	}
	if len(creates) != 2 {
		t.Errorf("create count = %d, want 2", len(creates))
	}

	all, err := s.List("", 10, 0)
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("all count = %d, want 3", len(all))
	}
}
