package llm

import (
	"encoding/json"
	"fmt"
)

// Shared types and helpers for OpenAI-compatible APIs (OpenRouter, llama.cpp).

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model       string        `json:"model,omitempty"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// parseChatCompletion parses an OpenAI-compatible chat completion response
// and records token usage into the global meter.
func parseChatCompletion(body []byte, provider string) (string, error) {
	var result chatCompletionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		GlobalUsage.Record(provider, 0, 0, true)
		return "", fmt.Errorf("%s: unmarshal response: %w", provider, err)
	}

	if result.Error != nil {
		GlobalUsage.Record(provider, 0, 0, true)
		return "", fmt.Errorf("%s: API error: %s", provider, result.Error.Message)
	}

	if len(result.Choices) == 0 {
		GlobalUsage.Record(provider, 0, 0, true)
		return "", fmt.Errorf("%s: no choices in response", provider)
	}

	pt, ot := 0, 0
	if result.Usage != nil {
		pt = result.Usage.PromptTokens
		ot = result.Usage.CompletionTokens
	}
	GlobalUsage.Record(provider, pt, ot, false)

	return result.Choices[0].Message.Content, nil
}
