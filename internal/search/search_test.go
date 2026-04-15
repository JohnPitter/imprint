package search

import (
	"fmt"
	"math"
	"testing"
)

// ---------------------------------------------------------------------------
// BM25Index
// ---------------------------------------------------------------------------

func TestBM25Index_IndexAndSearch(t *testing.T) {
	idx, err := NewBM25Index(t.TempDir())
	if err != nil {
		t.Fatalf("NewBM25Index: %v", err)
	}
	defer idx.Close()

	docs := []IndexDocument{
		{ID: "doc1", SessionID: "ses1", Title: "Go error handling", Narrative: "Always wrap errors with context using fmt.Errorf", Type: "decision"},
		{ID: "doc2", SessionID: "ses1", Title: "Database migrations", Narrative: "Use embedded SQL files for database migrations", Type: "discovery"},
		{ID: "doc3", SessionID: "ses2", Title: "React component patterns", Narrative: "Use functional components with hooks", Type: "pattern"},
	}

	for _, doc := range docs {
		if err := idx.Index(doc); err != nil {
			t.Fatalf("Index(%s): %v", doc.ID, err)
		}
	}

	hits, err := idx.Search("error handling", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least 1 hit for 'error handling', got 0")
	}

	// The first hit should be doc1 since it matches "error handling" in title.
	if hits[0].ID != "doc1" {
		t.Errorf("expected first hit to be doc1, got %s", hits[0].ID)
	}
	if hits[0].Score <= 0 {
		t.Errorf("expected score > 0, got %f", hits[0].Score)
	}
	if hits[0].SessionID != "ses1" {
		t.Errorf("expected sessionId ses1, got %s", hits[0].SessionID)
	}
}

func TestBM25Index_Remove(t *testing.T) {
	idx, err := NewBM25Index(t.TempDir())
	if err != nil {
		t.Fatalf("NewBM25Index: %v", err)
	}
	defer idx.Close()

	doc := IndexDocument{ID: "doc_rm", SessionID: "ses1", Title: "removable document", Narrative: "this will be removed"}
	if err := idx.Index(doc); err != nil {
		t.Fatalf("Index: %v", err)
	}

	// Verify it exists.
	hits, err := idx.Search("removable", 10)
	if err != nil {
		t.Fatalf("Search before remove: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("expected hit before remove")
	}

	// Remove.
	if err := idx.Remove("doc_rm"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Should no longer appear.
	hits, err = idx.Search("removable", 10)
	if err != nil {
		t.Fatalf("Search after remove: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("expected 0 hits after remove, got %d", len(hits))
	}
}

func TestBM25Index_Count(t *testing.T) {
	idx, err := NewBM25Index(t.TempDir())
	if err != nil {
		t.Fatalf("NewBM25Index: %v", err)
	}
	defer idx.Close()

	count, err := idx.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 docs initially, got %d", count)
	}

	for i := 0; i < 3; i++ {
		doc := IndexDocument{ID: fmt.Sprintf("doc_%d", i), Title: "test doc"}
		if err := idx.Index(doc); err != nil {
			t.Fatalf("Index: %v", err)
		}
	}

	count, err = idx.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 docs, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// VectorIndex
// ---------------------------------------------------------------------------

func TestVectorIndex_AddAndSearch(t *testing.T) {
	vi := NewVectorIndex()

	// doc1: [1, 0, 0] — should be closest to query [1, 0, 0]
	vi.Add("doc1", "ses1", []float32{1, 0, 0})
	// doc2: [0, 1, 0] — orthogonal to query
	vi.Add("doc2", "ses1", []float32{0, 1, 0})
	// doc3: [0.9, 0.1, 0] — close to query
	vi.Add("doc3", "ses2", []float32{0.9, 0.1, 0})

	query := []float32{1, 0, 0}
	hits := vi.Search(query, 10)

	if len(hits) < 2 {
		t.Fatalf("expected at least 2 hits, got %d", len(hits))
	}

	// doc1 should be first (exact match, similarity = 1.0).
	if hits[0].ID != "doc1" {
		t.Errorf("expected first hit to be doc1, got %s", hits[0].ID)
	}
	if math.Abs(hits[0].Score-1.0) > 0.001 {
		t.Errorf("expected score ~1.0 for exact match, got %f", hits[0].Score)
	}

	// doc3 should be second (high similarity).
	if hits[1].ID != "doc3" {
		t.Errorf("expected second hit to be doc3, got %s", hits[1].ID)
	}
	if hits[1].Score <= 0 {
		t.Errorf("expected positive score for doc3, got %f", hits[1].Score)
	}
}

func TestVectorIndex_Remove(t *testing.T) {
	vi := NewVectorIndex()

	vi.Add("v1", "ses1", []float32{1, 0, 0})
	vi.Add("v2", "ses1", []float32{0, 1, 0})

	if vi.Count() != 2 {
		t.Fatalf("expected count 2, got %d", vi.Count())
	}

	vi.Remove("v1")

	if vi.Count() != 1 {
		t.Errorf("expected count 1 after remove, got %d", vi.Count())
	}

	// Search for [1, 0, 0] should not return v1 anymore.
	hits := vi.Search([]float32{1, 0, 0}, 10)
	for _, h := range hits {
		if h.ID == "v1" {
			t.Error("removed entry v1 should not appear in search results")
		}
	}
}

// ---------------------------------------------------------------------------
// CosineSimilarity
// ---------------------------------------------------------------------------

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a, b     []float32
		expected float32
		delta    float32
	}{
		{
			name:     "identical vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{1, 0, 0},
			expected: 1.0,
			delta:    0.001,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{0, 1, 0},
			expected: 0.0,
			delta:    0.001,
		},
		{
			name:     "opposite vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{-1, 0, 0},
			expected: -1.0,
			delta:    0.001,
		},
		{
			name:     "partial similarity",
			a:        []float32{1, 1, 0},
			b:        []float32{1, 0, 0},
			expected: float32(1.0 / math.Sqrt(2)),
			delta:    0.001,
		},
		{
			name:     "different lengths returns 0",
			a:        []float32{1, 0},
			b:        []float32{1, 0, 0},
			expected: 0.0,
			delta:    0.001,
		},
		{
			name:     "empty vectors returns 0",
			a:        []float32{},
			b:        []float32{},
			expected: 0.0,
			delta:    0.001,
		},
		{
			name:     "zero vector returns 0",
			a:        []float32{0, 0, 0},
			b:        []float32{1, 0, 0},
			expected: 0.0,
			delta:    0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cosineSimilarity(tt.a, tt.b)
			if diff := float32(math.Abs(float64(got - tt.expected))); diff > tt.delta {
				t.Errorf("cosineSimilarity(%v, %v) = %f, want %f (diff %f)", tt.a, tt.b, got, tt.expected, diff)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// HybridSearcher
// ---------------------------------------------------------------------------

func TestHybridSearcher_BM25Only(t *testing.T) {
	bm25, err := NewBM25Index(t.TempDir())
	if err != nil {
		t.Fatalf("NewBM25Index: %v", err)
	}
	defer bm25.Close()

	vectors := NewVectorIndex()
	searcher := NewHybridSearcher(bm25, vectors, 1.0, 0.5)

	// Index documents into BM25 only.
	docs := []IndexDocument{
		{ID: "h1", SessionID: "ses1", Title: "Golang concurrency patterns", Narrative: "Use goroutines and channels for concurrency"},
		{ID: "h2", SessionID: "ses1", Title: "Python decorators", Narrative: "Decorators modify function behavior"},
	}
	for _, doc := range docs {
		if err := bm25.Index(doc); err != nil {
			t.Fatalf("Index: %v", err)
		}
	}

	// Search BM25 only (nil embedding).
	results := searcher.SearchBM25Only("concurrency goroutines", 10)
	if len(results) == 0 {
		t.Fatal("expected at least 1 result from BM25-only search")
	}

	// First result should be h1.
	if results[0].ID != "h1" {
		t.Errorf("expected first result to be h1, got %s", results[0].ID)
	}
	if results[0].BM25Score <= 0 {
		t.Errorf("expected positive BM25Score, got %f", results[0].BM25Score)
	}
	if results[0].VecScore != 0 {
		t.Errorf("expected VecScore 0 for BM25-only search, got %f", results[0].VecScore)
	}
}

func TestHybridSearcher_Combined(t *testing.T) {
	bm25, err := NewBM25Index(t.TempDir())
	if err != nil {
		t.Fatalf("NewBM25Index: %v", err)
	}
	defer bm25.Close()

	vectors := NewVectorIndex()
	searcher := NewHybridSearcher(bm25, vectors, 1.0, 1.0)

	// Index doc in BM25 — strong text match for "database".
	bm25Doc := IndexDocument{ID: "combined1", SessionID: "ses1", Title: "Database indexing strategies", Narrative: "Use B-tree indexes for database queries"}
	if err := bm25.Index(bm25Doc); err != nil {
		t.Fatalf("BM25 Index: %v", err)
	}

	// Index doc in vector — close to query embedding.
	vectors.Add("combined1", "ses1", []float32{0.9, 0.1, 0})
	// Another doc only in vectors.
	vectors.Add("combined2", "ses2", []float32{1, 0, 0})

	// Also index combined2 in BM25 with different text.
	bm25Doc2 := IndexDocument{ID: "combined2", SessionID: "ses2", Title: "API design principles", Narrative: "REST API best practices"}
	if err := bm25.Index(bm25Doc2); err != nil {
		t.Fatalf("BM25 Index: %v", err)
	}

	// Search with both text and embedding.
	queryEmb := []float32{1, 0, 0}
	results := searcher.Search("database indexing", queryEmb, 10)

	if len(results) == 0 {
		t.Fatal("expected at least 1 result from combined search")
	}

	// combined1 should rank highly — it matches both BM25 and vector.
	found := false
	for _, r := range results {
		if r.ID == "combined1" {
			found = true
			if r.Score <= 0 {
				t.Errorf("expected positive score for combined1, got %f", r.Score)
			}
			break
		}
	}
	if !found {
		t.Error("combined1 should appear in results (matches both BM25 and vector)")
	}

	// combined2 should also appear (vector match).
	found = false
	for _, r := range results {
		if r.ID == "combined2" {
			found = true
			if r.VecScore <= 0 {
				t.Errorf("expected positive VecScore for combined2, got %f", r.VecScore)
			}
			break
		}
	}
	if !found {
		t.Error("combined2 should appear in results (vector match)")
	}
}
