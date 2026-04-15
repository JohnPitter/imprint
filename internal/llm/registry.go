package llm

import (
	"context"
	"fmt"
	"time"

	"imprint/internal/config"
)

// BuildProviderChain constructs the LLM provider fallback chain from config.
// Providers are added in the order specified by cfg.LLMProviderOrder,
// each wrapped with a circuit breaker for resilience.
func BuildProviderChain(cfg *config.Config) LLMProvider {
	var providers []LLMProvider

	for _, name := range cfg.LLMProviderOrder {
		switch name {
		case "anthropic":
			p := NewAnthropicProvider(cfg.AnthropicAPIKey, cfg.AnthropicBaseURL, cfg.AnthropicModel, cfg.AnthropicAuthMode)
			if p.Available() {
				providers = append(providers, NewResilientProvider(p, NewCircuitBreaker(5, 60*time.Second)))
			}
		case "openrouter":
			p := NewOpenRouterProvider(cfg.OpenRouterAPIKey, cfg.OpenRouterModel)
			if p.Available() {
				providers = append(providers, NewResilientProvider(p, NewCircuitBreaker(5, 60*time.Second)))
			}
		case "llamacpp":
			p := NewLlamaCppProvider(cfg.LlamaCppURL, cfg.LlamaCppModel)
			if p.Available() {
				providers = append(providers, NewResilientProvider(p, NewCircuitBreaker(5, 60*time.Second)))
			}
		}
	}

	if len(providers) == 0 {
		return &NoopProvider{}
	}

	return NewFallbackChain(providers...)
}

// NoopProvider returns an error for all calls. Used when no providers are configured.
type NoopProvider struct{}

func (p *NoopProvider) Name() string      { return "noop" }
func (p *NoopProvider) Available() bool    { return false }
func (p *NoopProvider) Complete(_ context.Context, _ CompletionRequest) (string, error) {
	return "", fmt.Errorf("no LLM provider configured")
}
