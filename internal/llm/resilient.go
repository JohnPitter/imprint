package llm

import (
	"context"
	"fmt"
)

// ResilientProvider wraps an LLMProvider with a circuit breaker to prevent
// repeated calls to a failing backend.
type ResilientProvider struct {
	inner   LLMProvider
	breaker *CircuitBreaker
}

// NewResilientProvider wraps the given provider with circuit breaker protection.
func NewResilientProvider(inner LLMProvider, breaker *CircuitBreaker) *ResilientProvider {
	return &ResilientProvider{
		inner:   inner,
		breaker: breaker,
	}
}

func (p *ResilientProvider) Name() string    { return p.inner.Name() }
func (p *ResilientProvider) Available() bool { return p.inner.Available() }

func (p *ResilientProvider) Complete(ctx context.Context, req CompletionRequest) (string, error) {
	// Budget ceiling: only instrumented (background) calls are gated. Checked
	// before the breaker so a budget stop is not counted as a provider failure.
	if req.SpendPoint != "" && !GlobalBudget.Allow(req.SessionID) {
		return "", ErrBudgetExceeded
	}

	if !p.breaker.Allow() {
		return "", fmt.Errorf("circuit breaker open for provider %s", p.Name())
	}

	result, err := p.inner.Complete(ctx, req)
	if err != nil {
		p.breaker.RecordFailure()
		return "", err
	}

	p.breaker.RecordSuccess()
	return result, nil
}
