package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"imprint/internal/llm"
	"imprint/internal/store"
)

// Rooter turns a convergent cluster of refined memories into an intuition (a
// "how to reason" premise) and judges whether a new memory contradicts an
// existing intuition. Both are deliberately LLM-backed — rooting is expensive by
// design (3.5) — and both are instrumented as the "root" spend point so the cost
// shows in the token meter and is gated by the budget ceiling.
type Rooter struct {
	provider llm.LLMProvider
}

func NewRooter(provider llm.LLMProvider) *Rooter { return &Rooter{provider: provider} }

// RootedDraft is a synthesized intuition candidate.
type RootedDraft struct {
	Statement string
	Concepts  []string
	Files     []string
}

const rootSystemPrompt = `You distill a TRANSVERSAL reasoning premise ("intuition") from a cluster of distilled memories. An intuition is NOT a fact about a file — it is a rule about HOW TO REASON in this context (e.g. "here, simplicity beats sophisticated patterns"; "err toward less abstraction").

Only emit an intuition if the memories genuinely CONVERGE on one reasoning premise across different episodes. If they are merely related facts that do not converge into a how-to-reason rule, emit NONE.

Respond with XML:
<intuition>
  <converges>yes|no</converges>
  <statement>one sentence: the reasoning premise (empty if no)</statement>
  <concepts><concept>concept</concept></concepts>
</intuition>`

const rootUserPrompt = `These %d distilled memories recurred across distinct sessions. Do they converge into a single how-to-reason premise?

%s`

// Synthesize asks the LLM to extract a reasoning premise from the cluster.
// Returns nil (no draft) when the cluster does not converge — the high bar.
func (r *Rooter) Synthesize(ctx context.Context, project, sessionID string, cluster []store.MemoryRow) (*RootedDraft, error) {
	if len(cluster) == 0 {
		return nil, nil
	}
	var sb strings.Builder
	concepts := map[string]struct{}{}
	files := map[string]struct{}{}
	for _, m := range cluster {
		fmt.Fprintf(&sb, "- [%s] %s: %s\n", m.Type, m.Title, truncate(m.Content, 200))
		for _, c := range jsonStrings(m.Concepts) {
			concepts[c] = struct{}{}
		}
		for _, f := range jsonStrings(m.Files) {
			files[f] = struct{}{}
		}
	}

	resp, err := r.provider.Complete(ctx, llm.CompletionRequest{
		SystemPrompt: rootSystemPrompt,
		UserPrompt:   fmt.Sprintf(rootUserPrompt, len(cluster), sb.String()),
		MaxTokens:    600,
		Temperature:  0.2,
		SpendPoint:   "root",
		SessionID:    sessionID,
		Project:      project,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM root synthesize: %w", err)
	}

	converges := strings.ToLower(strings.TrimSpace(getXMLTag(resp, "converges")))
	statement := strings.TrimSpace(getXMLTag(resp, "statement"))
	if converges != "yes" || statement == "" {
		return nil, nil // did not clear the convergence bar
	}
	return &RootedDraft{
		Statement: statement,
		Concepts:  getXMLChildren(resp, "concepts", "concept"),
		Files:     setToSlice(files),
	}, nil
}

const contradictSystemPrompt = `You judge whether a new distilled memory CONTRADICTS a standing reasoning premise (intuition). Contradiction means the memory shows the premise is wrong or does not hold here — not merely that it adds a specific exception (a documented exception is NOT a contradiction of the general default).

Respond with XML:
<judgement>
  <contradicts>yes|no</contradicts>
  <detail>short reason</detail>
</judgement>`

const contradictUserPrompt = `Intuition (general premise): %s

New memory: [%s] %s: %s

Does the new memory contradict the premise?`

// Contradicts asks the LLM whether a memory contradicts an intuition. Used by
// the auto-weakening loop; a documented specific exception is not a contradiction.
func (r *Rooter) Contradicts(ctx context.Context, project, statement string, mem store.MemoryRow) (bool, string, error) {
	resp, err := r.provider.Complete(ctx, llm.CompletionRequest{
		SystemPrompt: contradictSystemPrompt,
		UserPrompt:   fmt.Sprintf(contradictUserPrompt, statement, mem.Type, mem.Title, truncate(mem.Content, 200)),
		MaxTokens:    300,
		Temperature:  0.1,
		SpendPoint:   "root",
		Project:      project,
	})
	if err != nil {
		return false, "", fmt.Errorf("LLM contradiction check: %w", err)
	}
	yes := strings.EqualFold(strings.TrimSpace(getXMLTag(resp, "contradicts")), "yes")
	return yes, strings.TrimSpace(getXMLTag(resp, "detail")), nil
}

// jsonStrings decodes a json.RawMessage array field into a slice (tolerant).
func jsonStrings(raw []byte) []string {
	var out []string
	if len(raw) == 0 {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

func setToSlice(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		if k != "" {
			out = append(out, k)
		}
	}
	return out
}
