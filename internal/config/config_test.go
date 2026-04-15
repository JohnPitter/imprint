package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	envVars := []string{
		"IMPRINT_PORT", "IMPRINT_SECRET", "IMPRINT_DATA_DIR",
		"ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL", "ANTHROPIC_MODEL",
		"OPENROUTER_API_KEY", "OPENROUTER_MODEL",
		"LLAMACPP_URL", "LLAMACPP_MODEL",
		"LLM_PROVIDER_ORDER",
		"EMBEDDING_PROVIDER", "EMBEDDING_MODEL",
		"BM25_WEIGHT", "VECTOR_WEIGHT", "GRAPH_WEIGHT",
		"COMPRESS_WORKERS", "CONSOLIDATION_ENABLED", "CLAUDE_MEMORY_BRIDGE",
		"MAX_OBS_PER_SESSION", "CONTEXT_TOKEN_BUDGET", "TOOL_OUTPUT_MAX_LEN",
		"IMPRINT_TOOLS", "VIEWER_ALLOWED_ORIGINS",
	}
	for _, key := range envVars {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Port != 3111 {
		t.Errorf("Port = %d, want 3111", cfg.Port)
	}
	if cfg.Secret != "" {
		t.Errorf("Secret = %q, want empty", cfg.Secret)
	}
	if cfg.AnthropicModel != "claude-haiku-4-5-20251001" {
		t.Errorf("AnthropicModel = %q, want %q", cfg.AnthropicModel, "claude-haiku-4-5-20251001")
	}
	if cfg.LlamaCppURL != "http://localhost:8080" {
		t.Errorf("LlamaCppURL = %q, want %q", cfg.LlamaCppURL, "http://localhost:8080")
	}
	if cfg.EmbeddingProvider != "llamacpp" {
		t.Errorf("EmbeddingProvider = %q, want %q", cfg.EmbeddingProvider, "llamacpp")
	}
	if cfg.BM25Weight != 0.4 {
		t.Errorf("BM25Weight = %f, want 0.4", cfg.BM25Weight)
	}
	if cfg.CompressWorkers != 4 {
		t.Errorf("CompressWorkers = %d, want 4", cfg.CompressWorkers)
	}
	if !cfg.ConsolidationEnabled {
		t.Error("ConsolidationEnabled = false, want true")
	}
	if cfg.ClaudeBridgeEnabled {
		t.Error("ClaudeBridgeEnabled = true, want false")
	}
	if cfg.MaxObservationsPerSession != 500 {
		t.Errorf("MaxObservationsPerSession = %d, want 500", cfg.MaxObservationsPerSession)
	}
	if cfg.MCPToolsMode != "core" {
		t.Errorf("MCPToolsMode = %q, want %q", cfg.MCPToolsMode, "core")
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("IMPRINT_PORT", "9999")
	t.Setenv("IMPRINT_SECRET", "my-secret")
	t.Setenv("IMPRINT_DATA_DIR", t.TempDir())
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")
	t.Setenv("ANTHROPIC_MODEL", "claude-sonnet-4-20250514")
	t.Setenv("LLAMACPP_URL", "http://remote:8080")
	t.Setenv("EMBEDDING_PROVIDER", "anthropic")
	t.Setenv("BM25_WEIGHT", "0.5")
	t.Setenv("COMPRESS_WORKERS", "8")
	t.Setenv("CONSOLIDATION_ENABLED", "false")
	t.Setenv("CLAUDE_MEMORY_BRIDGE", "true")
	t.Setenv("MAX_OBS_PER_SESSION", "1000")
	t.Setenv("IMPRINT_TOOLS", "all")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Port != 9999 {
		t.Errorf("Port = %d, want 9999", cfg.Port)
	}
	if cfg.Secret != "my-secret" {
		t.Errorf("Secret = %q, want %q", cfg.Secret, "my-secret")
	}
	if cfg.AnthropicAPIKey != "sk-ant-test" {
		t.Errorf("AnthropicAPIKey = %q, want %q", cfg.AnthropicAPIKey, "sk-ant-test")
	}
	if cfg.AnthropicAuthMode != "api_key" {
		t.Errorf("AnthropicAuthMode = %q, want %q", cfg.AnthropicAuthMode, "api_key")
	}
	if cfg.LlamaCppURL != "http://remote:8080" {
		t.Errorf("LlamaCppURL = %q, want %q", cfg.LlamaCppURL, "http://remote:8080")
	}
	if cfg.EmbeddingProvider != "anthropic" {
		t.Errorf("EmbeddingProvider = %q, want %q", cfg.EmbeddingProvider, "anthropic")
	}
	if cfg.BM25Weight != 0.5 {
		t.Errorf("BM25Weight = %f, want 0.5", cfg.BM25Weight)
	}
	if cfg.CompressWorkers != 8 {
		t.Errorf("CompressWorkers = %d, want 8", cfg.CompressWorkers)
	}
	if cfg.ConsolidationEnabled {
		t.Error("ConsolidationEnabled = true, want false")
	}
	if !cfg.ClaudeBridgeEnabled {
		t.Error("ClaudeBridgeEnabled = false, want true")
	}
	if cfg.MaxObservationsPerSession != 1000 {
		t.Errorf("MaxObservationsPerSession = %d, want 1000", cfg.MaxObservationsPerSession)
	}
	if cfg.MCPToolsMode != "all" {
		t.Errorf("MCPToolsMode = %q, want %q", cfg.MCPToolsMode, "all")
	}
}

func TestLoadEnvFile(t *testing.T) {
	dir := t.TempDir()
	envContent := `# Comment line
IMPRINT_PORT=4444
IMPRINT_SECRET="env-file-secret"
LLAMACPP_MODEL='custom-model'
EMBEDDING_MODEL=custom-embed
`
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	os.Unsetenv("IMPRINT_PORT")
	os.Unsetenv("IMPRINT_SECRET")
	os.Unsetenv("LLAMACPP_MODEL")
	os.Unsetenv("EMBEDDING_MODEL")

	t.Setenv("IMPRINT_DATA_DIR", dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Port != 4444 {
		t.Errorf("Port = %d, want 4444", cfg.Port)
	}
	if cfg.Secret != "env-file-secret" {
		t.Errorf("Secret = %q, want %q", cfg.Secret, "env-file-secret")
	}
	if cfg.LlamaCppModel != "custom-model" {
		t.Errorf("LlamaCppModel = %q, want %q", cfg.LlamaCppModel, "custom-model")
	}
	if cfg.EmbeddingModel != "custom-embed" {
		t.Errorf("EmbeddingModel = %q, want %q", cfg.EmbeddingModel, "custom-embed")
	}
}

func TestLoad_ProviderOrder(t *testing.T) {
	t.Setenv("IMPRINT_DATA_DIR", t.TempDir())
	t.Setenv("LLM_PROVIDER_ORDER", "llamacpp, anthropic")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(cfg.LLMProviderOrder) != 2 {
		t.Fatalf("LLMProviderOrder len = %d, want 2", len(cfg.LLMProviderOrder))
	}
	if cfg.LLMProviderOrder[0] != "llamacpp" {
		t.Errorf("LLMProviderOrder[0] = %q, want %q", cfg.LLMProviderOrder[0], "llamacpp")
	}
	if cfg.LLMProviderOrder[1] != "anthropic" {
		t.Errorf("LLMProviderOrder[1] = %q, want %q", cfg.LLMProviderOrder[1], "anthropic")
	}
}

func TestLoad_OAuthAutoDetect(t *testing.T) {
	// When ANTHROPIC_API_KEY is not set but Claude Code credentials exist,
	// the OAuth token should be auto-detected.
	// This test just verifies the auth mode logic when API key IS set.
	t.Setenv("IMPRINT_DATA_DIR", t.TempDir())
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-api03-test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.AnthropicAuthMode != "api_key" {
		t.Errorf("AnthropicAuthMode = %q, want api_key", cfg.AnthropicAuthMode)
	}
	if cfg.AnthropicAPIKey != "sk-ant-api03-test" {
		t.Errorf("AnthropicAPIKey = %q, want sk-ant-api03-test", cfg.AnthropicAPIKey)
	}
}
