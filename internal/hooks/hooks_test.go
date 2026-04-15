package hooks

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	cfg := LoadConfig()

	if cfg.BaseURL != DefaultBaseURL {
		t.Errorf("expected BaseURL %q, got %q", DefaultBaseURL, cfg.BaseURL)
	}
	if cfg.Secret != "" {
		t.Errorf("expected empty Secret, got %q", cfg.Secret)
	}
	if cfg.Timeout != DefaultTimeout {
		t.Errorf("expected Timeout %v, got %v", DefaultTimeout, cfg.Timeout)
	}
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	t.Setenv("IMPRINT_URL", "http://custom:9999")
	t.Setenv("IMPRINT_SECRET", "my-secret-key")

	cfg := LoadConfig()

	if cfg.BaseURL != "http://custom:9999" {
		t.Errorf("expected BaseURL %q, got %q", "http://custom:9999", cfg.BaseURL)
	}
	if cfg.Secret != "my-secret-key" {
		t.Errorf("expected Secret %q, got %q", "my-secret-key", cfg.Secret)
	}
}

func TestGetString(t *testing.T) {
	m := map[string]any{
		"name":  "hello",
		"count": 42,
	}

	if got := GetString(m, "name"); got != "hello" {
		t.Errorf("expected %q, got %q", "hello", got)
	}
	if got := GetString(m, "missing"); got != "" {
		t.Errorf("expected empty string for missing key, got %q", got)
	}
	if got := GetString(m, "count"); got != "" {
		t.Errorf("expected empty string for non-string value, got %q", got)
	}
}

func TestTruncateString(t *testing.T) {
	short := "hello"
	if got := TruncateString(short, 10); got != short {
		t.Errorf("short string should be unchanged, got %q", got)
	}

	long := "hello world, this is a long string"
	if got := TruncateString(long, 5); got != "hello" {
		t.Errorf("expected %q, got %q", "hello", got)
	}

	exact := "12345"
	if got := TruncateString(exact, 5); got != exact {
		t.Errorf("exact-length string should be unchanged, got %q", got)
	}
}

func TestPost_MockServer(t *testing.T) {
	var receivedBody map[string]any
	var receivedAuth string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")

		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}

		json.NewDecoder(r.Body).Decode(&receivedBody)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"matched": 3,
		})
	}))
	defer ts.Close()

	cfg := Config{
		BaseURL: ts.URL,
		Secret:  "test-token",
		Timeout: DefaultTimeout,
	}

	body := map[string]any{"query": "test search"}
	result, err := Post(cfg, "/imprint/search", body)
	if err != nil {
		t.Fatalf("Post returned error: %v", err)
	}

	if receivedAuth != "Bearer test-token" {
		t.Errorf("expected Authorization header %q, got %q", "Bearer test-token", receivedAuth)
	}
	if receivedBody["query"] != "test search" {
		t.Errorf("expected query %q in body, got %v", "test search", receivedBody["query"])
	}
	if result["status"] != "ok" {
		t.Errorf("expected status ok, got %v", result["status"])
	}
}

func TestGet_MockServer(t *testing.T) {
	var receivedPath string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path

		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"profiles": []string{"project-a", "project-b"},
		})
	}))
	defer ts.Close()

	cfg := Config{
		BaseURL: ts.URL,
		Secret:  "",
		Timeout: DefaultTimeout,
	}

	result, err := Get(cfg, "/imprint/profile")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	if receivedPath != "/imprint/profile" {
		t.Errorf("expected path %q, got %q", "/imprint/profile", receivedPath)
	}
	if result["profiles"] == nil {
		t.Error("expected profiles in response")
	}
}
