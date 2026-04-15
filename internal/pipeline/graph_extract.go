package pipeline

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"imprint/internal/llm"
	"imprint/internal/store"
)

const graphExtractSystemPrompt = `You are a knowledge graph extractor. Given a compressed observation, extract entities (nodes) and relationships (edges).

Respond with XML:
<graph>
  <nodes>
    <node>
      <type>file|function|concept|error|decision|pattern|library|person|project</type>
      <name>entity name</name>
    </node>
  </nodes>
  <edges>
    <edge>
      <source>source entity name</source>
      <target>target entity name</target>
      <type>uses|imports|modifies|causes|fixes|depends_on|related_to</type>
      <weight>0.0-1.0</weight>
    </edge>
  </edges>
</graph>`

const graphExtractUserPrompt = `Extract entities and relationships from this observation:
Type: %s
Title: %s
Narrative: %s
Files: %s
Concepts: %s`

// GraphExtractor uses an LLM to extract entities and relationships from compressed observations.
type GraphExtractor struct {
	provider llm.LLMProvider
}

// NewGraphExtractor creates a new GraphExtractor with the given LLM provider.
func NewGraphExtractor(provider llm.LLMProvider) *GraphExtractor {
	return &GraphExtractor{provider: provider}
}

// ExtractedNode represents a single entity extracted from an observation.
type ExtractedNode struct {
	Type string
	Name string
}

// ExtractedEdge represents a relationship between two entities.
type ExtractedEdge struct {
	Source string
	Target string
	Type   string
	Weight float64
}

// ExtractionResult holds the nodes and edges extracted from a single observation.
type ExtractionResult struct {
	Nodes []ExtractedNode
	Edges []ExtractedEdge
}

// Extract processes a compressed observation and returns graph entities.
func (g *GraphExtractor) Extract(ctx context.Context, obs *store.CompressedObservationRow) (*ExtractionResult, error) {
	narrative := ""
	if obs.Narrative != nil {
		narrative = *obs.Narrative
	}
	files := strings.Join(obs.Files, ", ")
	concepts := strings.Join(obs.Concepts, ", ")

	userPrompt := fmt.Sprintf(graphExtractUserPrompt, obs.Type, obs.Title, narrative, files, concepts)

	resp, err := g.provider.Complete(ctx, llm.CompletionRequest{
		SystemPrompt: graphExtractSystemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    1500,
		Temperature:  0.2,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM graph extract: %w", err)
	}

	// Parse nodes
	var nodes []ExtractedNode
	nodeContent := getXMLTag(resp, "nodes")
	if nodeContent != "" {
		nodePattern := regexp.MustCompile(`(?s)<node>(.*?)</node>`)
		nodeMatches := nodePattern.FindAllStringSubmatch(nodeContent, -1)
		for _, m := range nodeMatches {
			if len(m) < 2 {
				continue
			}
			nType := getXMLTag(m[1], "type")
			nName := getXMLTag(m[1], "name")
			if nType != "" && nName != "" {
				nodes = append(nodes, ExtractedNode{Type: nType, Name: nName})
			}
		}
	}

	// Parse edges
	var edges []ExtractedEdge
	edgeContent := getXMLTag(resp, "edges")
	if edgeContent != "" {
		edgePattern := regexp.MustCompile(`(?s)<edge>(.*?)</edge>`)
		edgeMatches := edgePattern.FindAllStringSubmatch(edgeContent, -1)
		for _, m := range edgeMatches {
			if len(m) < 2 {
				continue
			}
			source := getXMLTag(m[1], "source")
			target := getXMLTag(m[1], "target")
			eType := getXMLTag(m[1], "type")
			weight := 0.5
			if w := getXMLTag(m[1], "weight"); w != "" {
				if parsed, parseErr := strconv.ParseFloat(w, 64); parseErr == nil && parsed > 0 && parsed <= 1 {
					weight = parsed
				}
			}
			if source != "" && target != "" && eType != "" {
				edges = append(edges, ExtractedEdge{Source: source, Target: target, Type: eType, Weight: weight})
			}
		}
	}

	return &ExtractionResult{Nodes: nodes, Edges: edges}, nil
}
