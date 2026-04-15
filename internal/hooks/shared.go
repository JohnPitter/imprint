package hooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

var (
	DefaultBaseURL = "http://localhost:3111"
	DefaultTimeout = 5 * time.Second
)

// Config holds hook configuration from environment variables.
type Config struct {
	BaseURL string
	Secret  string
	Timeout time.Duration
}

// LoadConfig reads hook config from environment.
func LoadConfig() Config {
	baseURL := os.Getenv("IMPRINT_URL")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return Config{
		BaseURL: baseURL,
		Secret:  os.Getenv("IMPRINT_SECRET"),
		Timeout: DefaultTimeout,
	}
}

// ReadStdin reads all JSON from stdin.
func ReadStdin() (map[string]any, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty stdin")
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse stdin JSON: %w", err)
	}
	return payload, nil
}

// Post sends a POST request to the imprint server.
// Returns the response body as parsed JSON.
func Post(cfg Config, path string, body any) (map[string]any, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	client := &http.Client{Timeout: cfg.Timeout}
	req, err := http.NewRequest("POST", cfg.BaseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Secret)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return result, nil
}

// Get sends a GET request.
func Get(cfg Config, path string) (map[string]any, error) {
	client := &http.Client{Timeout: cfg.Timeout}
	req, err := http.NewRequest("GET", cfg.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Secret)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]any
	json.Unmarshal(body, &result)
	return result, nil
}

// GetString safely extracts a string from a map.
func GetString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// TruncateString truncates a string to maxLen.
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
