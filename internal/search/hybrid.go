package search

import (
	"math"
	"sort"
)

type HybridResult struct {
	ID            string  `json:"id"`
	SessionID     string  `json:"sessionId"`
	Score         float64 `json:"score"`
	BM25Score     float64 `json:"bm25Score"`
	VecScore      float64 `json:"vecScore"`
	BacklinkBoost float64 `json:"backlinkBoost"`
}

// BacklinkProvider returns the number of knowledge-graph edges pointing at
// each candidate id in a single batched call. Implementations are expected
// to return 0 (not an error) for ids the graph has never seen — a freshly
// compressed memory is the common case.
//
// The provider is optional: if nil, hybrid search behaves exactly like
// before. This keeps the integration safe to ship before the graph layer
// has any data.
type BacklinkProvider interface {
	InDegrees(ids []string) map[string]int
}

type HybridSearcher struct {
	bm25         *BM25Index
	vectors      *VectorIndex
	bm25Weight   float64
	vecWeight    float64
	graphWeight  float64
	backlinks    BacklinkProvider
}

func NewHybridSearcher(bm25 *BM25Index, vectors *VectorIndex, bm25W, vecW float64) *HybridSearcher {
	return &HybridSearcher{bm25: bm25, vectors: vectors, bm25Weight: bm25W, vecWeight: vecW}
}

// SetBacklinkProvider attaches a knowledge-graph in-degree provider. After it
// is set, every hit gets multiplied by `1 + graphWeight * log(1 + inDegree)`,
// so a memory referenced by N other nodes ranks higher than an isolated
// memory with the same BM25+vector score. Set graphWeight to 0 to disable
// the boost without removing the provider.
func (h *HybridSearcher) SetBacklinkProvider(p BacklinkProvider, graphWeight float64) {
	h.backlinks = p
	h.graphWeight = graphWeight
}

func (h *HybridSearcher) Search(query string, queryEmb []float32, limit int) []HybridResult {
	if limit <= 0 {
		limit = 20
	}

	bm25Hits, _ := h.bm25.Search(query, limit*2)

	var vecHits []SearchHit
	if len(queryEmb) > 0 && h.vectors.Count() > 0 {
		vecHits = h.vectors.Search(queryEmb, limit*2)
	}

	const k = 60
	scores := map[string]*HybridResult{}

	for rank, hit := range bm25Hits {
		r, ok := scores[hit.ID]
		if !ok {
			r = &HybridResult{ID: hit.ID, SessionID: hit.SessionID}
			scores[hit.ID] = r
		}
		r.BM25Score = hit.Score
		r.Score += h.bm25Weight * (1.0 / float64(k+rank+1))
	}

	for rank, hit := range vecHits {
		r, ok := scores[hit.ID]
		if !ok {
			r = &HybridResult{ID: hit.ID, SessionID: hit.SessionID}
			scores[hit.ID] = r
		}
		r.VecScore = hit.Score
		r.Score += h.vecWeight * (1.0 / float64(k+rank+1))
	}

	results := make([]HybridResult, 0, len(scores))
	for _, r := range scores {
		results = append(results, *r)
	}

	// Apply backlink boost if a provider is configured. Memories that are
	// referenced by many graph nodes get a logarithmic bonus on top of the
	// fused BM25+vector score. log(1+x) keeps the bonus diminishing — the
	// difference between 5 and 10 backlinks matters less than the
	// difference between 0 and 5, which matches the intuition that any
	// graph presence is much more meaningful than zero.
	if h.backlinks != nil && h.graphWeight > 0 && len(results) > 0 {
		ids := make([]string, len(results))
		for i, r := range results {
			ids[i] = r.ID
		}
		degrees := h.backlinks.InDegrees(ids)
		for i := range results {
			deg := degrees[results[i].ID]
			if deg <= 0 {
				continue
			}
			boost := h.graphWeight * math.Log1p(float64(deg))
			results[i].BacklinkBoost = boost
			results[i].Score *= (1.0 + boost)
		}
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })

	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

func (h *HybridSearcher) SearchBM25Only(query string, limit int) []HybridResult {
	return h.Search(query, nil, limit)
}
