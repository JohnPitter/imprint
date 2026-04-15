package search

import (
	"math"
	"sort"
	"sync"
)

type VectorEntry struct {
	ID        string
	SessionID string
	Embedding []float32
}

type VectorIndex struct {
	mu      sync.RWMutex
	entries map[string]*VectorEntry
}

func NewVectorIndex() *VectorIndex {
	return &VectorIndex{entries: make(map[string]*VectorEntry)}
}

func (v *VectorIndex) Add(id, sessionID string, embedding []float32) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.entries[id] = &VectorEntry{id, sessionID, embedding}
}

func (v *VectorIndex) Remove(id string) {
	v.mu.Lock()
	delete(v.entries, id)
	v.mu.Unlock()
}

func (v *VectorIndex) Search(query []float32, limit int) []SearchHit {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	type scored struct {
		id    string
		sid   string
		score float64
	}

	results := make([]scored, 0, len(v.entries))
	for _, e := range v.entries {
		sim := cosineSimilarity(query, e.Embedding)
		if sim > 0 {
			results = append(results, scored{e.ID, e.SessionID, float64(sim)})
		}
	}

	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })

	if len(results) > limit {
		results = results[:limit]
	}

	hits := make([]SearchHit, len(results))
	for i, r := range results {
		hits[i] = SearchHit{r.id, r.sid, r.score}
	}

	return hits
}

func (v *VectorIndex) Count() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.entries)
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, nA, nB float32
	for i := range a {
		dot += a[i] * b[i]
		nA += a[i] * a[i]
		nB += b[i] * b[i]
	}

	d := float32(math.Sqrt(float64(nA)) * math.Sqrt(float64(nB)))
	if d == 0 {
		return 0
	}

	return dot / d
}
