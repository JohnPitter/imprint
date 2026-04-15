package service

import (
	"encoding/json"
	"testing"
	"time"

	"imprint/internal/search"
	"imprint/internal/store"
	"imprint/internal/types"
)

// ---------------------------------------------------------------------------
// SearchService
// ---------------------------------------------------------------------------

func setupSearchTest(t *testing.T) (*Container, *search.BM25Index, *search.HybridSearcher) {
	t.Helper()
	dir := t.TempDir()

	db, err := store.Open(dir)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	bm25, err := search.NewBM25Index(dir)
	if err != nil {
		t.Fatalf("failed to create BM25 index: %v", err)
	}
	t.Cleanup(func() { bm25.Close() })

	vectors := search.NewVectorIndex()
	searcher := search.NewHybridSearcher(bm25, vectors, 1.0, 0.5)

	c := NewContainer(db)
	return c, bm25, searcher
}

func createTestSession(t *testing.T, c *Container, id, project string) {
	t.Helper()
	err := c.Sessions.Create(&store.SessionRow{
		ID:        id,
		Project:   project,
		Cwd:       "/tmp/test",
		StartedAt: time.Now(),
		Status:    types.SessionActive,
	})
	if err != nil {
		t.Fatalf("failed to create test session: %v", err)
	}
}

func TestSearchService_Search(t *testing.T) {
	c, bm25, searcher := setupSearchTest(t)

	// Create a session (required FK for compressed observations).
	createTestSession(t, c, "ses_search1", "proj-search")

	narrative1 := "Implemented error handling with custom error types"
	narrative2 := "Added database migration support with embedded SQL files"

	// Insert compressed observations into the DB.
	obs := []*store.CompressedObservationRow{
		{
			ID:         "obs1",
			SessionID:  "ses_search1",
			Timestamp:  time.Now(),
			Type:       "decision",
			Title:      "Error handling patterns",
			Narrative:  &narrative1,
			Concepts:   []string{"error-handling", "go"},
			Files:      []string{"main.go"},
			Importance: 8,
			Confidence: 0.9,
		},
		{
			ID:         "obs2",
			SessionID:  "ses_search1",
			Timestamp:  time.Now(),
			Type:       "discovery",
			Title:      "Database migration strategy",
			Narrative:  &narrative2,
			Concepts:   []string{"database", "migrations"},
			Files:      []string{"db.go"},
			Importance: 7,
			Confidence: 0.85,
		},
	}

	for _, o := range obs {
		if err := c.Observations.CreateCompressed(o); err != nil {
			t.Fatalf("CreateCompressed(%s): %v", o.ID, err)
		}

		// Also index into BM25.
		doc := search.IndexDocument{
			ID:        o.ID,
			SessionID: o.SessionID,
			Title:     o.Title,
			Type:      o.Type,
		}
		if o.Narrative != nil {
			doc.Narrative = *o.Narrative
		}
		if err := bm25.Index(doc); err != nil {
			t.Fatalf("BM25 Index(%s): %v", o.ID, err)
		}
	}

	svc := NewSearchService(c, searcher)

	results, err := svc.Search("error handling", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result, got 0")
	}

	// The first result should be obs1 (best match for "error handling").
	if results[0].ID != "obs1" {
		t.Errorf("expected first result to be obs1, got %s", results[0].ID)
	}
	if results[0].Title != "Error handling patterns" {
		t.Errorf("expected title 'Error handling patterns', got %s", results[0].Title)
	}
	if results[0].Type != "decision" {
		t.Errorf("expected type 'decision', got %s", results[0].Type)
	}
	if results[0].Score <= 0 {
		t.Errorf("expected positive score, got %f", results[0].Score)
	}
}

func TestSearchService_SearchNoResults(t *testing.T) {
	c, _, searcher := setupSearchTest(t)

	createTestSession(t, c, "ses_empty", "proj-empty")

	svc := NewSearchService(c, searcher)

	results, err := svc.Search("nonexistent term xyzzy", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for nonexistent term, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// ContextService
// ---------------------------------------------------------------------------

func TestContextService_BuildContext(t *testing.T) {
	c := setupTestContainer(t)

	// Create session.
	createTestSession(t, c, "ses_ctx1", "proj-ctx")

	// Insert a summary for the session.
	err := c.Summaries.Create(&store.SummaryRow{
		SessionID:        "ses_ctx1",
		Project:          "proj-ctx",
		CreatedAt:        store.TimeToString(time.Now()),
		Title:            "Setup phase",
		Narrative:        "Initialized project structure and dependencies",
		ObservationCount: 5,
	})
	if err != nil {
		t.Fatalf("Create summary: %v", err)
	}

	// Insert high-importance compressed observations.
	narrative := "Critical architectural decision about database layer"
	err = c.Observations.CreateCompressed(&store.CompressedObservationRow{
		ID:         "obs_ctx1",
		SessionID:  "ses_ctx1",
		Timestamp:  time.Now(),
		Type:       "decision",
		Title:      "Use SQLite for storage",
		Narrative:  &narrative,
		Importance: 9,
		Confidence: 0.95,
	})
	if err != nil {
		t.Fatalf("Create compressed observation: %v", err)
	}

	// Insert a strong memory.
	err = c.Memories.Create(&store.MemoryRow{
		ID:       "mem_ctx1",
		Type:     "pattern",
		Title:    "Error wrapping convention",
		Content:  "Always use fmt.Errorf with %%w for error wrapping",
		Strength: 8,
		IsLatest: 1,
	})
	if err != nil {
		t.Fatalf("Create memory: %v", err)
	}

	svc := NewContextService(c, 10000)

	blocks, err := svc.BuildContext("ses_ctx1", "proj-ctx", 0)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}

	if len(blocks) == 0 {
		t.Fatal("expected at least 1 context block, got 0")
	}

	// Verify all 3 block types are present.
	blockTypes := make(map[string]bool)
	for _, b := range blocks {
		blockTypes[b.Type] = true
	}

	expectedTypes := []string{"session-history", "key-observations", "memories"}
	for _, et := range expectedTypes {
		if !blockTypes[et] {
			t.Errorf("expected block type %q to be present, but it was not found", et)
		}
	}

	// Verify priorities are set correctly.
	for _, b := range blocks {
		switch b.Type {
		case "session-history":
			if b.Priority != 1 {
				t.Errorf("session-history priority should be 1, got %d", b.Priority)
			}
			if b.Label != "Recent Sessions" {
				t.Errorf("session-history label should be 'Recent Sessions', got %s", b.Label)
			}
		case "key-observations":
			if b.Priority != 2 {
				t.Errorf("key-observations priority should be 2, got %d", b.Priority)
			}
		case "memories":
			if b.Priority != 3 {
				t.Errorf("memories priority should be 3, got %d", b.Priority)
			}
		}
	}
}

func TestContextService_BuildContextEmpty(t *testing.T) {
	c := setupTestContainer(t)

	svc := NewContextService(c, 10000)

	blocks, err := svc.BuildContext("ses_nonexistent", "proj-empty-ctx", 0)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}

	if len(blocks) != 0 {
		t.Errorf("expected 0 blocks for empty project, got %d", len(blocks))
	}
}

// Ensure json import is used (needed for MemoryRow fields).
var _ = json.RawMessage{}
