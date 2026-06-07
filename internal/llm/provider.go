package llm

import (
	"context"
	"sync"
	"sync/atomic"
)

// CompletionRequest holds the parameters for an LLM completion call.
//
// SpendPoint/SessionID/Project are optional economy metadata: when SpendPoint is
// set, the provider reports the call's token usage to SpendSink so the token
// ledger can attribute the Haiku spend to a session/repo (Phase 1). They are
// passed through every wrapper (resilient, fallback) unchanged.
type CompletionRequest struct {
	SystemPrompt string
	UserPrompt   string
	MaxTokens    int
	Temperature  float64

	SpendPoint string
	SessionID  string
	Project    string
}

// SpendEvent is one attributed LLM spend, emitted to SpendSink when a request
// carries a SpendPoint. Tokens are the real counts reported by the provider.
type SpendEvent struct {
	Provider     string
	SpendPoint   string
	SessionID    string
	Project      string
	InputTokens  int
	OutputTokens int
}

// SpendSink, when non-nil, receives one event per instrumented LLM call. main.go
// wires it to the token ledger. Nil by default so the llm package has no store
// dependency and tests stay isolated. Must be cheap and non-blocking.
var SpendSink func(SpendEvent)

// emitSpend reports a successful instrumented call's usage to the budget gate
// (always, so the ceiling stays accurate) and to the ledger sink (if wired).
// Untagged calls — those without a SpendPoint — are ignored here; they are still
// counted in GlobalUsage for the dashboard.
func emitSpend(req CompletionRequest, provider string, promptTokens, outputTokens int) {
	if req.SpendPoint == "" {
		return
	}
	GlobalBudget.Record(req.SessionID, promptTokens+outputTokens)
	if sink := SpendSink; sink != nil {
		sink(SpendEvent{
			Provider:     provider,
			SpendPoint:   req.SpendPoint,
			SessionID:    req.SessionID,
			Project:      req.Project,
			InputTokens:  promptTokens,
			OutputTokens: outputTokens,
		})
	}
}

// LLMProvider is the interface that all LLM backends must implement.
type LLMProvider interface {
	// Name returns the provider identifier (e.g. "anthropic", "openrouter").
	Name() string
	// Complete sends a completion request and returns the generated text.
	Complete(ctx context.Context, req CompletionRequest) (string, error)
	// Available returns true if the provider is configured (has API key, URL, etc.).
	Available() bool
}

// UsageMeter aggregates token counts across LLM calls. Providers report into
// the global meter via Record; consumers (the dashboard pipeline panel) read
// the snapshot via Snapshot. Concurrent-safe.
//
// We record at the provider layer rather than the call site because each
// provider knows the response shape — Anthropic returns input/output_tokens,
// OpenAI-compat returns prompt/completion_tokens, llama.cpp the same. The
// caller doesn't need to care.
type UsageMeter struct {
	mu       sync.Mutex
	calls    atomic.Int64
	failures atomic.Int64
	prompt   atomic.Int64
	output   atomic.Int64
	byProv   map[string]*providerUsage
}

type providerUsage struct {
	Calls    int64 `json:"calls"`
	Failures int64 `json:"failures"`
	Prompt   int64 `json:"promptTokens"`
	Output   int64 `json:"outputTokens"`
}

// UsageSnapshot is the read-only view returned by Snapshot.
type UsageSnapshot struct {
	Calls        int64                     `json:"calls"`
	Failures     int64                     `json:"failures"`
	PromptTokens int64                     `json:"promptTokens"`
	OutputTokens int64                     `json:"outputTokens"`
	ByProvider   map[string]*providerUsage `json:"byProvider"`
}

// GlobalUsage is the meter every provider records into. main.go wires it via
// each provider's NewX so the meter outlives reloads.
var GlobalUsage = &UsageMeter{byProv: map[string]*providerUsage{}}

// Record adds one call's stats to the meter. promptTokens or outputTokens
// may be 0 when the provider doesn't report them (some local backends).
func (m *UsageMeter) Record(provider string, promptTokens, outputTokens int, failed bool) {
	m.calls.Add(1)
	m.prompt.Add(int64(promptTokens))
	m.output.Add(int64(outputTokens))
	if failed {
		m.failures.Add(1)
	}
	m.mu.Lock()
	pu, ok := m.byProv[provider]
	if !ok {
		pu = &providerUsage{}
		m.byProv[provider] = pu
	}
	pu.Calls++
	pu.Prompt += int64(promptTokens)
	pu.Output += int64(outputTokens)
	if failed {
		pu.Failures++
	}
	m.mu.Unlock()
}

// Snapshot returns a deep copy of the current counters. Safe to serialise.
func (m *UsageMeter) Snapshot() UsageSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	by := make(map[string]*providerUsage, len(m.byProv))
	for k, v := range m.byProv {
		copy := *v
		by[k] = &copy
	}
	return UsageSnapshot{
		Calls:        m.calls.Load(),
		Failures:     m.failures.Load(),
		PromptTokens: m.prompt.Load(),
		OutputTokens: m.output.Load(),
		ByProvider:   by,
	}
}
