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
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// parseChatCompletion parses an OpenAI-compatible chat completion response.
func parseChatCompletion(body []byte, provider string) (string, error) {
	var result chatCompletionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("%s: unmarshal response: %w", provider, err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("%s: API error: %s", provider, result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("%s: no choices in response", provider)
	}

	return result.Choices[0].Message.Content, nil
}
