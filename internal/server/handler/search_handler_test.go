package handler

import (
	"net/http"
	"testing"
	"time"

	"imprint/internal/search"
	"imprint/internal/service"
	"imprint/internal/store"
	"imprint/internal/types"
)

// setupSearchHandlerTest creates a full stack (DB + BM25 + services + handler)
// with a session and indexed observations ready for search tests.
func setupSearchHandlerTest(t *testing.T) *SearchHandler {
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

	c := service.NewContainer(db)

	// Create a session.
	err = c.Sessions.Create(&store.SessionRow{
		ID:        "ses_handler1",
		Project:   "proj-handler",
		Cwd:       "/tmp/test",
		StartedAt: time.Now(),
		Status:    types.SessionActive,
	})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Insert compressed observations and index them in BM25.
	narrative := "Implemented retry logic with exponential backoff for network calls"
	obs := &store.CompressedObservationRow{
		ID:         "obs_h1",
		SessionID:  "ses_handler1",
		Timestamp:  time.Now(),
		Type:       "decision",
		Title:      "Retry with backoff",
		Narrative:  &narrative,
		Concepts:   []string{"retry", "backoff", "networking"},
		Files:      []string{"client.go"},
		Importance: 8,
		Confidence: 0.9,
	}
	if err := c.Observations.CreateCompressed(obs); err != nil {
		t.Fatalf("CreateCompressed: %v", err)
	}

	doc := search.IndexDocument{
		ID:        obs.ID,
		SessionID: obs.SessionID,
		Title:     obs.Title,
		Narrative: narrative,
		Concepts:  search.JoinStrings(obs.Concepts),
		Files:     search.JoinStrings(obs.Files),
		Type:      obs.Type,
	}
	if err := bm25.Index(doc); err != nil {
		t.Fatalf("BM25 Index: %v", err)
	}

	// Create a summary for context endpoint.
	err = c.Summaries.Create(&store.SummaryRow{
		SessionID:        "ses_handler1",
		Project:          "proj-handler",
		CreatedAt:        store.TimeToString(time.Now()),
		Title:            "Network layer setup",
		Narrative:        "Added HTTP client with retry and backoff",
		ObservationCount: 3,
	})
	if err != nil {
		t.Fatalf("Create summary: %v", err)
	}

	searchSvc := service.NewSearchService(c, searcher)
	contextSvc := service.NewContextService(c, 10000)

	return NewSearchHandler(searchSvc, contextSvc)
}

// ---------------------------------------------------------------------------
// HandleSearch
// ---------------------------------------------------------------------------

func TestSearchHandler_Search(t *testing.T) {
	h := setupSearchHandlerTest(t)

	rec := postJSON(h.HandleSearch, "/imprint/search", map[string]any{
		"query": "retry backoff",
		"limit": 10,
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("HandleSearch: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := decodeBody(t, rec)

	results, ok := body["results"].([]any)
	if !ok {
		t.Fatal("response missing 'results' array")
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 search result")
	}

	count, ok := body["count"].(float64)
	if !ok {
		t.Fatal("response missing 'count' field")
	}
	if int(count) != len(results) {
		t.Errorf("count (%d) does not match results length (%d)", int(count), len(results))
	}

	// Verify first result has expected fields.
	first := results[0].(map[string]any)
	if first["id"] != "obs_h1" {
		t.Errorf("expected first result ID obs_h1, got %v", first["id"])
	}
	if first["title"] != "Retry with backoff" {
		t.Errorf("expected title 'Retry with backoff', got %v", first["title"])
	}
}

func TestSearchHandler_SearchMissingQuery(t *testing.T) {
	h := setupSearchHandlerTest(t)

	rec := postJSON(h.HandleSearch, "/imprint/search", map[string]any{
		"query": "",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty query, got %d: %s", rec.Code, rec.Body.String())
	}

	body := decodeBody(t, rec)
	if body["error"] == nil {
		t.Error("expected error message in response")
	}
}

// ---------------------------------------------------------------------------
// HandleContext
// ---------------------------------------------------------------------------

func TestSearchHandler_Context(t *testing.T) {
	h := setupSearchHandlerTest(t)

	rec := postJSON(h.HandleContext, "/imprint/context", map[string]any{
		"sessionId": "ses_handler1",
		"project":   "proj-handler",
		"budget":    10000,
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("HandleContext: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := decodeBody(t, rec)

	contextStr, ok := body["context"].(string)
	if !ok {
		t.Fatal("response missing 'context' string")
	}
	if contextStr == "" {
		t.Error("expected non-empty context string")
	}

	blocks, ok := body["blocks"].([]any)
	if !ok {
		t.Fatal("response missing 'blocks' array")
	}
	if len(blocks) == 0 {
		t.Error("expected at least 1 context block")
	}
}

// ---------------------------------------------------------------------------
// HandleEnrich
// ---------------------------------------------------------------------------

func TestSearchHandler_Enrich(t *testing.T) {
	h := setupSearchHandlerTest(t)

	rec := postJSON(h.HandleEnrich, "/imprint/enrich", map[string]any{
		"sessionId": "ses_handler1",
		"files":     []string{"client.go"},
		"terms":     []string{"retry"},
		"toolName":  "Read",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("HandleEnrich: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := decodeBody(t, rec)

	// Context string may or may not have results depending on BM25 matching files.
	if _, ok := body["context"]; !ok {
		t.Fatal("response missing 'context' field")
	}
}

func TestSearchHandler_EnrichEmpty(t *testing.T) {
	h := setupSearchHandlerTest(t)

	rec := postJSON(h.HandleEnrich, "/imprint/enrich", map[string]any{
		"sessionId": "ses_handler1",
		"files":     []string{},
		"terms":     []string{},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("HandleEnrich empty: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := decodeBody(t, rec)
	if body["context"] != "" {
		t.Errorf("expected empty context for empty files/terms, got %v", body["context"])
	}
}
