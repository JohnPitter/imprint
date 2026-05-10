package service

import "imprint/internal/search"

// SearchService provides search operations over compressed observations.
type SearchService struct {
	c        *Container
	searcher *search.HybridSearcher
}

// NewSearchService creates a new SearchService.
func NewSearchService(c *Container, searcher *search.HybridSearcher) *SearchService {
	return &SearchService{c: c, searcher: searcher}
}

// SearchResultItem is a single enriched search result.
// Os campos BM25Score e VecScore expõem o breakdown do ranking pra UI
// poder mostrar "por que essa memória ficou no topo". Rank é a posição
// 1-based no resultado original.
type SearchResultItem struct {
	ID        string   `json:"id"`
	SessionID string   `json:"sessionId"`
	Score     float64  `json:"score"`
	BM25Score float64  `json:"bm25Score"`
	VecScore  float64  `json:"vecScore"`
	Rank      int      `json:"rank"`
	Title     string   `json:"title"`
	Type      string   `json:"type"`
	Narrative *string  `json:"narrative"`
	Concepts  []string `json:"concepts"`
	Files     []string `json:"files"`
}

// Search performs hybrid search and enriches results with observation data.
// The query is sanitized first to mitigate system prompt contamination.
func (s *SearchService) Search(query string, limit int) ([]SearchResultItem, error) {
	if limit <= 0 {
		limit = 20
	}

	query = search.SanitizeQuery(query)

	results := s.searcher.SearchBM25Only(query, limit)
	items := make([]SearchResultItem, 0, len(results))

	for i, r := range results {
		obs, err := s.c.Observations.GetCompressedByID(r.ID)
		if err != nil {
			continue
		}
		items = append(items, SearchResultItem{
			ID:        r.ID,
			SessionID: r.SessionID,
			Score:     r.Score,
			BM25Score: r.Score, // BM25Only path: score total = BM25
			VecScore:  0,       // Vector pleno ainda não plugado nesta call
			Rank:      i + 1,
			Title:     obs.Title,
			Type:      obs.Type,
			Narrative: obs.Narrative,
			Concepts:  obs.Concepts,
			Files:     obs.Files,
		})
	}

	return items, nil
}
