package store

import (
	"fmt"
	"testing"
	"time"

	"imprint/internal/types"
)

func TestSummaryStore_CreateAndGet(t *testing.T) {
	db := setupTestDB(t)
	createTestSession(t, db, "sess-sum-1", "proj-summary")
	store := NewSummaryStore(db)

	summary := &SummaryRow{
		SessionID:        "sess-sum-1",
		Project:          "proj-summary",
		Title:            "Implemented session CRUD",
		Narrative:        "Built the full session store with create, read, list, and end operations.",
		KeyDecisions:     `["Used SQLite WAL mode","Chose RFC3339 for timestamps"]`,
		FilesModified:    `["internal/store/session.go","internal/store/db.go"]`,
		Concepts:         `["sqlite","crud","sessions"]`,
		ObservationCount: 15,
	}

	if err := store.Create(summary); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.GetBySessionID("sess-sum-1")
	if err != nil {
		t.Fatalf("GetBySessionID: %v", err)
	}

	if got.SessionID != "sess-sum-1" {
		t.Errorf("SessionID = %q, want sess-sum-1", got.SessionID)
	}
	if got.Project != "proj-summary" {
		t.Errorf("Project = %q, want proj-summary", got.Project)
	}
	if got.Title != "Implemented session CRUD" {
		t.Errorf("Title = %q, want 'Implemented session CRUD'", got.Title)
	}
	if got.Narrative != "Built the full session store with create, read, list, and end operations." {
		t.Errorf("Narrative mismatch")
	}
	if got.KeyDecisions != `["Used SQLite WAL mode","Chose RFC3339 for timestamps"]` {
		t.Errorf("KeyDecisions = %q", got.KeyDecisions)
	}
	if got.FilesModified != `["internal/store/session.go","internal/store/db.go"]` {
		t.Errorf("FilesModified = %q", got.FilesModified)
	}
	if got.Concepts != `["sqlite","crud","sessions"]` {
		t.Errorf("Concepts = %q", got.Concepts)
	}
	if got.ObservationCount != 15 {
		t.Errorf("ObservationCount = %d, want 15", got.ObservationCount)
	}
	if got.CreatedAt == "" {
		t.Error("CreatedAt should be auto-set")
	}
}

func TestSummaryStore_GetBySessionID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	store := NewSummaryStore(db)

	_, err := store.GetBySessionID("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent summary")
	}
}

func TestSummaryStore_CreateReplace(t *testing.T) {
	db := setupTestDB(t)
	createTestSession(t, db, "sess-replace", "proj-replace")
	store := NewSummaryStore(db)

	original := &SummaryRow{
		SessionID:        "sess-replace",
		Project:          "proj-replace",
		Title:            "Original summary",
		Narrative:        "First version of the summary.",
		ObservationCount: 5,
	}
	if err := store.Create(original); err != nil {
		t.Fatalf("Create original: %v", err)
	}

	// Replace with updated data.
	replacement := &SummaryRow{
		SessionID:        "sess-replace",
		Project:          "proj-replace",
		Title:            "Updated summary",
		Narrative:        "Second version with more observations processed.",
		KeyDecisions:     `["Added error handling"]`,
		FilesModified:    `["main.go"]`,
		Concepts:         `["refactoring"]`,
		ObservationCount: 12,
	}
	if err := store.Create(replacement); err != nil {
		t.Fatalf("Create replacement: %v", err)
	}

	got, err := store.GetBySessionID("sess-replace")
	if err != nil {
		t.Fatalf("GetBySessionID after replace: %v", err)
	}
	if got.Title != "Updated summary" {
		t.Errorf("Title = %q, want 'Updated summary'", got.Title)
	}
	if got.Narrative != "Second version with more observations processed." {
		t.Errorf("Narrative not updated")
	}
	if got.ObservationCount != 12 {
		t.Errorf("ObservationCount = %d, want 12", got.ObservationCount)
	}
}

func TestSummaryStore_ListByProject(t *testing.T) {
	db := setupTestDB(t)
	store := NewSummaryStore(db)

	// Create sessions and summaries for two projects.
	sessStore := NewSessionStore(db)
	projects := []struct {
		sessID  string
		project string
	}{
		{"sess-lp-1", "project-x"},
		{"sess-lp-2", "project-x"},
		{"sess-lp-3", "project-y"},
	}

	for _, p := range projects {
		sess := &SessionRow{
			ID:        p.sessID,
			Project:   p.project,
			Cwd:       "/workspace",
			StartedAt: time.Now().UTC(),
			Status:    types.SessionActive,
			Tags:      []string{},
		}
		if err := sessStore.Create(sess); err != nil {
			t.Fatalf("Create session %s: %v", p.sessID, err)
		}

		summary := &SummaryRow{
			SessionID:        p.sessID,
			Project:          p.project,
			Title:            fmt.Sprintf("Summary for %s", p.sessID),
			Narrative:        "Description",
			ObservationCount: 5,
		}
		if err := store.Create(summary); err != nil {
			t.Fatalf("Create summary %s: %v", p.sessID, err)
		}
	}

	xResults, err := store.ListByProject("project-x", 10)
	if err != nil {
		t.Fatalf("ListByProject project-x: %v", err)
	}
	if len(xResults) != 2 {
		t.Fatalf("project-x count = %d, want 2", len(xResults))
	}
	for _, r := range xResults {
		if r.Project != "project-x" {
			t.Errorf("unexpected project %q in project-x results", r.Project)
		}
	}

	yResults, err := store.ListByProject("project-y", 10)
	if err != nil {
		t.Fatalf("ListByProject project-y: %v", err)
	}
	if len(yResults) != 1 {
		t.Fatalf("project-y count = %d, want 1", len(yResults))
	}

	// Empty project.
	empty, err := store.ListByProject("nonexistent", 10)
	if err != nil {
		t.Fatalf("ListByProject nonexistent: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("nonexistent project count = %d, want 0", len(empty))
	}
}

func TestSummaryStore_ListRecent(t *testing.T) {
	db := setupTestDB(t)
	store := NewSummaryStore(db)

	baseTime := time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC)

	for i := 0; i < 3; i++ {
		sessID := fmt.Sprintf("sess-recent-%d", i)
		createTestSession(t, db, sessID, "proj-recent")

		summary := &SummaryRow{
			SessionID:        sessID,
			Project:          "proj-recent",
			CreatedAt:        TimeToString(baseTime.Add(time.Duration(i) * time.Hour)),
			Title:            fmt.Sprintf("Summary %d", i),
			Narrative:        "Description",
			ObservationCount: i + 1,
		}
		if err := store.Create(summary); err != nil {
			t.Fatalf("Create summary %d: %v", i, err)
		}
	}

	results, err := store.ListRecent(10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("ListRecent len = %d, want 3", len(results))
	}

	// Verify DESC order (newest first).
	if results[0].SessionID != "sess-recent-2" {
		t.Errorf("first = %q, want sess-recent-2", results[0].SessionID)
	}
	if results[2].SessionID != "sess-recent-0" {
		t.Errorf("last = %q, want sess-recent-0", results[2].SessionID)
	}

	// Test limit.
	limited, err := store.ListRecent(2)
	if err != nil {
		t.Fatalf("ListRecent limited: %v", err)
	}
	if len(limited) != 2 {
		t.Fatalf("limited len = %d, want 2", len(limited))
	}
}

func TestSummaryStore_Delete(t *testing.T) {
	db := setupTestDB(t)
	createTestSession(t, db, "sess-del-sum", "proj-del")
	store := NewSummaryStore(db)

	summary := &SummaryRow{
		SessionID:        "sess-del-sum",
		Project:          "proj-del",
		Title:            "To be deleted",
		Narrative:        "This summary will be removed.",
		ObservationCount: 3,
	}
	if err := store.Create(summary); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Verify it exists.
	_, err := store.GetBySessionID("sess-del-sum")
	if err != nil {
		t.Fatalf("GetBySessionID before delete: %v", err)
	}

	if err := store.Delete("sess-del-sum"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify it's gone.
	_, err = store.GetBySessionID("sess-del-sum")
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}
