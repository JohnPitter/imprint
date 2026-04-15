package pipeline

import (
	"context"
	"fmt"

	"imprint/internal/llm"
	"imprint/internal/store"

	"github.com/google/uuid"
)

// Compressor transforms raw observations into compressed observations via LLM.
type Compressor struct {
	provider llm.LLMProvider
}

// NewCompressor creates a new Compressor with the given LLM provider.
func NewCompressor(provider llm.LLMProvider) *Compressor {
	return &Compressor{provider: provider}
}

// Compress takes a raw observation and produces a compressed observation via LLM.
func (c *Compressor) Compress(ctx context.Context, raw *store.RawObservationRow) (*store.CompressedObservationRow, error) {
	// Build user prompt from raw observation fields.
	toolName := ""
	if raw.ToolName != nil {
		toolName = *raw.ToolName
	}
	input := truncate(string(raw.ToolInput), 2000)
	output := truncate(string(raw.ToolOutput), 2000)

	userPrompt := fmt.Sprintf(compressUserPrompt, toolName, input, output)

	// Call LLM.
	resp, err := c.provider.Complete(ctx, llm.CompletionRequest{
		SystemPrompt: compressSystemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    1000,
		Temperature:  0.3,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM compress: %w", err)
	}

	// Parse XML response.
	obsType := getXMLTag(resp, "type")
	if obsType == "" {
		obsType = "other"
	}
	title := getXMLTag(resp, "title")
	if title == "" {
		title = "Observation"
	}
	subtitle := getXMLTag(resp, "subtitle")
	narrative := getXMLTag(resp, "narrative")
	facts := getXMLChildren(resp, "facts", "fact")
	concepts := getXMLChildren(resp, "concepts", "concept")
	files := getXMLChildren(resp, "files", "file")
	importance := getXMLInt(resp, "importance")
	if importance < 1 {
		importance = 5
	}
	if importance > 10 {
		importance = 10
	}

	// Build CompressedObservationRow.
	id := "cobs_" + uuid.New().String()[:8]

	var subtitlePtr *string
	if subtitle != "" {
		subtitlePtr = &subtitle
	}
	var narrativePtr *string
	if narrative != "" {
		narrativePtr = &narrative
	}
	sourceID := raw.ID

	compressed := &store.CompressedObservationRow{
		ID:                  id,
		SessionID:           raw.SessionID,
		Timestamp:           raw.Timestamp,
		Type:                obsType,
		Title:               title,
		Subtitle:            subtitlePtr,
		Facts:               facts,
		Narrative:           narrativePtr,
		Concepts:            concepts,
		Files:               files,
		Importance:          importance,
		Confidence:          0.8,
		SourceObservationID: &sourceID,
	}

	return compressed, nil
}

// truncate shortens a string to maxLen characters, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
