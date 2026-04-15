package search

import "sort"

type HybridResult struct {
	ID        string  `json:"id"`
	SessionID string  `json:"sessionId"`
	Score     float64 `json:"score"`
	BM25Score float64 `json:"bm25Score"`
	VecScore  float64 `json:"vecScore"`
}

type HybridSearcher struct {
	bm25       *BM25Index
	vectors    *VectorIndex
	bm25Weight float64
	vecWeight  float64
}

func NewHybridSearcher(bm25 *BM25Index, vectors *VectorIndex, bm25W, vecW float64) *HybridSearcher {
	return &HybridSearcher{bm25: bm25, vectors: vectors, bm25Weight: bm25W, vecWeight: vecW}
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

	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })

	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

func (h *HybridSearcher) SearchBM25Only(query string, limit int) []HybridResult {
	return h.Search(query, nil, limit)
}
