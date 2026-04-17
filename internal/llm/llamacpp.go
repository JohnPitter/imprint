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

// LlamaCppProvider implements LLMProvider using a local llama.cpp server
// that exposes an OpenAI-compatible chat completions endpoint.
type LlamaCppProvider struct {
	baseURL string
	model   string // optional; server uses its default if empty
	client  *http.Client
}

// NewLlamaCppProvider creates a provider for a llama.cpp server.
func NewLlamaCppProvider(baseURL, model string) *LlamaCppProvider {
	return &LlamaCppProvider{
		baseURL: baseURL,
		model:   model,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (p *LlamaCppProvider) Name() string    { return "llamacpp" }
func (p *LlamaCppProvider) Available() bool { return p.baseURL != "" }

func (p *LlamaCppProvider) Complete(ctx context.Context, req CompletionRequest) (string, error) {
	messages := make([]chatMessage, 0, 2)
	if req.SystemPrompt != "" {
		messages = append(messages, chatMessage{Role: "system", Content: req.SystemPrompt})
	}
	messages = append(messages, chatMessage{Role: "user", Content: req.UserPrompt})

	body := chatCompletionRequest{
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}
	if p.model != "" {
		body.Model = p.model
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("llamacpp: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("llamacpp: create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("llamacpp: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("llamacpp: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llamacpp: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return parseChatCompletion(respBody, "llamacpp")
}
