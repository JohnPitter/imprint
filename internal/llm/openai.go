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

// OpenAIProvider targets the OpenAI Chat Completions API (api.openai.com),
// defaulting to a cheap GPT-5 tier (gpt-5-mini) as the Codex-side equivalent of
// Haiku for background work.
//
// GPT-5 reasoning models differ from classic chat models on this endpoint:
//   - they reject a non-default "temperature" (only 1 is allowed), so we omit it
//   - they require "max_completion_tokens" instead of the deprecated "max_tokens"
//   - they accept an optional "reasoning_effort" (minimal|low|medium|high);
//     "minimal" keeps background compression fast and cheap.
//
// The response shape is identical to other OpenAI-compatible backends, so we
// reuse parseChatCompletion (which records token usage + the spend ledger).
type OpenAIProvider struct {
	apiKey          string
	baseURL         string
	model           string
	reasoningEffort string
	client          *http.Client
}

// NewOpenAIProvider creates an OpenAI provider. baseURL defaults to the public
// API; reasoningEffort may be "" to let the model use its own default.
func NewOpenAIProvider(apiKey, baseURL, model, reasoningEffort string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	if model == "" {
		model = "gpt-5-mini"
	}
	return &OpenAIProvider{
		apiKey:          apiKey,
		baseURL:         baseURL,
		model:           model,
		reasoningEffort: reasoningEffort,
		client:          &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *OpenAIProvider) Name() string    { return "openai" }
func (p *OpenAIProvider) Available() bool { return p.apiKey != "" }

type openaiRequest struct {
	Model               string        `json:"model"`
	Messages            []chatMessage `json:"messages"`
	MaxCompletionTokens int           `json:"max_completion_tokens,omitempty"`
	ReasoningEffort     string        `json:"reasoning_effort,omitempty"`
}

func (p *OpenAIProvider) Complete(ctx context.Context, req CompletionRequest) (string, error) {
	messages := make([]chatMessage, 0, 2)
	if req.SystemPrompt != "" {
		messages = append(messages, chatMessage{Role: "system", Content: req.SystemPrompt})
	}
	messages = append(messages, chatMessage{Role: "user", Content: req.UserPrompt})

	body := openaiRequest{
		Model:               p.model,
		Messages:            messages,
		MaxCompletionTokens: req.MaxTokens, // GPT-5 uses max_completion_tokens; temperature is intentionally omitted
		ReasoningEffort:     p.reasoningEffort,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("openai: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("openai: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("openai: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("openai: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return parseChatCompletion(respBody, "openai", req)
}
