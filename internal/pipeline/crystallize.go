package pipeline

import (
	"context"
	"fmt"
	"strings"

	"imprint/internal/llm"
	"imprint/internal/store"
)

const crystallizeSystemPrompt = `You are a narrative crystallizer. Given a set of completed actions from a development session, create a cohesive narrative digest that captures what was accomplished, the key outcomes, files affected, and lessons learned.

Respond with XML:
<crystal>
  <narrative>A cohesive 3-5 sentence narrative of what was accomplished and why</narrative>
  <key_outcomes>
    <outcome>Key outcome 1</outcome>
    <outcome>Key outcome 2</outcome>
  </key_outcomes>
  <files_affected>
    <file>path/to/file</file>
  </files_affected>
  <lessons>
    <lesson>Lesson learned from this work</lesson>
  </lessons>
</crystal>`

const crystallizeUserPrompt = `Crystallize these %d completed actions into a narrative digest:

%s`

// CrystalResult represents the output of the crystallization pipeline.
type CrystalResult struct {
	Narrative   string   `json:"narrative"`
	KeyOutcomes []string `json:"keyOutcomes"`
	Files       []string `json:"filesAffected"`
	Lessons     []string `json:"lessons"`
}

// Crystallizer creates narrative digests from completed action chains via LLM.
type Crystallizer struct {
	provider llm.LLMProvider
}

// NewCrystallizer creates a new Crystallizer with the given LLM provider.
func NewCrystallizer(provider llm.LLMProvider) *Crystallizer {
	return &Crystallizer{provider: provider}
}

// Crystallize generates a narrative digest from completed actions.
func (c *Crystallizer) Crystallize(ctx context.Context, actions []store.ActionRow) (*CrystalResult, error) {
	if len(actions) == 0 {
		return nil, fmt.Errorf("no actions to crystallize")
	}

	// Build action list for the prompt.
	var sb strings.Builder
	for _, a := range actions {
		status := a.Status
		desc := truncate(a.Description, 200)
		fmt.Fprintf(&sb, "- [%s] %s: %s\n", status, a.Title, desc)
	}

	userPrompt := fmt.Sprintf(crystallizeUserPrompt, len(actions), sb.String())

	resp, err := c.provider.Complete(ctx, llm.CompletionRequest{
		SystemPrompt: crystallizeSystemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    1500,
		Temperature:  0.3,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM crystallize: %w", err)
	}

	return parseCrystalResult(resp), nil
}

// parseCrystalResult extracts the crystal result from the LLM XML response.
func parseCrystalResult(resp string) *CrystalResult {
	narrative := getXMLTag(resp, "narrative")
	if narrative == "" {
		narrative = "No narrative generated."
	}

	keyOutcomes := getXMLChildren(resp, "key_outcomes", "outcome")
	files := getXMLChildren(resp, "files_affected", "file")
	lessons := getXMLChildren(resp, "lessons", "lesson")

	return &CrystalResult{
		Narrative:   narrative,
		KeyOutcomes: keyOutcomes,
		Files:       files,
		Lessons:     lessons,
	}
}
