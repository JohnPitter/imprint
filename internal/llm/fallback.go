package llm

import (
	"context"
	"fmt"
	"log"
)

// FallbackChainProvider tries multiple LLM providers in order, falling back
// to the next one if the current provider fails.
type FallbackChainProvider struct {
	providers []LLMProvider
}

// NewFallbackChain creates a provider that tries each provider in order.
func NewFallbackChain(providers ...LLMProvider) *FallbackChainProvider {
	return &FallbackChainProvider{providers: providers}
}

func (f *FallbackChainProvider) Name() string { return "fallback-chain" }

func (f *FallbackChainProvider) Available() bool {
	for _, p := range f.providers {
		if p.Available() {
			return true
		}
	}
	return false
}

func (f *FallbackChainProvider) Complete(ctx context.Context, req CompletionRequest) (string, error) {
	var lastErr error
	for _, p := range f.providers {
		if !p.Available() {
			continue
		}
		result, err := p.Complete(ctx, req)
		if err == nil {
			return result, nil
		}
		lastErr = err
		log.Printf("[llm] Provider %s failed: %v, trying next...", p.Name(), err)
	}

	if lastErr != nil {
		return "", fmt.Errorf("all providers failed, last: %w", lastErr)
	}
	return "", fmt.Errorf("no available LLM providers")
}
