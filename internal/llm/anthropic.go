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

// AnthropicProvider implements LLMProvider using the Anthropic Messages API.
// Supports both API key and OAuth authentication modes.
type AnthropicProvider struct {
	apiKey   string
	baseURL  string
	model    string
	authMode string // "api_key" or "oauth"
	client   *http.Client
}

// NewAnthropicProvider creates a provider for the Anthropic Messages API.
func NewAnthropicProvider(apiKey, baseURL, model, authMode string) *AnthropicProvider {
	return &AnthropicProvider{
		apiKey:   apiKey,
		baseURL:  baseURL,
		model:    model,
		authMode: authMode,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (p *AnthropicProvider) Name() string    { return "anthropic" }
func (p *AnthropicProvider) Available() bool { return p.apiKey != "" }

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (p *AnthropicProvider) Complete(ctx context.Context, req CompletionRequest) (string, error) {
	body := anthropicRequest{
		Model:     p.model,
		MaxTokens: req.MaxTokens,
		System:    req.SystemPrompt,
		Messages: []anthropicMessage{
			{Role: "user", Content: req.UserPrompt},
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("anthropic: create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	// Both API key and OAuth token use x-api-key header
	httpReq.Header.Set("x-api-key", p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("anthropic: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("anthropic: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result anthropicResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("anthropic: unmarshal response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("anthropic: API error: %s: %s", result.Error.Type, result.Error.Message)
	}

	for _, block := range result.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}

	return "", fmt.Errorf("anthropic: no text content in response")
}
