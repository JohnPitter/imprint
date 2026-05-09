package search

import "testing"

type fakeBacklinks map[string]int

func (f fakeBacklinks) InDegrees(ids []string) map[string]int {
	out := make(map[string]int, len(ids))
	for _, id := range ids {
		out[id] = f[id]
	}
	return out
}

func TestHybridSearcher_BacklinkBoost(t *testing.T) {
	bm25, err := NewBM25Index(t.TempDir())
	if err != nil {
		t.Fatalf("bm25: %v", err)
	}
	defer bm25.Close()
	vectors := NewVectorIndex()

	for _, id := range []string{"a", "b"} {
		if err := bm25.Index(IndexDocument{ID: id, SessionID: "s", Title: "shared title", Narrative: "shared narrative"}); err != nil {
			t.Fatalf("index %s: %v", id, err)
		}
	}

	searcher := NewHybridSearcher(bm25, vectors, 1.0, 0.5)

	// Baseline: no backlink provider — equal scores, equal rank order is by id.
	baseline := searcher.Search("shared", nil, 10)
	if len(baseline) != 2 {
		t.Fatalf("expected 2 results, got %d", len(baseline))
	}

	// Attach a provider that says "b" has 9 backlinks, "a" has 0.
	searcher.SetBacklinkProvider(fakeBacklinks{"b": 9}, 0.5)
	boosted := searcher.Search("shared", nil, 10)

	if len(boosted) != 2 {
		t.Fatalf("expected 2 results after boost, got %d", len(boosted))
	}
	if boosted[0].ID != "b" {
		t.Errorf("expected b to rank first after boost, got %v", boosted)
	}
	if boosted[0].BacklinkBoost <= 0 {
		t.Errorf("expected positive backlink boost, got %f", boosted[0].BacklinkBoost)
	}
	for _, r := range boosted {
		if r.ID == "a" && r.BacklinkBoost != 0 {
			t.Errorf("a should have no boost, got %f", r.BacklinkBoost)
		}
	}
}

func TestHybridSearcher_BacklinkProviderOptional(t *testing.T) {
	bm25, err := NewBM25Index(t.TempDir())
	if err != nil {
		t.Fatalf("bm25: %v", err)
	}
	defer bm25.Close()
	vectors := NewVectorIndex()
	bm25.Index(IndexDocument{ID: "x", SessionID: "s", Title: "x", Narrative: "x"})

	searcher := NewHybridSearcher(bm25, vectors, 1.0, 0.5)
	got := searcher.Search("x", nil, 10)
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if got[0].BacklinkBoost != 0 {
		t.Errorf("expected zero boost when provider is nil, got %f", got[0].BacklinkBoost)
	}
}
