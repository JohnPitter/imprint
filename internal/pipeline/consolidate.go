package pipeline

import (
	"context"
	"fmt"
	"strings"

	"imprint/internal/llm"
	"imprint/internal/store"
)

const consolidateSystemPrompt = `You are a memory consolidator. Given a group of related observations, create one or more long-term memories that capture the key patterns, preferences, or facts.

Respond with XML:
<memories>
  <memory>
    <type>pattern|preference|architecture|bug|workflow|fact</type>
    <title>Concise title (max 80 chars)</title>
    <content>Detailed content describing the pattern/preference/fact</content>
    <concepts><concept>concept</concept></concepts>
    <files><file>path/to/file</file></files>
    <strength>1-10 (10=critical, 1=trivial)</strength>
  </memory>
</memories>`

const consolidateUserPrompt = `Consolidate these %d related observations into long-term memories:

%s`

// ConsolidatedMemory represents a memory produced by the consolidation pipeline.
type ConsolidatedMemory struct {
	Type     string // pattern, preference, architecture, bug, workflow, fact
	Title    string
	Content  string
	Concepts []string
	Files    []string
	Strength int
}

// Consolidator groups compressed observations by shared concepts and produces
// long-term memories via LLM.
type Consolidator struct {
	provider llm.LLMProvider
}

// NewConsolidator creates a new Consolidator with the given LLM provider.
func NewConsolidator(provider llm.LLMProvider) *Consolidator {
	return &Consolidator{provider: provider}
}

// Consolidate takes compressed observations, groups by shared concepts, and
// produces memories via LLM. Returns up to 10 groups with up to 8 observations each.
func (c *Consolidator) Consolidate(ctx context.Context, observations []store.CompressedObservationRow) ([]ConsolidatedMemory, error) {
	if len(observations) == 0 {
		return nil, nil
	}

	groups := groupBySharedConcepts(observations)

	var allMemories []ConsolidatedMemory

	for i, group := range groups {
		if i >= 10 {
			break
		}
		// Cap each group at 8 observations.
		if len(group) > 8 {
			group = group[:8]
		}

		memories, err := c.consolidateGroup(ctx, group)
		if err != nil {
			return nil, fmt.Errorf("consolidate group %d: %w", i, err)
		}
		allMemories = append(allMemories, memories...)
	}

	return allMemories, nil
}

// consolidateGroup sends a single group of observations to the LLM and parses the response.
func (c *Consolidator) consolidateGroup(ctx context.Context, group []store.CompressedObservationRow) ([]ConsolidatedMemory, error) {
	var sb strings.Builder
	for _, obs := range group {
		narrative := ""
		if obs.Narrative != nil {
			narrative = *obs.Narrative
		}
		concepts := strings.Join(obs.Concepts, ", ")
		files := strings.Join(obs.Files, ", ")
		fmt.Fprintf(&sb, "- [%s] %s: %s (concepts: %s) (files: %s)\n",
			obs.Type, obs.Title, narrative, concepts, files)
	}

	userPrompt := fmt.Sprintf(consolidateUserPrompt, len(group), sb.String())

	resp, err := c.provider.Complete(ctx, llm.CompletionRequest{
		SystemPrompt: consolidateSystemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    2000,
		Temperature:  0.3,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM consolidate: %w", err)
	}

	return parseConsolidatedMemories(resp), nil
}

// parseConsolidatedMemories extracts memories from the LLM XML response.
func parseConsolidatedMemories(resp string) []ConsolidatedMemory {
	memoriesBlock := getXMLTag(resp, "memories")
	if memoriesBlock == "" {
		memoriesBlock = resp
	}

	// Split into individual <memory> blocks.
	blocks := splitXMLBlocks(memoriesBlock, "memory")
	var result []ConsolidatedMemory

	for _, block := range blocks {
		memType := getXMLTag(block, "type")
		title := getXMLTag(block, "title")
		if title == "" {
			continue
		}
		content := getXMLTag(block, "content")
		if memType == "" {
			// Fallback: use heuristic classification from content.
			memType = ClassifyMemoryType(title + " " + content)
		}
		concepts := getXMLChildren(block, "concepts", "concept")
		files := getXMLChildren(block, "files", "file")
		strength := getXMLInt(block, "strength")
		if strength < 1 {
			strength = 5
		}
		if strength > 10 {
			strength = 10
		}

		result = append(result, ConsolidatedMemory{
			Type:     memType,
			Title:    title,
			Content:  content,
			Concepts: concepts,
			Files:    files,
			Strength: strength,
		})
	}

	return result
}

// groupBySharedConcepts groups observations that share at least one concept.
// Uses union-find for efficient grouping.
func groupBySharedConcepts(observations []store.CompressedObservationRow) [][]store.CompressedObservationRow {
	n := len(observations)
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}

	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}

	// Build concept -> observation indices map.
	conceptIdx := map[string][]int{}
	for i, obs := range observations {
		for _, c := range obs.Concepts {
			conceptIdx[c] = append(conceptIdx[c], i)
		}
	}

	// Union observations sharing a concept.
	for _, indices := range conceptIdx {
		for j := 1; j < len(indices); j++ {
			union(indices[0], indices[j])
		}
	}

	// Collect groups.
	groupMap := map[int][]int{}
	for i := range observations {
		root := find(i)
		groupMap[root] = append(groupMap[root], i)
	}

	var groups [][]store.CompressedObservationRow
	for _, indices := range groupMap {
		var group []store.CompressedObservationRow
		for _, idx := range indices {
			group = append(group, observations[idx])
		}
		groups = append(groups, group)
	}

	return groups
}
