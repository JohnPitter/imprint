package search

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/blevesearch/bleve/v2"
)

type SearchHit struct {
	ID        string  `json:"id"`
	SessionID string  `json:"sessionId"`
	Score     float64 `json:"score"`
}

type IndexDocument struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionId"`
	Title     string `json:"title"`
	Narrative string `json:"narrative"`
	Facts     string `json:"facts"`
	Concepts  string `json:"concepts"`
	Files     string `json:"files"`
	Type      string `json:"type"`
}

type BM25Index struct {
	index bleve.Index
}

func NewBM25Index(dataDir string) (*BM25Index, error) {
	indexPath := filepath.Join(dataDir, "bleve_index")

	idx, err := bleve.Open(indexPath)
	if err != nil {
		mapping := bleve.NewIndexMapping()

		docMapping := bleve.NewDocumentMapping()

		textField := bleve.NewTextFieldMapping()
		textField.Analyzer = "standard"

		keywordField := bleve.NewKeywordFieldMapping()

		docMapping.AddFieldMappingsAt("title", textField)
		docMapping.AddFieldMappingsAt("narrative", textField)
		docMapping.AddFieldMappingsAt("facts", textField)
		docMapping.AddFieldMappingsAt("concepts", textField)
		docMapping.AddFieldMappingsAt("files", keywordField)
		docMapping.AddFieldMappingsAt("type", keywordField)
		docMapping.AddFieldMappingsAt("sessionId", keywordField)

		mapping.DefaultMapping = docMapping

		idx, err = bleve.New(indexPath, mapping)
		if err != nil {
			return nil, fmt.Errorf("create bleve index: %w", err)
		}
	}

	return &BM25Index{index: idx}, nil
}

func (b *BM25Index) Index(doc IndexDocument) error {
	return b.index.Index(doc.ID, doc)
}

func (b *BM25Index) Search(query string, limit int) ([]SearchHit, error) {
	if limit <= 0 {
		limit = 20
	}

	q := bleve.NewQueryStringQuery(query)
	req := bleve.NewSearchRequestOptions(q, limit, 0, false)
	req.Fields = []string{"sessionId"}

	result, err := b.index.Search(req)
	if err != nil {
		return nil, fmt.Errorf("bleve search: %w", err)
	}

	hits := make([]SearchHit, 0, len(result.Hits))
	for _, hit := range result.Hits {
		sid := ""
		if v, ok := hit.Fields["sessionId"].(string); ok {
			sid = v
		}
		hits = append(hits, SearchHit{ID: hit.ID, SessionID: sid, Score: hit.Score})
	}

	return hits, nil
}

func (b *BM25Index) Remove(id string) error {
	return b.index.Delete(id)
}

func (b *BM25Index) Count() (uint64, error) {
	return b.index.DocCount()
}

func (b *BM25Index) Close() error {
	return b.index.Close()
}

func JoinStrings(s []string) string {
	return strings.Join(s, " ")
}
