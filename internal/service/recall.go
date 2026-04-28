package service

import (
	"context"
	"fmt"
	"strings"

	"imprint/internal/llm"
)

// RecallSource is one piece of evidence cited in a recall answer.
type RecallSource struct {
	ID        string  `json:"id"`
	SessionID string  `json:"sessionId"`
	Type      string  `json:"type"`
	Title     string  `json:"title"`
	Score     float64 `json:"score"`
}

// RecallResult is what /imprint/recall returns to the UI.
type RecallResult struct {
	Answer  string         `json:"answer"`
	Sources []RecallSource `json:"sources"`
	Used    int            `json:"used"`
	Skipped string         `json:"skipped"` // why the LLM step was skipped, if any
}

// RecallService answers natural-language questions by retrieving relevant
// observations via SearchService and asking the configured LLM to synthesise
// an answer with citations.
type RecallService struct {
	search *SearchService
	llm    llm.LLMProvider
}

// NewRecallService wires the dependencies. The LLM provider can be nil — in
// that case Recall still returns sources without a synthesised answer.
func NewRecallService(searchSvc *SearchService, provider llm.LLMProvider) *RecallService {
	return &RecallService{search: searchSvc, llm: provider}
}

const recallSystemPrompt = `You answer questions about a developer's past coding sessions using the provided context. Be concise and direct.

Rules:
- Use ONLY the supplied context. If the context doesn't answer the question, say so plainly.
- Cite sources by their bracketed number (e.g. "switched to Drizzle [3]").
- Plain text only — no markdown headings, no bullet lists unless the answer is naturally a list.
- Maximum 6 sentences for the answer.`

const recallUserPromptTpl = `Question: %s

Context:
%s

Answer:`

// Recall searches for relevant context, asks the LLM to synthesise an answer,
// and returns both the answer and the sources used. If the LLM provider isn't
// available, it still returns the matched sources so the UI can render them.
func (s *RecallService) Recall(ctx context.Context, query string, limit int) (*RecallResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if limit <= 0 || limit > 25 {
		limit = 8
	}

	hits, err := s.search.Search(query, limit)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	sources := make([]RecallSource, 0, len(hits))
	for _, h := range hits {
		sources = append(sources, RecallSource{
			ID:        h.ID,
			SessionID: h.SessionID,
			Type:      h.Type,
			Title:     h.Title,
			Score:     h.Score,
		})
	}

	res := &RecallResult{Sources: sources, Used: len(sources)}

	if len(sources) == 0 {
		res.Answer = "No relevant context found for that question."
		res.Used = 0
		return res, nil
	}

	if s.llm == nil || !s.llm.Available() {
		res.Skipped = "no LLM provider configured"
		return res, nil
	}

	ctxBlock := buildRecallContext(hits)
	answer, err := s.llm.Complete(ctx, llm.CompletionRequest{
		SystemPrompt: recallSystemPrompt,
		UserPrompt:   fmt.Sprintf(recallUserPromptTpl, query, ctxBlock),
		MaxTokens:    600,
		Temperature:  0.2,
	})
	if err != nil {
		// Surface the failure but still return sources so the UI isn't empty.
		res.Skipped = "LLM call failed: " + err.Error()
		return res, nil
	}

	res.Answer = strings.TrimSpace(answer)
	return res, nil
}

// buildRecallContext renders the search hits as a numbered, plaintext block.
// Each source is small (title + narrative + concepts/files) so the prompt
// stays well under the model's context window even with limit=25.
func buildRecallContext(hits []SearchResultItem) string {
	var sb strings.Builder
	for i, h := range hits {
		fmt.Fprintf(&sb, "[%d] (%s) %s\n", i+1, h.Type, h.Title)
		if h.Narrative != nil && *h.Narrative != "" {
			fmt.Fprintf(&sb, "    %s\n", trimRunes(*h.Narrative, 600))
		}
		if len(h.Concepts) > 0 {
			fmt.Fprintf(&sb, "    concepts: %s\n", strings.Join(h.Concepts, ", "))
		}
		if len(h.Files) > 0 && len(h.Files) <= 5 {
			fmt.Fprintf(&sb, "    files: %s\n", strings.Join(h.Files, ", "))
		}
	}
	return sb.String()
}

// trimRunes truncates a string to at most max bytes without splitting a
// multi-byte UTF-8 sequence.
func trimRunes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	for i := max; i > 0; i-- {
		if (s[i] & 0xC0) != 0x80 {
			return s[:i]
		}
	}
	return s[:max]
}
