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
type SearchResultItem struct {
	ID        string   `json:"id"`
	SessionID string   `json:"sessionId"`
	Score     float64  `json:"score"`
	Title     string   `json:"title"`
	Type      string   `json:"type"`
	Narrative *string  `json:"narrative"`
	Concepts  []string `json:"concepts"`
	Files     []string `json:"files"`
}

// Search performs hybrid search and enriches results with observation data.
func (s *SearchService) Search(query string, limit int) ([]SearchResultItem, error) {
	if limit <= 0 {
		limit = 20
	}

	results := s.searcher.SearchBM25Only(query, limit)
	items := make([]SearchResultItem, 0, len(results))

	for _, r := range results {
		obs, err := s.c.Observations.GetCompressedByID(r.ID)
		if err != nil {
			continue
		}
		items = append(items, SearchResultItem{
			ID:        r.ID,
			SessionID: r.SessionID,
			Score:     r.Score,
			Title:     obs.Title,
			Type:      obs.Type,
			Narrative: obs.Narrative,
			Concepts:  obs.Concepts,
			Files:     obs.Files,
		})
	}

	return items, nil
}
