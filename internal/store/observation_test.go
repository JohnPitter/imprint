package store

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"imprint/internal/types"
)

// createTestSession is a helper that inserts a session so FK constraints are satisfied.
func createTestSession(t *testing.T, db *DB, id, project string) {
	t.Helper()
	store := NewSessionStore(db)
	sess := &SessionRow{
		ID:        id,
		Project:   project,
		Cwd:       "/workspace/" + project,
		StartedAt: time.Now().UTC(),
		Status:    types.SessionActive,
		Tags:      []string{},
	}
	if err := store.Create(sess); err != nil {
		t.Fatalf("createTestSession %s: %v", id, err)
	}
}

func TestObservationStore_CreateRawAndList(t *testing.T) {
	db := setupTestDB(t)
	createTestSession(t, db, "sess-obs-raw", "proj")
	store := NewObservationStore(db)

	baseTime := time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC)
	toolName := "Read"
	prompt := "Show me the file"

	for i := 0; i < 3; i++ {
		obs := &RawObservationRow{
			ID:         fmt.Sprintf("raw-%03d", i+1),
			SessionID:  "sess-obs-raw",
			Timestamp:  baseTime.Add(time.Duration(i) * time.Minute),
			HookType:   "tool_use",
			ToolName:   &toolName,
			ToolInput:  json.RawMessage(`{"path":"/tmp/test.go"}`),
			ToolOutput: json.RawMessage(`{"content":"package main"}`),
			UserPrompt: &prompt,
			Raw:        json.RawMessage(`{"full":"event"}`),
		}
		if err := store.CreateRaw(obs); err != nil {
			t.Fatalf("CreateRaw #%d: %v", i+1, err)
		}
	}

	results, err := store.ListRaw("sess-obs-raw", 10, 0)
	if err != nil {
		t.Fatalf("ListRaw: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("ListRaw len = %d, want 3", len(results))
	}

	// Verify ASC order.
	if results[0].ID != "raw-001" {
		t.Errorf("first raw obs = %q, want raw-001", results[0].ID)
	}
	if results[2].ID != "raw-003" {
		t.Errorf("last raw obs = %q, want raw-003", results[2].ID)
	}

	// Verify fields on first observation.
	first := results[0]
	if first.HookType != "tool_use" {
		t.Errorf("HookType = %q, want tool_use", first.HookType)
	}
	if first.ToolName == nil || *first.ToolName != "Read" {
		t.Errorf("ToolName = %v, want Read", first.ToolName)
	}
	if first.UserPrompt == nil || *first.UserPrompt != prompt {
		t.Errorf("UserPrompt = %v, want %q", first.UserPrompt, prompt)
	}
	if string(first.ToolInput) != `{"path":"/tmp/test.go"}` {
		t.Errorf("ToolInput = %s, want {\"path\":\"/tmp/test.go\"}", first.ToolInput)
	}
}

func TestObservationStore_CreateCompressedAndList(t *testing.T) {
	db := setupTestDB(t)
	createTestSession(t, db, "sess-obs-comp", "proj")
	store := NewObservationStore(db)

	baseTime := time.Date(2026, 4, 13, 11, 0, 0, 0, time.UTC)
	subtitle := "Reading config files"
	narrative := "The agent read configuration files to understand project setup."
	sourceID := "raw-001"

	for i := 0; i < 3; i++ {
		obs := &CompressedObservationRow{
			ID:                  fmt.Sprintf("comp-%03d", i+1),
			SessionID:           "sess-obs-comp",
			Timestamp:           baseTime.Add(time.Duration(i) * time.Minute),
			Type:                "tool_usage",
			Title:               fmt.Sprintf("Read file #%d", i+1),
			Subtitle:            &subtitle,
			Facts:               []string{"read config.yaml", "parsed settings"},
			Narrative:           &narrative,
			Concepts:            []string{"configuration", "yaml"},
			Files:               []string{"config.yaml", "settings.json"},
			Importance:          5 + i,
			Confidence:          0.85,
			SourceObservationID: &sourceID,
		}
		if err := store.CreateCompressed(obs); err != nil {
			t.Fatalf("CreateCompressed #%d: %v", i+1, err)
		}
	}

	results, err := store.ListCompressed("sess-obs-comp", 10, 0)
	if err != nil {
		t.Fatalf("ListCompressed: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("ListCompressed len = %d, want 3", len(results))
	}

	// Verify ASC order.
	if results[0].ID != "comp-001" {
		t.Errorf("first = %q, want comp-001", results[0].ID)
	}

	// Verify fields.
	first := results[0]
	if first.Type != "tool_usage" {
		t.Errorf("Type = %q, want tool_usage", first.Type)
	}
	if first.Title != "Read file #1" {
		t.Errorf("Title = %q, want 'Read file #1'", first.Title)
	}
	if first.Subtitle == nil || *first.Subtitle != subtitle {
		t.Errorf("Subtitle = %v, want %q", first.Subtitle, subtitle)
	}
	if len(first.Facts) != 2 {
		t.Errorf("Facts len = %d, want 2", len(first.Facts))
	}
	if len(first.Concepts) != 2 || first.Concepts[0] != "configuration" {
		t.Errorf("Concepts = %v, want [configuration yaml]", first.Concepts)
	}
	if len(first.Files) != 2 {
		t.Errorf("Files len = %d, want 2", len(first.Files))
	}
	if first.Importance != 5 {
		t.Errorf("Importance = %d, want 5", first.Importance)
	}
	if first.Confidence != 0.85 {
		t.Errorf("Confidence = %f, want 0.85", first.Confidence)
	}
	if first.SourceObservationID == nil || *first.SourceObservationID != sourceID {
		t.Errorf("SourceObservationID = %v, want %q", first.SourceObservationID, sourceID)
	}
}

func TestObservationStore_GetCompressedByID(t *testing.T) {
	db := setupTestDB(t)
	createTestSession(t, db, "sess-get-comp", "proj")
	store := NewObservationStore(db)

	obs := &CompressedObservationRow{
		ID:         "comp-single",
		SessionID:  "sess-get-comp",
		Timestamp:  time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC),
		Type:       "decision",
		Title:      "Chose REST over GraphQL",
		Facts:      []string{"REST is simpler for this use case"},
		Concepts:   []string{"api-design"},
		Files:      []string{},
		Importance: 7,
		Confidence: 0.9,
	}
	if err := store.CreateCompressed(obs); err != nil {
		t.Fatalf("CreateCompressed: %v", err)
	}

	got, err := store.GetCompressedByID("comp-single")
	if err != nil {
		t.Fatalf("GetCompressedByID: %v", err)
	}
	if got.ID != "comp-single" {
		t.Errorf("ID = %q, want comp-single", got.ID)
	}
	if got.Title != "Chose REST over GraphQL" {
		t.Errorf("Title = %q, want 'Chose REST over GraphQL'", got.Title)
	}
	if got.Importance != 7 {
		t.Errorf("Importance = %d, want 7", got.Importance)
	}

	// Not found case.
	_, err = store.GetCompressedByID("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent compressed observation")
	}
}

func TestObservationStore_CountBySession(t *testing.T) {
	db := setupTestDB(t)
	createTestSession(t, db, "sess-count", "proj")
	store := NewObservationStore(db)

	// Empty count.
	count, err := store.CountBySession("sess-count")
	if err != nil {
		t.Fatalf("CountBySession empty: %v", err)
	}
	if count != 0 {
		t.Errorf("CountBySession empty = %d, want 0", count)
	}

	// Insert 2 raw observations.
	for i := 0; i < 2; i++ {
		obs := &RawObservationRow{
			ID:        fmt.Sprintf("raw-count-%d", i),
			SessionID: "sess-count",
			Timestamp: time.Now().UTC(),
			HookType:  "notification",
		}
		if err := store.CreateRaw(obs); err != nil {
			t.Fatalf("CreateRaw: %v", err)
		}
	}

	count, err = store.CountBySession("sess-count")
	if err != nil {
		t.Fatalf("CountBySession: %v", err)
	}
	if count != 2 {
		t.Errorf("CountBySession = %d, want 2", count)
	}
}

func TestObservationStore_ListCompressedByImportance(t *testing.T) {
	db := setupTestDB(t)
	createTestSession(t, db, "sess-imp", "proj-importance")
	store := NewObservationStore(db)

	importanceLevels := []int{3, 5, 8}
	for i, imp := range importanceLevels {
		obs := &CompressedObservationRow{
			ID:         fmt.Sprintf("imp-%d", i),
			SessionID:  "sess-imp",
			Timestamp:  time.Date(2026, 4, 13, 10, i, 0, 0, time.UTC),
			Type:       "observation",
			Title:      fmt.Sprintf("Obs with importance %d", imp),
			Facts:      []string{},
			Concepts:   []string{},
			Files:      []string{},
			Importance: imp,
			Confidence: 0.8,
		}
		if err := store.CreateCompressed(obs); err != nil {
			t.Fatalf("CreateCompressed importance=%d: %v", imp, err)
		}
	}

	// Filter by importance >= 7.
	results, err := store.ListCompressedByImportance("proj-importance", 7, 10)
	if err != nil {
		t.Fatalf("ListCompressedByImportance: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("filtered count = %d, want 1", len(results))
	}
	if results[0].Importance != 8 {
		t.Errorf("Importance = %d, want 8", results[0].Importance)
	}
	if results[0].ID != "imp-2" {
		t.Errorf("ID = %q, want imp-2", results[0].ID)
	}

	// Filter by importance >= 4 should return 2.
	results, err = store.ListCompressedByImportance("proj-importance", 4, 10)
	if err != nil {
		t.Fatalf("ListCompressedByImportance >= 4: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("filtered count >= 4 = %d, want 2", len(results))
	}
}

func TestObservationStore_DeleteRawOlderThan(t *testing.T) {
	db := setupTestDB(t)
	createTestSession(t, db, "sess-del", "proj")
	store := NewObservationStore(db)

	cutoff := time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)

	// Old observation (before cutoff).
	old := &RawObservationRow{
		ID:        "raw-old",
		SessionID: "sess-del",
		Timestamp: time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
		HookType:  "tool_use",
	}
	if err := store.CreateRaw(old); err != nil {
		t.Fatalf("CreateRaw old: %v", err)
	}

	// New observation (after cutoff).
	newObs := &RawObservationRow{
		ID:        "raw-new",
		SessionID: "sess-del",
		Timestamp: time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC),
		HookType:  "notification",
	}
	if err := store.CreateRaw(newObs); err != nil {
		t.Fatalf("CreateRaw new: %v", err)
	}

	deleted, err := store.DeleteRawOlderThan(cutoff)
	if err != nil {
		t.Fatalf("DeleteRawOlderThan: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted count = %d, want 1", deleted)
	}

	// Verify only the new observation remains.
	remaining, err := store.ListRaw("sess-del", 10, 0)
	if err != nil {
		t.Fatalf("ListRaw after delete: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("remaining count = %d, want 1", len(remaining))
	}
	if remaining[0].ID != "raw-new" {
		t.Errorf("remaining ID = %q, want raw-new", remaining[0].ID)
	}
}

func TestObservationStore_InsertDedup(t *testing.T) {
	db := setupTestDB(t)
	store := NewObservationStore(db)

	hash := "sha256:abc123def456"

	// First insert should return true (new entry).
	inserted, err := store.InsertDedup(hash)
	if err != nil {
		t.Fatalf("InsertDedup first: %v", err)
	}
	if !inserted {
		t.Error("first InsertDedup should return true")
	}

	// Second insert with same hash should return false (duplicate).
	inserted, err = store.InsertDedup(hash)
	if err != nil {
		t.Fatalf("InsertDedup second: %v", err)
	}
	if inserted {
		t.Error("second InsertDedup should return false (duplicate)")
	}

	// Different hash should return true.
	inserted, err = store.InsertDedup("sha256:different")
	if err != nil {
		t.Fatalf("InsertDedup different: %v", err)
	}
	if !inserted {
		t.Error("InsertDedup with different hash should return true")
	}
}
