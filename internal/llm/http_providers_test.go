package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// AnthropicProvider tests
// ---------------------------------------------------------------------------

func TestAnthropicProvider_Metadata(t *testing.T) {
	p := NewAnthropicProvider("key", "https://example.com", "claude", "api_key")
	if p.Name() != "anthropic" {
		t.Fatalf("expected name 'anthropic', got %q", p.Name())
	}
	if !p.Available() {
		t.Fatal("expected available when api key is set")
	}

	empty := NewAnthropicProvider("", "https://example.com", "claude", "api_key")
	if empty.Available() {
		t.Fatal("expected unavailable with empty api key")
	}
}

func TestAnthropicProvider_CompleteSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "my-key" {
			t.Errorf("expected x-api-key header")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Errorf("expected anthropic-version header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"hello from claude"}]}`))
	}))
	defer srv.Close()

	p := NewAnthropicProvider("my-key", srv.URL, "claude-haiku", "api_key")
	out, err := p.Complete(context.Background(), CompletionRequest{
		SystemPrompt: "be nice",
		UserPrompt:   "hi",
		MaxTokens:    64,
		Temperature:  0.5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "hello from claude" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestAnthropicProvider_CompleteHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("oops"))
	}))
	defer srv.Close()

	p := NewAnthropicProvider("my-key", srv.URL, "claude-haiku", "api_key")
	_, err := p.Complete(context.Background(), CompletionRequest{UserPrompt: "hi"})
	if err == nil {
		t.Fatal("expected error on HTTP 500")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("expected error to mention HTTP 500, got %v", err)
	}
}

func TestAnthropicProvider_CompleteAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[],"error":{"type":"invalid_request","message":"bad model"}}`))
	}))
	defer srv.Close()

	p := NewAnthropicProvider("my-key", srv.URL, "claude-haiku", "api_key")
	_, err := p.Complete(context.Background(), CompletionRequest{UserPrompt: "hi"})
	if err == nil {
		t.Fatal("expected API error")
	}
	if !strings.Contains(err.Error(), "bad model") {
		t.Fatalf("expected API error message, got %v", err)
	}
}

func TestAnthropicProvider_CompleteMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not json`))
	}))
	defer srv.Close()

	p := NewAnthropicProvider("my-key", srv.URL, "claude-haiku", "api_key")
	_, err := p.Complete(context.Background(), CompletionRequest{UserPrompt: "hi"})
	if err == nil {
		t.Fatal("expected error on malformed JSON")
	}
}

func TestAnthropicProvider_CompleteNoTextContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"image","text":""}]}`))
	}))
	defer srv.Close()

	p := NewAnthropicProvider("my-key", srv.URL, "claude-haiku", "api_key")
	_, err := p.Complete(context.Background(), CompletionRequest{UserPrompt: "hi"})
	if err == nil {
		t.Fatal("expected error when no text content")
	}
	if !strings.Contains(err.Error(), "no text content") {
		t.Fatalf("expected 'no text content' error, got %v", err)
	}
}

func TestAnthropicProvider_CompleteRequestFailed(t *testing.T) {
	// Use a URL that cannot be reached.
	p := NewAnthropicProvider("my-key", "http://127.0.0.1:1", "claude-haiku", "api_key")
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err := p.Complete(ctx, CompletionRequest{UserPrompt: "hi"})
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestAnthropicProvider_CompleteBadURL(t *testing.T) {
	// A URL containing a control character should fail at http.NewRequestWithContext.
	p := NewAnthropicProvider("my-key", "http://bad\x7furl", "claude", "api_key")
	_, err := p.Complete(context.Background(), CompletionRequest{UserPrompt: "hi"})
	if err == nil {
		t.Fatal("expected error constructing request with bad URL")
	}
}

// ---------------------------------------------------------------------------
// LlamaCppProvider tests
// ---------------------------------------------------------------------------

func TestLlamaCppProvider_Metadata(t *testing.T) {
	p := NewLlamaCppProvider("http://localhost:8080", "model")
	if p.Name() != "llamacpp" {
		t.Fatalf("expected 'llamacpp', got %q", p.Name())
	}
	if !p.Available() {
		t.Fatal("expected available with baseURL set")
	}
	empty := NewLlamaCppProvider("", "")
	if empty.Available() {
		t.Fatal("expected unavailable with empty URL")
	}
}

func TestLlamaCppProvider_CompleteSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hi from llama"}}]}`))
	}))
	defer srv.Close()

	p := NewLlamaCppProvider(srv.URL, "my-model")
	out, err := p.Complete(context.Background(), CompletionRequest{
		SystemPrompt: "system",
		UserPrompt:   "user",
		MaxTokens:    32,
		Temperature:  0.2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "hi from llama" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestLlamaCppProvider_CompleteNoSystemPrompt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	p := NewLlamaCppProvider(srv.URL, "")
	out, err := p.Complete(context.Background(), CompletionRequest{UserPrompt: "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "ok" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestLlamaCppProvider_CompleteHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("bad gateway"))
	}))
	defer srv.Close()

	p := NewLlamaCppProvider(srv.URL, "")
	_, err := p.Complete(context.Background(), CompletionRequest{UserPrompt: "hi"})
	if err == nil {
		t.Fatal("expected HTTP error")
	}
	if !strings.Contains(err.Error(), "HTTP 502") {
		t.Fatalf("expected 'HTTP 502' in error, got %v", err)
	}
}

func TestLlamaCppProvider_CompleteMalformed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	p := NewLlamaCppProvider(srv.URL, "")
	_, err := p.Complete(context.Background(), CompletionRequest{UserPrompt: "hi"})
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestLlamaCppProvider_CompleteRequestFailed(t *testing.T) {
	p := NewLlamaCppProvider("http://127.0.0.1:1", "")
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err := p.Complete(ctx, CompletionRequest{UserPrompt: "hi"})
	if err == nil {
		t.Fatal("expected connection failure")
	}
}

func TestLlamaCppProvider_CompleteBadURL(t *testing.T) {
	p := NewLlamaCppProvider("http://bad\x7furl", "")
	_, err := p.Complete(context.Background(), CompletionRequest{UserPrompt: "hi"})
	if err == nil {
		t.Fatal("expected error with malformed URL")
	}
}

// ---------------------------------------------------------------------------
// OpenRouterProvider tests (URL is hardcoded; we exercise metadata + error paths)
// ---------------------------------------------------------------------------

func TestOpenRouterProvider_Metadata(t *testing.T) {
	p := NewOpenRouterProvider("k", "m")
	if p.Name() != "openrouter" {
		t.Fatalf("expected 'openrouter', got %q", p.Name())
	}
	if !p.Available() {
		t.Fatal("expected available with api key")
	}
	empty := NewOpenRouterProvider("", "m")
	if empty.Available() {
		t.Fatal("expected unavailable with empty key")
	}
}

func TestOpenRouterProvider_CompleteContextCanceled(t *testing.T) {
	p := NewOpenRouterProvider("k", "m")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before call

	_, err := p.Complete(ctx, CompletionRequest{
		SystemPrompt: "sys",
		UserPrompt:   "user",
		MaxTokens:    8,
		Temperature:  0.1,
	})
	if err == nil {
		t.Fatal("expected error when context is already canceled")
	}
}

func TestOpenRouterProvider_CompleteNoSystemPrompt(t *testing.T) {
	// Cancel context so we exercise marshal + request construction paths without
	// making a real network call.
	p := NewOpenRouterProvider("k", "m")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := p.Complete(ctx, CompletionRequest{UserPrompt: "user"})
	if err == nil {
		t.Fatal("expected error when context is canceled")
	}
}

// ---------------------------------------------------------------------------
// parseChatCompletion tests
// ---------------------------------------------------------------------------

func TestParseChatCompletion_Success(t *testing.T) {
	body := []byte(`{"choices":[{"message":{"content":"hello"}}]}`)
	out, err := parseChatCompletion(body, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "hello" {
		t.Fatalf("expected 'hello', got %q", out)
	}
}

func TestParseChatCompletion_APIError(t *testing.T) {
	body := []byte(`{"error":{"message":"rate limit"}}`)
	_, err := parseChatCompletion(body, "test")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Fatalf("expected 'rate limit' in error, got %v", err)
	}
}

func TestParseChatCompletion_NoChoices(t *testing.T) {
	body := []byte(`{"choices":[]}`)
	_, err := parseChatCompletion(body, "test")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no choices") {
		t.Fatalf("expected 'no choices' in error, got %v", err)
	}
}

func TestParseChatCompletion_MalformedJSON(t *testing.T) {
	body := []byte(`{bad`)
	_, err := parseChatCompletion(body, "test")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Fatalf("expected 'unmarshal' in error, got %v", err)
	}
}
