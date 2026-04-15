package llm

import "context"

// CompletionRequest holds the parameters for an LLM completion call.
type CompletionRequest struct {
	SystemPrompt string
	UserPrompt   string
	MaxTokens    int
	Temperature  float64
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
