package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"imprint/internal/llm"
	"imprint/internal/store"
)

// Summarizer produces session summaries from compressed observations via LLM.
type Summarizer struct {
	provider llm.LLMProvider
}

// NewSummarizer creates a new Summarizer with the given LLM provider.
func NewSummarizer(provider llm.LLMProvider) *Summarizer {
	return &Summarizer{provider: provider}
}

// Summarize takes compressed observations for a session and produces a summary.
func (s *Summarizer) Summarize(ctx context.Context, sessionID, project string, observations []store.CompressedObservationRow) (*store.SummaryRow, error) {
	if len(observations) == 0 {
		return nil, fmt.Errorf("no observations to summarize")
	}

	// Build observation text for the LLM prompt.
	var sb strings.Builder
	for _, obs := range observations {
		narrative := ""
		if obs.Narrative != nil {
			narrative = *obs.Narrative
		}
		fmt.Fprintf(&sb, "- [%s] %s: %s\n", obs.Type, obs.Title, narrative)
	}

	userPrompt := fmt.Sprintf(summarizeUserPrompt, len(observations), sb.String())

	resp, err := s.provider.Complete(ctx, llm.CompletionRequest{
		SystemPrompt: summarizeSystemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    1500,
		Temperature:  0.3,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM summarize: %w", err)
	}

	// Parse XML response.
	title := getXMLTag(resp, "title")
	if title == "" {
		title = "Session Summary"
	}
	narrative := getXMLTag(resp, "narrative")
	decisions := getXMLChildren(resp, "key_decisions", "decision")
	files := getXMLChildren(resp, "files_modified", "file")
	concepts := getXMLChildren(resp, "concepts", "concept")

	// SummaryRow stores JSON arrays as strings for KeyDecisions, FilesModified, Concepts.
	decisionsJSON, _ := json.Marshal(decisions)
	filesJSON, _ := json.Marshal(files)
	conceptsJSON, _ := json.Marshal(concepts)

	summary := &store.SummaryRow{
		SessionID:        sessionID,
		Project:          project,
		Title:            title,
		Narrative:        narrative,
		KeyDecisions:     string(decisionsJSON),
		FilesModified:    string(filesJSON),
		Concepts:         string(conceptsJSON),
		ObservationCount: len(observations),
	}

	return summary, nil
}
