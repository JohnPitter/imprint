package pipeline

import (
	"context"
	"fmt"
	"unicode/utf8"

	"imprint/internal/extract"
	"imprint/internal/llm"
	"imprint/internal/store"

	"github.com/google/uuid"
)

// Compressor transforms raw observations into compressed observations via LLM.
//
// In hybrid mode the compressor runs a deterministic regex pre-pass to extract
// the mechanically-discoverable entities (file paths, concepts, URLs, error
// markers, git refs) before calling the LLM. The LLM is then asked to handle
// only the parts that genuinely need it — title, narrative, importance,
// subtitle, type — which shrinks both the prompt and the response and cuts
// the per-observation Haiku spend roughly in half.
//
// llm-only mode preserves the pre-1.2.0 behavior where the LLM does the
// entire extraction. It exists as an escape hatch in case the regex pass
// loses recall on some unusual observation; flip with IMPRINT_EXTRACTION_MODE.
type Compressor struct {
	provider       llm.LLMProvider
	extractionMode string
}

// NewCompressor creates a new Compressor with the given LLM provider and
// extraction mode ("hybrid" | "llm-only"). Empty mode defaults to "hybrid".
func NewCompressor(provider llm.LLMProvider, mode string) *Compressor {
	if mode == "" {
		mode = "hybrid"
	}
	return &Compressor{provider: provider, extractionMode: mode}
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

	// Hybrid mode: deterministic pre-pass for the extractable entities.
	var prePass extract.Result
	if c.extractionMode != "llm-only" {
		prePass = extract.Extract(toolName, input, output)
	}

	systemPrompt := compressSystemPrompt
	if c.extractionMode != "llm-only" {
		systemPrompt = compressSystemPromptHybrid
	}
	userPrompt := fmt.Sprintf(compressUserPrompt, toolName, input, output)

	// Call LLM.
	resp, err := c.provider.Complete(ctx, llm.CompletionRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    1000,
		Temperature:  0.3,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM compress: %w", err)
	}

	// Parse XML response.
	obsType := getXMLTag(resp, "type")
	if obsType == "" || obsType == "other" {
		// Fallback: use heuristic classification from input/output content.
		obsType = ClassifyMemoryType(input + " " + output)
	}
	title := getXMLTag(resp, "title")
	if title == "" {
		title = "Observation"
	}
	subtitle := getXMLTag(resp, "subtitle")
	narrative := getXMLTag(resp, "narrative")
	facts := getXMLChildren(resp, "facts", "fact")
	importance := getXMLInt(resp, "importance")
	if importance < 1 {
		importance = 5
	}
	if importance > 10 {
		importance = 10
	}

	// Concepts and files: in hybrid mode, prefer the deterministic pre-pass
	// and merge anything extra the LLM still chose to surface. In llm-only
	// mode the LLM is the sole source.
	//
	// URLs and error markers are detected by the pre-pass too but the
	// compressed_observations schema currently has no dedicated columns
	// for them — folding URLs into concepts would conflate two different
	// kinds of references, so they are dropped here and we will revisit
	// once the schema gains url/error tables (see GitHub issue tracker).
	concepts := getXMLChildren(resp, "concepts", "concept")
	files := getXMLChildren(resp, "files", "file")
	if c.extractionMode != "llm-only" {
		concepts = mergeUnique(prePass.Concepts, concepts)
		files = mergeUnique(prePass.Files, files)
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

// mergeUnique appends every item from `extra` that is not already present in
// `primary`, preserving primary's order. Used to fold whatever extra
// concepts/files the LLM still chose to surface in hybrid mode on top of the
// regex pre-pass without losing the regex ordering or duplicating entries.
func mergeUnique(primary, extra []string) []string {
	seen := make(map[string]struct{}, len(primary)+len(extra))
	out := make([]string, 0, len(primary)+len(extra))
	for _, s := range primary {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	for _, s := range extra {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// truncate shortens a string to at most maxLen bytes, appending "..." if
// truncated. Walks back to a rune boundary so multi-byte UTF-8 chars don't
// get cut in half.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	i := maxLen
	for i > 0 && !utf8.RuneStart(s[i]) {
		i--
	}
	return s[:i] + "..."
}
