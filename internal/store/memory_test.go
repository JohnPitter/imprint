package store

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestMemoryStore_CreateAndGetByID(t *testing.T) {
	db := setupTestDB(t)
	store := NewMemoryStore(db)

	ttl := 30
	mem := &MemoryRow{
		ID:                   "mem-001",
		Type:                 "pattern",
		Title:                "Error handling pattern",
		Content:              "Always wrap errors with fmt.Errorf and context",
		Concepts:             json.RawMessage(`["error-handling","go-patterns"]`),
		Files:                json.RawMessage(`["internal/store/session.go"]`),
		SessionIDs:           json.RawMessage(`["sess-001"]`),
		Strength:             7,
		Version:              1,
		SourceObservationIDs: json.RawMessage(`["comp-001"]`),
		IsLatest:             1,
		TTLDays:              &ttl,
	}

	if err := store.Create(mem); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.GetByID("mem-001")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if got.ID != "mem-001" {
		t.Errorf("ID = %q, want mem-001", got.ID)
	}
	if got.Type != "pattern" {
		t.Errorf("Type = %q, want pattern", got.Type)
	}
	if got.Title != "Error handling pattern" {
		t.Errorf("Title = %q, want 'Error handling pattern'", got.Title)
	}
	if got.Content != "Always wrap errors with fmt.Errorf and context" {
		t.Errorf("Content mismatch")
	}
	if got.Strength != 7 {
		t.Errorf("Strength = %d, want 7", got.Strength)
	}
	if got.Version != 1 {
		t.Errorf("Version = %d, want 1", got.Version)
	}
	if got.IsLatest != 1 {
		t.Errorf("IsLatest = %d, want 1", got.IsLatest)
	}
	if got.TTLDays == nil || *got.TTLDays != 30 {
		t.Errorf("TTLDays = %v, want 30", got.TTLDays)
	}
	if got.ParentID != nil {
		t.Errorf("ParentID = %v, want nil", got.ParentID)
	}
	if got.CreatedAt == "" {
		t.Error("CreatedAt should be auto-set")
	}
	if got.UpdatedAt == "" {
		t.Error("UpdatedAt should be auto-set")
	}
}

func TestMemoryStore_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	store := NewMemoryStore(db)

	_, err := store.GetByID("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent memory")
	}
}

func TestMemoryStore_List(t *testing.T) {
	db := setupTestDB(t)
	store := NewMemoryStore(db)

	for i := 0; i < 3; i++ {
		mem := &MemoryRow{
			ID:       fmt.Sprintf("mem-list-%d", i),
			Type:     "fact",
			Title:    fmt.Sprintf("Fact #%d", i),
			Content:  fmt.Sprintf("Content for fact %d", i),
			Strength: 5,
			IsLatest: 1,
		}
		if err := store.Create(mem); err != nil {
			t.Fatalf("Create #%d: %v", i, err)
		}
	}

	// List all.
	results, err := store.List("", 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("List len = %d, want 3", len(results))
	}

	// Test pagination: limit 2, offset 0.
	page1, err := store.List("", 2, 0)
	if err != nil {
		t.Fatalf("List page1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page1 len = %d, want 2", len(page1))
	}

	// Page 2: limit 2, offset 2.
	page2, err := store.List("", 2, 2)
	if err != nil {
		t.Fatalf("List page2: %v", err)
	}
	if len(page2) != 1 {
		t.Fatalf("page2 len = %d, want 1", len(page2))
	}
}

func TestMemoryStore_ListByType(t *testing.T) {
	db := setupTestDB(t)
	store := NewMemoryStore(db)

	types := []string{"pattern", "pattern", "bug", "preference"}
	for i, typ := range types {
		mem := &MemoryRow{
			ID:       fmt.Sprintf("mem-type-%d", i),
			Type:     typ,
			Title:    fmt.Sprintf("Memory %d", i),
			Content:  "Content",
			Strength: 5,
			IsLatest: 1,
		}
		if err := store.Create(mem); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	patterns, err := store.List("pattern", 10, 0)
	if err != nil {
		t.Fatalf("List pattern: %v", err)
	}
	if len(patterns) != 2 {
		t.Fatalf("pattern count = %d, want 2", len(patterns))
	}
	for _, p := range patterns {
		if p.Type != "pattern" {
			t.Errorf("unexpected type %q in pattern results", p.Type)
		}
	}

	bugs, err := store.List("bug", 10, 0)
	if err != nil {
		t.Fatalf("List bug: %v", err)
	}
	if len(bugs) != 1 {
		t.Fatalf("bug count = %d, want 1", len(bugs))
	}
}

func TestMemoryStore_Update(t *testing.T) {
	db := setupTestDB(t)
	store := NewMemoryStore(db)

	mem := &MemoryRow{
		ID:       "mem-update",
		Type:     "fact",
		Title:    "Original title",
		Content:  "Original content",
		Strength: 3,
		IsLatest: 1,
	}
	if err := store.Create(mem); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Update content and strength.
	mem.Content = "Updated content with more detail"
	mem.Strength = 8
	mem.Title = "Updated title"
	if err := store.Update(mem); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := store.GetByID("mem-update")
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if got.Content != "Updated content with more detail" {
		t.Errorf("Content = %q, want updated", got.Content)
	}
	if got.Strength != 8 {
		t.Errorf("Strength = %d, want 8", got.Strength)
	}
	if got.Title != "Updated title" {
		t.Errorf("Title = %q, want 'Updated title'", got.Title)
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	db := setupTestDB(t)
	store := NewMemoryStore(db)

	mem := &MemoryRow{
		ID:       "mem-soft-del",
		Type:     "workflow",
		Title:    "Deploy workflow",
		Content:  "Steps to deploy",
		Strength: 6,
		IsLatest: 1,
	}
	if err := store.Create(mem); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.Delete("mem-soft-del"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := store.GetByID("mem-soft-del")
	if err != nil {
		t.Fatalf("GetByID after soft delete: %v", err)
	}
	if got.IsLatest != 0 {
		t.Errorf("IsLatest = %d, want 0 after soft delete", got.IsLatest)
	}

	// Soft-deleted memories should not appear in List (which filters is_latest=1).
	results, err := store.List("", 10, 0)
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	for _, r := range results {
		if r.ID == "mem-soft-del" {
			t.Error("soft-deleted memory should not appear in List")
		}
	}
}

func TestMemoryStore_HardDelete(t *testing.T) {
	db := setupTestDB(t)
	store := NewMemoryStore(db)

	mem := &MemoryRow{
		ID:       "mem-hard-del",
		Type:     "fact",
		Title:    "Temporary fact",
		Content:  "This will be permanently deleted",
		Strength: 2,
		IsLatest: 1,
	}
	if err := store.Create(mem); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.HardDelete("mem-hard-del"); err != nil {
		t.Fatalf("HardDelete: %v", err)
	}

	_, err := store.GetByID("mem-hard-del")
	if err == nil {
		t.Fatal("expected error after hard delete, got nil")
	}
}

func TestMemoryStore_Supersede(t *testing.T) {
	db := setupTestDB(t)
	store := NewMemoryStore(db)

	old := &MemoryRow{
		ID:       "mem-old",
		Type:     "architecture",
		Title:    "Database schema v1",
		Content:  "Initial schema with 5 tables",
		Strength: 6,
		IsLatest: 1,
		Version:  1,
	}
	if err := store.Create(old); err != nil {
		t.Fatalf("Create old: %v", err)
	}

	newMem := &MemoryRow{
		ID:       "mem-new",
		Type:     "architecture",
		Title:    "Database schema v2",
		Content:  "Updated schema with 10 tables and indexes",
		Strength: 8,
		Version:  2,
	}
	if err := store.Supersede("mem-old", newMem); err != nil {
		t.Fatalf("Supersede: %v", err)
	}

	// Old memory should no longer be latest.
	oldGot, err := store.GetByID("mem-old")
	if err != nil {
		t.Fatalf("GetByID old: %v", err)
	}
	if oldGot.IsLatest != 0 {
		t.Errorf("old IsLatest = %d, want 0", oldGot.IsLatest)
	}

	// New memory should be latest with parent_id set.
	newGot, err := store.GetByID("mem-new")
	if err != nil {
		t.Fatalf("GetByID new: %v", err)
	}
	if newGot.IsLatest != 1 {
		t.Errorf("new IsLatest = %d, want 1", newGot.IsLatest)
	}
	if newGot.ParentID == nil || *newGot.ParentID != "mem-old" {
		t.Errorf("new ParentID = %v, want mem-old", newGot.ParentID)
	}
	if string(newGot.Supersedes) != `["mem-old"]` {
		t.Errorf("new Supersedes = %q, want [\"mem-old\"]", string(newGot.Supersedes))
	}

	// List should only return the new memory.
	results, err := store.List("", 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("List len = %d, want 1", len(results))
	}
	if results[0].ID != "mem-new" {
		t.Errorf("List result = %q, want mem-new", results[0].ID)
	}
}

func TestMemoryStore_ListByStrength(t *testing.T) {
	db := setupTestDB(t)
	store := NewMemoryStore(db)

	strengths := []int{2, 5, 9}
	for i, s := range strengths {
		mem := &MemoryRow{
			ID:       fmt.Sprintf("mem-str-%d", i),
			Type:     "fact",
			Title:    fmt.Sprintf("Strength %d memory", s),
			Content:  "Content",
			Strength: s,
			IsLatest: 1,
		}
		if err := store.Create(mem); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	// Min strength 5.
	results, err := store.ListByStrength(5, 10)
	if err != nil {
		t.Fatalf("ListByStrength: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("ListByStrength count = %d, want 2", len(results))
	}

	// Should be ordered by strength DESC.
	if results[0].Strength != 9 {
		t.Errorf("first strength = %d, want 9", results[0].Strength)
	}
	if results[1].Strength != 5 {
		t.Errorf("second strength = %d, want 5", results[1].Strength)
	}

	// Min strength 8.
	high, err := store.ListByStrength(8, 10)
	if err != nil {
		t.Fatalf("ListByStrength >= 8: %v", err)
	}
	if len(high) != 1 {
		t.Fatalf("high strength count = %d, want 1", len(high))
	}
}

func TestMemoryStore_ListByConcept(t *testing.T) {
	db := setupTestDB(t)
	store := NewMemoryStore(db)

	memories := []struct {
		id       string
		concepts json.RawMessage
	}{
		{"mem-c1", json.RawMessage(`["golang","testing"]`)},
		{"mem-c2", json.RawMessage(`["golang","concurrency"]`)},
		{"mem-c3", json.RawMessage(`["python","ml"]`)},
	}

	for _, m := range memories {
		mem := &MemoryRow{
			ID:       m.id,
			Type:     "fact",
			Title:    "Concept test",
			Content:  "Content",
			Concepts: m.concepts,
			Strength: 5,
			IsLatest: 1,
		}
		if err := store.Create(mem); err != nil {
			t.Fatalf("Create %s: %v", m.id, err)
		}
	}

	// Search for "golang".
	results, err := store.ListByConcept("golang", 10)
	if err != nil {
		t.Fatalf("ListByConcept golang: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("golang count = %d, want 2", len(results))
	}

	// Search for "testing".
	results, err = store.ListByConcept("testing", 10)
	if err != nil {
		t.Fatalf("ListByConcept testing: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("testing count = %d, want 1", len(results))
	}
	if results[0].ID != "mem-c1" {
		t.Errorf("testing result ID = %q, want mem-c1", results[0].ID)
	}

	// Search for nonexistent concept.
	results, err = store.ListByConcept("rust", 10)
	if err != nil {
		t.Fatalf("ListByConcept rust: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("rust count = %d, want 0", len(results))
	}
}

func TestMemoryStore_Count(t *testing.T) {
	db := setupTestDB(t)
	store := NewMemoryStore(db)

	// Empty.
	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count empty: %v", err)
	}
	if count != 0 {
		t.Errorf("Count empty = %d, want 0", count)
	}

	// Create 2 latest + 1 not latest.
	for i := 0; i < 2; i++ {
		mem := &MemoryRow{
			ID:       fmt.Sprintf("mem-cnt-%d", i),
			Type:     "fact",
			Title:    "Counted",
			Content:  "Content",
			Strength: 5,
			IsLatest: 1,
		}
		if err := store.Create(mem); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	notLatest := &MemoryRow{
		ID:       "mem-cnt-old",
		Type:     "fact",
		Title:    "Not counted",
		Content:  "Old version",
		Strength: 3,
		IsLatest: 0,
	}
	if err := store.Create(notLatest); err != nil {
		t.Fatalf("Create not latest: %v", err)
	}

	count, err = store.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 2 {
		t.Errorf("Count = %d, want 2 (should exclude is_latest=0)", count)
	}
}

func TestMemoryStore_ListExpired(t *testing.T) {
	db := setupTestDB(t)
	store := NewMemoryStore(db)

	pastDate := TimeToString(time.Now().UTC().Add(-24 * time.Hour))
	futureDate := TimeToString(time.Now().UTC().Add(24 * time.Hour))

	// Expired memory.
	expired := &MemoryRow{
		ID:          "mem-expired",
		Type:        "fact",
		Title:       "Expired fact",
		Content:     "This should have been forgotten",
		Strength:    3,
		IsLatest:    1,
		ForgetAfter: &pastDate,
	}
	if err := store.Create(expired); err != nil {
		t.Fatalf("Create expired: %v", err)
	}

	// Not yet expired memory.
	future := &MemoryRow{
		ID:          "mem-future",
		Type:        "fact",
		Title:       "Future fact",
		Content:     "Still valid",
		Strength:    5,
		IsLatest:    1,
		ForgetAfter: &futureDate,
	}
	if err := store.Create(future); err != nil {
		t.Fatalf("Create future: %v", err)
	}

	// Memory without forget_after.
	permanent := &MemoryRow{
		ID:       "mem-permanent",
		Type:     "fact",
		Title:    "Permanent fact",
		Content:  "Never expires",
		Strength: 8,
		IsLatest: 1,
	}
	if err := store.Create(permanent); err != nil {
		t.Fatalf("Create permanent: %v", err)
	}

	results, err := store.ListExpired()
	if err != nil {
		t.Fatalf("ListExpired: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("ListExpired count = %d, want 1", len(results))
	}
	if results[0].ID != "mem-expired" {
		t.Errorf("expired ID = %q, want mem-expired", results[0].ID)
	}
}
