package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIProvider_Metadata(t *testing.T) {
	p := NewOpenAIProvider("key", "", "", "minimal")
	if p.Name() != "openai" {
		t.Fatalf("name = %q, want openai", p.Name())
	}
	if !p.Available() {
		t.Fatal("expected available with api key")
	}
	if p.model != "gpt-5-mini" {
		t.Errorf("default model = %q, want gpt-5-mini", p.model)
	}
	if NewOpenAIProvider("", "", "", "").Available() {
		t.Error("expected unavailable with empty key")
	}
}

func TestOpenAIProvider_GPT5RequestShape(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q, want /v1/chat/completions", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer my-key" {
			t.Errorf("missing/wrong Authorization header: %q", r.Header.Get("Authorization"))
		}
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":3,"completion_tokens":2}}`))
	}))
	defer srv.Close()

	p := NewOpenAIProvider("my-key", srv.URL, "gpt-5-mini", "minimal")
	out, err := p.Complete(context.Background(), CompletionRequest{
		SystemPrompt: "sys", UserPrompt: "hi", MaxTokens: 100, Temperature: 0.3,
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if out != "ok" {
		t.Fatalf("output = %q, want ok", out)
	}
	// GPT-5 reasoning models: must use max_completion_tokens, never max_tokens,
	// and must NOT send a custom temperature (only default 1 is accepted).
	if !strings.Contains(gotBody, `"max_completion_tokens":100`) {
		t.Errorf("body missing max_completion_tokens: %s", gotBody)
	}
	if strings.Contains(gotBody, `"max_tokens"`) {
		t.Errorf("body must not contain max_tokens: %s", gotBody)
	}
	if strings.Contains(gotBody, `"temperature"`) {
		t.Errorf("body must not send temperature for GPT-5: %s", gotBody)
	}
	if !strings.Contains(gotBody, `"reasoning_effort":"minimal"`) {
		t.Errorf("body missing reasoning_effort: %s", gotBody)
	}
}

func TestOpenAIProvider_OmitsReasoningWhenEmpty(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	p := NewOpenAIProvider("k", srv.URL, "gpt-5-mini", "") // no reasoning effort
	if _, err := p.Complete(context.Background(), CompletionRequest{UserPrompt: "hi", MaxTokens: 10}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if strings.Contains(gotBody, "reasoning_effort") {
		t.Errorf("reasoning_effort should be omitted when empty: %s", gotBody)
	}
}
