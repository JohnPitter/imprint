package server

import (
	"embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"imprint/internal/config"
)

func newTestRouter() http.Handler {
	cfg := &config.Config{
		ViewerAllowedOrigins: []string{"*"},
	}
	var assets embed.FS
	return NewRouter(cfg, assets, nil)
}

func TestLivez(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/imprint/livez", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", body["status"])
	}
	if _, ok := body["timestamp"]; !ok {
		t.Error("expected 'timestamp' field in response")
	}
}

func TestHealth(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/imprint/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	requiredFields := []string{"status", "goVersion", "goroutines", "memory"}
	for _, field := range requiredFields {
		if _, ok := body[field]; !ok {
			t.Errorf("expected %q field in health response", field)
		}
	}

	if body["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %q", body["status"])
	}

	memory, ok := body["memory"].(map[string]any)
	if !ok {
		t.Fatal("expected 'memory' to be a map")
	}
	for _, field := range []string{"allocMB", "sysMB", "numGC"} {
		if _, ok := memory[field]; !ok {
			t.Errorf("expected %q field in memory stats", field)
		}
	}
}

func TestNotImplementedEndpoints(t *testing.T) {
	router := newTestRouter()

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/imprint/session/start"},
		{http.MethodPost, "/imprint/session/end"},
		{http.MethodGet, "/imprint/sessions"},
		{http.MethodPost, "/imprint/observe"},
		{http.MethodPost, "/imprint/search"},
		{http.MethodPost, "/imprint/remember"},
		{http.MethodGet, "/imprint/memories"},
		{http.MethodPost, "/imprint/graph/extract"},
		{http.MethodGet, "/imprint/export"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotImplemented {
				t.Errorf("expected 501, got %d", rec.Code)
			}

			var body map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if body["error"] != "not implemented" {
				t.Errorf("expected error 'not implemented', got %q", body["error"])
			}
		})
	}
}
