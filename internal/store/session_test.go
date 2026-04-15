package store

import (
	"fmt"
	"testing"
	"time"

	"imprint/internal/types"
)

func setupTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSessionStore_CreateAndGetByID(t *testing.T) {
	db := setupTestDB(t)
	store := NewSessionStore(db)

	model := "claude-opus-4-20250514"
	sess := &SessionRow{
		ID:               "sess-001",
		Project:          "imprint",
		Cwd:              "/home/user/projects/imprint",
		StartedAt:        time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC),
		Status:           types.SessionActive,
		ObservationCount: 0,
		Model:            &model,
		Tags:             []string{"dev", "testing"},
	}

	if err := store.Create(sess); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.GetByID("sess-001")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if got.ID != sess.ID {
		t.Errorf("ID = %q, want %q", got.ID, sess.ID)
	}
	if got.Project != sess.Project {
		t.Errorf("Project = %q, want %q", got.Project, sess.Project)
	}
	if got.Cwd != sess.Cwd {
		t.Errorf("Cwd = %q, want %q", got.Cwd, sess.Cwd)
	}
	if !got.StartedAt.Equal(sess.StartedAt) {
		t.Errorf("StartedAt = %v, want %v", got.StartedAt, sess.StartedAt)
	}
	if got.EndedAt != nil {
		t.Errorf("EndedAt = %v, want nil", got.EndedAt)
	}
	if got.Status != types.SessionActive {
		t.Errorf("Status = %q, want %q", got.Status, types.SessionActive)
	}
	if got.ObservationCount != 0 {
		t.Errorf("ObservationCount = %d, want 0", got.ObservationCount)
	}
	if got.Model == nil || *got.Model != model {
		t.Errorf("Model = %v, want %q", got.Model, model)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "dev" || got.Tags[1] != "testing" {
		t.Errorf("Tags = %v, want [dev testing]", got.Tags)
	}
}

func TestSessionStore_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	store := NewSessionStore(db)

	_, err := store.GetByID("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session, got nil")
	}
}

func TestSessionStore_List(t *testing.T) {
	db := setupTestDB(t)
	store := NewSessionStore(db)

	// Create 3 sessions with different timestamps.
	for i, ts := range []time.Time{
		time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 11, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC),
	} {
		sess := &SessionRow{
			ID:        fmt.Sprintf("sess-%03d", i+1),
			Project:   "proj-a",
			Cwd:       "/tmp",
			StartedAt: ts,
			Status:    types.SessionActive,
			Tags:      []string{},
		}
		if err := store.Create(sess); err != nil {
			t.Fatalf("Create sess-%03d: %v", i+1, err)
		}
	}

	results, err := store.List("", 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("List len = %d, want 3", len(results))
	}

	// Verify DESC order (newest first).
	if results[0].ID != "sess-003" {
		t.Errorf("first session = %q, want sess-003", results[0].ID)
	}
	if results[2].ID != "sess-001" {
		t.Errorf("last session = %q, want sess-001", results[2].ID)
	}

	// Test pagination.
	page, err := store.List("", 2, 1)
	if err != nil {
		t.Fatalf("List paginated: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("paginated len = %d, want 2", len(page))
	}
	if page[0].ID != "sess-002" {
		t.Errorf("paginated first = %q, want sess-002", page[0].ID)
	}
}

func TestSessionStore_ListByProject(t *testing.T) {
	db := setupTestDB(t)
	store := NewSessionStore(db)

	projects := []struct {
		id      string
		project string
	}{
		{"sess-a1", "project-alpha"},
		{"sess-a2", "project-alpha"},
		{"sess-b1", "project-beta"},
	}

	for _, p := range projects {
		sess := &SessionRow{
			ID:        p.id,
			Project:   p.project,
			Cwd:       "/workspace",
			StartedAt: time.Now().UTC(),
			Status:    types.SessionActive,
			Tags:      []string{},
		}
		if err := store.Create(sess); err != nil {
			t.Fatalf("Create %s: %v", p.id, err)
		}
	}

	alphaResults, err := store.List("project-alpha", 10, 0)
	if err != nil {
		t.Fatalf("List project-alpha: %v", err)
	}
	if len(alphaResults) != 2 {
		t.Fatalf("project-alpha count = %d, want 2", len(alphaResults))
	}
	for _, r := range alphaResults {
		if r.Project != "project-alpha" {
			t.Errorf("unexpected project %q in alpha results", r.Project)
		}
	}

	betaResults, err := store.List("project-beta", 10, 0)
	if err != nil {
		t.Fatalf("List project-beta: %v", err)
	}
	if len(betaResults) != 1 {
		t.Fatalf("project-beta count = %d, want 1", len(betaResults))
	}
	if betaResults[0].ID != "sess-b1" {
		t.Errorf("beta session = %q, want sess-b1", betaResults[0].ID)
	}
}

func TestSessionStore_End(t *testing.T) {
	db := setupTestDB(t)
	store := NewSessionStore(db)

	sess := &SessionRow{
		ID:        "sess-end",
		Project:   "proj",
		Cwd:       "/tmp",
		StartedAt: time.Now().UTC(),
		Status:    types.SessionActive,
		Tags:      []string{},
	}
	if err := store.Create(sess); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.End("sess-end"); err != nil {
		t.Fatalf("End: %v", err)
	}

	got, err := store.GetByID("sess-end")
	if err != nil {
		t.Fatalf("GetByID after End: %v", err)
	}
	if got.Status != types.SessionCompleted {
		t.Errorf("Status = %q, want %q", got.Status, types.SessionCompleted)
	}
	if got.EndedAt == nil {
		t.Fatal("EndedAt should be set after End()")
	}
}

func TestSessionStore_End_NotFound(t *testing.T) {
	db := setupTestDB(t)
	store := NewSessionStore(db)

	err := store.End("nonexistent")
	if err == nil {
		t.Fatal("expected error when ending nonexistent session")
	}
}

func TestSessionStore_IncrementObservationCount(t *testing.T) {
	db := setupTestDB(t)
	store := NewSessionStore(db)

	sess := &SessionRow{
		ID:        "sess-inc",
		Project:   "proj",
		Cwd:       "/tmp",
		StartedAt: time.Now().UTC(),
		Status:    types.SessionActive,
		Tags:      []string{},
	}
	if err := store.Create(sess); err != nil {
		t.Fatalf("Create: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := store.IncrementObservationCount("sess-inc"); err != nil {
			t.Fatalf("IncrementObservationCount #%d: %v", i+1, err)
		}
	}

	got, err := store.GetByID("sess-inc")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ObservationCount != 3 {
		t.Errorf("ObservationCount = %d, want 3", got.ObservationCount)
	}
}

func TestSessionStore_Count(t *testing.T) {
	db := setupTestDB(t)
	store := NewSessionStore(db)

	// Empty database.
	count, err := store.Count("")
	if err != nil {
		t.Fatalf("Count empty: %v", err)
	}
	if count != 0 {
		t.Errorf("Count empty = %d, want 0", count)
	}

	// Add sessions.
	for _, s := range []struct {
		id      string
		project string
	}{
		{"s1", "proj-a"},
		{"s2", "proj-a"},
		{"s3", "proj-b"},
	} {
		sess := &SessionRow{
			ID:        s.id,
			Project:   s.project,
			Cwd:       "/tmp",
			StartedAt: time.Now().UTC(),
			Status:    types.SessionActive,
			Tags:      []string{},
		}
		if err := store.Create(sess); err != nil {
			t.Fatalf("Create %s: %v", s.id, err)
		}
	}

	total, err := store.Count("")
	if err != nil {
		t.Fatalf("Count all: %v", err)
	}
	if total != 3 {
		t.Errorf("Count all = %d, want 3", total)
	}

	projA, err := store.Count("proj-a")
	if err != nil {
		t.Fatalf("Count proj-a: %v", err)
	}
	if projA != 2 {
		t.Errorf("Count proj-a = %d, want 2", projA)
	}

	projB, err := store.Count("proj-b")
	if err != nil {
		t.Fatalf("Count proj-b: %v", err)
	}
	if projB != 1 {
		t.Errorf("Count proj-b = %d, want 1", projB)
	}
}

func TestSessionStore_GetActive(t *testing.T) {
	db := setupTestDB(t)
	store := NewSessionStore(db)

	// Create an active session.
	active := &SessionRow{
		ID:        "sess-active",
		Project:   "proj",
		Cwd:       "/tmp",
		StartedAt: time.Now().UTC(),
		Status:    types.SessionActive,
		Tags:      []string{},
	}
	if err := store.Create(active); err != nil {
		t.Fatalf("Create active: %v", err)
	}

	// Create a completed session.
	completed := &SessionRow{
		ID:        "sess-completed",
		Project:   "proj",
		Cwd:       "/tmp",
		StartedAt: time.Now().UTC(),
		Status:    types.SessionCompleted,
		Tags:      []string{},
	}
	if err := store.Create(completed); err != nil {
		t.Fatalf("Create completed: %v", err)
	}

	// Create an abandoned session.
	abandoned := &SessionRow{
		ID:        "sess-abandoned",
		Project:   "proj",
		Cwd:       "/tmp",
		StartedAt: time.Now().UTC(),
		Status:    types.SessionAbandoned,
		Tags:      []string{},
	}
	if err := store.Create(abandoned); err != nil {
		t.Fatalf("Create abandoned: %v", err)
	}

	results, err := store.GetActive()
	if err != nil {
		t.Fatalf("GetActive: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("GetActive count = %d, want 1", len(results))
	}
	if results[0].ID != "sess-active" {
		t.Errorf("active session ID = %q, want sess-active", results[0].ID)
	}
	if results[0].Status != types.SessionActive {
		t.Errorf("active session status = %q, want %q", results[0].Status, types.SessionActive)
	}
}
