package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenRouterProvider implements LLMProvider using the OpenRouter API
// (OpenAI-compatible chat completions).
type OpenRouterProvider struct {
	apiKey string
	model  string
	client *http.Client
}

// NewOpenRouterProvider creates a provider for the OpenRouter API.
func NewOpenRouterProvider(apiKey, model string) *OpenRouterProvider {
	return &OpenRouterProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (p *OpenRouterProvider) Name() string    { return "openrouter" }
func (p *OpenRouterProvider) Available() bool  { return p.apiKey != "" }

func (p *OpenRouterProvider) Complete(ctx context.Context, req CompletionRequest) (string, error) {
	messages := make([]chatMessage, 0, 2)
	if req.SystemPrompt != "" {
		messages = append(messages, chatMessage{Role: "system", Content: req.SystemPrompt})
	}
	messages = append(messages, chatMessage{Role: "user", Content: req.UserPrompt})

	body := chatCompletionRequest{
		Model:       p.model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("openrouter: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("openrouter: create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("openrouter: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("openrouter: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openrouter: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return parseChatCompletion(respBody, "openrouter")
}
