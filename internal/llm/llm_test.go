package llm

import (
	"context"
	"errors"
	"testing"
	"time"

	"imprint/internal/config"
)

// mockProvider is a configurable fake LLM provider for testing.
type mockProvider struct {
	name      string
	available bool
	response  string
	err       error
	calls     int
}

func (m *mockProvider) Name() string      { return m.name }
func (m *mockProvider) Available() bool    { return m.available }
func (m *mockProvider) Complete(_ context.Context, _ CompletionRequest) (string, error) {
	m.calls++
	return m.response, m.err
}

// ---------------------------------------------------------------------------
// CircuitBreaker tests
// ---------------------------------------------------------------------------

func TestCircuitBreaker_ClosedAllows(t *testing.T) {
	cb := NewCircuitBreaker(3, 10*time.Second)

	if !cb.Allow() {
		t.Fatal("expected closed circuit to allow requests")
	}
	if cb.State() != CircuitClosed {
		t.Fatalf("expected state CircuitClosed, got %d", cb.State())
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, 10*time.Second)

	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	if cb.State() != CircuitOpen {
		t.Fatalf("expected state CircuitOpen after %d failures, got %d", 3, cb.State())
	}
	if cb.Allow() {
		t.Fatal("expected open circuit to block requests")
	}
}

func TestCircuitBreaker_ResetsAfterTimeout(t *testing.T) {
	cb := NewCircuitBreaker(1, 1*time.Millisecond)

	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Fatal("expected circuit to open after 1 failure")
	}

	// Wait for the reset timeout to elapse.
	time.Sleep(5 * time.Millisecond)

	if !cb.Allow() {
		t.Fatal("expected circuit to transition to half-open and allow a probe request")
	}
	if cb.State() != CircuitHalfOpen {
		t.Fatalf("expected state CircuitHalfOpen, got %d", cb.State())
	}
}

func TestCircuitBreaker_ClosesOnSuccess(t *testing.T) {
	cb := NewCircuitBreaker(1, 1*time.Millisecond)

	cb.RecordFailure()
	time.Sleep(5 * time.Millisecond)

	// Allow transitions to half-open.
	cb.Allow()
	if cb.State() != CircuitHalfOpen {
		t.Fatal("expected half-open state before success")
	}

	cb.RecordSuccess()

	if cb.State() != CircuitClosed {
		t.Fatalf("expected circuit to close after success, got %d", cb.State())
	}
	if !cb.Allow() {
		t.Fatal("expected closed circuit to allow requests")
	}
}

// ---------------------------------------------------------------------------
// ResilientProvider tests
// ---------------------------------------------------------------------------

func TestResilientProvider_PassesThrough(t *testing.T) {
	mock := &mockProvider{name: "test", available: true, response: "hello"}
	cb := NewCircuitBreaker(5, 10*time.Second)
	rp := NewResilientProvider(mock, cb)

	result, err := rp.Complete(context.Background(), CompletionRequest{UserPrompt: "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello" {
		t.Fatalf("expected 'hello', got %q", result)
	}
	if mock.calls != 1 {
		t.Fatalf("expected 1 call, got %d", mock.calls)
	}
	if rp.Name() != "test" {
		t.Fatalf("expected name 'test', got %q", rp.Name())
	}
}

func TestResilientProvider_TripsBreaker(t *testing.T) {
	mock := &mockProvider{name: "fail", available: true, err: errors.New("server error")}
	cb := NewCircuitBreaker(2, 10*time.Second)
	rp := NewResilientProvider(mock, cb)

	// Two failures should trip the breaker.
	for i := 0; i < 2; i++ {
		_, err := rp.Complete(context.Background(), CompletionRequest{})
		if err == nil {
			t.Fatal("expected error from failing provider")
		}
	}

	if cb.State() != CircuitOpen {
		t.Fatalf("expected circuit to be open after 2 failures, got %d", cb.State())
	}

	// Next call should be blocked by the circuit breaker, not reaching the mock.
	_, err := rp.Complete(context.Background(), CompletionRequest{})
	if err == nil {
		t.Fatal("expected circuit breaker open error")
	}
	if mock.calls != 2 {
		t.Fatalf("expected only 2 calls to inner provider, got %d", mock.calls)
	}
}

// ---------------------------------------------------------------------------
// FallbackChainProvider tests
// ---------------------------------------------------------------------------

func TestFallbackChain_UsesFirst(t *testing.T) {
	first := &mockProvider{name: "first", available: true, response: "from-first"}
	second := &mockProvider{name: "second", available: true, response: "from-second"}
	chain := NewFallbackChain(first, second)

	result, err := chain.Complete(context.Background(), CompletionRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "from-first" {
		t.Fatalf("expected 'from-first', got %q", result)
	}
	if first.calls != 1 {
		t.Fatalf("expected 1 call to first, got %d", first.calls)
	}
	if second.calls != 0 {
		t.Fatalf("expected 0 calls to second, got %d", second.calls)
	}
}

func TestFallbackChain_FallsBack(t *testing.T) {
	first := &mockProvider{name: "first", available: true, err: errors.New("down")}
	second := &mockProvider{name: "second", available: true, response: "from-second"}
	chain := NewFallbackChain(first, second)

	result, err := chain.Complete(context.Background(), CompletionRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "from-second" {
		t.Fatalf("expected 'from-second', got %q", result)
	}
	if first.calls != 1 {
		t.Fatalf("expected 1 call to first, got %d", first.calls)
	}
	if second.calls != 1 {
		t.Fatalf("expected 1 call to second, got %d", second.calls)
	}
}

func TestFallbackChain_SkipsUnavailable(t *testing.T) {
	first := &mockProvider{name: "first", available: false, response: "from-first"}
	second := &mockProvider{name: "second", available: true, response: "from-second"}
	chain := NewFallbackChain(first, second)

	result, err := chain.Complete(context.Background(), CompletionRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "from-second" {
		t.Fatalf("expected 'from-second', got %q", result)
	}
	if first.calls != 0 {
		t.Fatalf("expected 0 calls to unavailable provider, got %d", first.calls)
	}
}

func TestFallbackChain_AllFail(t *testing.T) {
	first := &mockProvider{name: "first", available: true, err: errors.New("err1")}
	second := &mockProvider{name: "second", available: true, err: errors.New("err2")}
	chain := NewFallbackChain(first, second)

	_, err := chain.Complete(context.Background(), CompletionRequest{})
	if err == nil {
		t.Fatal("expected error when all providers fail")
	}
}

// ---------------------------------------------------------------------------
// BuildProviderChain tests
// ---------------------------------------------------------------------------

func TestBuildProviderChain_NoConfig(t *testing.T) {
	cfg := &config.Config{
		LLMProviderOrder: []string{"anthropic", "openrouter", "llamacpp"},
		AnthropicAPIKey:  "",
		OpenRouterAPIKey: "",
		LlamaCppURL:      "", // empty URL means unavailable
	}

	provider := BuildProviderChain(cfg)

	if provider.Name() != "noop" {
		t.Fatalf("expected noop provider, got %q", provider.Name())
	}
	if provider.Available() {
		t.Fatal("expected noop provider to be unavailable")
	}
}

func TestBuildProviderChain_WithAPIKey(t *testing.T) {
	cfg := &config.Config{
		LLMProviderOrder: []string{"anthropic"},
		AnthropicAPIKey:  "sk-test-key-12345",
		AnthropicBaseURL: "https://api.anthropic.com",
		AnthropicModel:   "claude-haiku-4-5-20251001",
		AnthropicAuthMode: "api_key",
	}

	provider := BuildProviderChain(cfg)

	if provider.Name() == "noop" {
		t.Fatal("expected a real provider chain, got noop")
	}
	if !provider.Available() {
		t.Fatal("expected provider chain to be available")
	}
}

// ---------------------------------------------------------------------------
// NoopProvider tests
// ---------------------------------------------------------------------------

func TestNoopProvider(t *testing.T) {
	noop := &NoopProvider{}

	if noop.Name() != "noop" {
		t.Fatalf("expected name 'noop', got %q", noop.Name())
	}
	if noop.Available() {
		t.Fatal("expected noop to be unavailable")
	}

	_, err := noop.Complete(context.Background(), CompletionRequest{})
	if err == nil {
		t.Fatal("expected error from noop provider")
	}
}
