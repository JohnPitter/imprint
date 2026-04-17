package config

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	// Server
	Port    int    // IMPRINT_PORT, default 3111
	Secret  string // IMPRINT_SECRET, optional Bearer token
	DataDir string // IMPRINT_DATA_DIR, default ~/.imprint

	// LLM
	AnthropicAPIKey   string   // ANTHROPIC_API_KEY (or auto-detected from Claude Code OAuth)
	AnthropicBaseURL  string   // ANTHROPIC_BASE_URL
	AnthropicModel    string   // ANTHROPIC_MODEL, default "claude-haiku-4-5-20251001"
	AnthropicAuthMode string   // "api_key" or "oauth" (auto-detected)
	OpenRouterAPIKey  string   // OPENROUTER_API_KEY
	OpenRouterModel   string   // OPENROUTER_MODEL, default "anthropic/claude-haiku-4-5-20251001"
	LlamaCppURL       string   // LLAMACPP_URL, default "http://localhost:8080"
	LlamaCppModel     string   // LLAMACPP_MODEL, default "" (server decides)
	LLMProviderOrder  []string // LLM_PROVIDER_ORDER, default ["anthropic","openrouter","llamacpp"]

	// Embedding
	EmbeddingProvider string // EMBEDDING_PROVIDER, default "llamacpp"
	EmbeddingModel    string // EMBEDDING_MODEL, default ""

	// Search
	BM25Weight   float64 // BM25_WEIGHT, default 0.4
	VectorWeight float64 // VECTOR_WEIGHT, default 0.4
	GraphWeight  float64 // GRAPH_WEIGHT, default 0.2

	// Pipeline
	CompressWorkers      int  // COMPRESS_WORKERS, default 4
	ConsolidationEnabled bool // CONSOLIDATION_ENABLED, default true
	ClaudeBridgeEnabled  bool // CLAUDE_MEMORY_BRIDGE, default false
	PipelineIntervalMin  int  // PIPELINE_INTERVAL_MIN, default 5 (0 = disabled)

	// Limits
	MaxObservationsPerSession int // MAX_OBS_PER_SESSION, default 500
	ContextTokenBudget        int // CONTEXT_TOKEN_BUDGET, default 2000
	ToolOutputMaxLen          int // TOOL_OUTPUT_MAX_LEN, default 8000

	// MCP
	MCPToolsMode string // IMPRINT_TOOLS, default "core" (or "all")

	// Viewer
	ViewerAllowedOrigins []string // VIEWER_ALLOWED_ORIGINS
}

// Load reads configuration from environment variables and an optional .env file.
// The .env file is loaded from the data directory (~/.imprint/.env by default).
// Existing environment variables are never overridden by the .env file.
func Load() (*Config, error) {
	dataDir := envStr("IMPRINT_DATA_DIR", defaultDataDir())

	envPath := filepath.Join(dataDir, ".env")
	if err := loadEnvFile(envPath); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	cfg := &Config{
		Port:    envInt("IMPRINT_PORT", 3111),
		Secret:  envStr("IMPRINT_SECRET", ""),
		DataDir: dataDir,

		AnthropicAPIKey:  envStr("ANTHROPIC_API_KEY", ""),
		AnthropicBaseURL: envStr("ANTHROPIC_BASE_URL", "https://api.anthropic.com"),
		AnthropicModel:   envStr("ANTHROPIC_MODEL", "claude-haiku-4-5-20251001"),
		OpenRouterAPIKey: envStr("OPENROUTER_API_KEY", ""),
		OpenRouterModel:  envStr("OPENROUTER_MODEL", "anthropic/claude-haiku-4-5-20251001"),
		LlamaCppURL:      envStr("LLAMACPP_URL", "http://localhost:8080"),
		LlamaCppModel:    envStr("LLAMACPP_MODEL", ""),
		LLMProviderOrder: envList("LLM_PROVIDER_ORDER", []string{"anthropic", "openrouter", "llamacpp"}),

		EmbeddingProvider: envStr("EMBEDDING_PROVIDER", "llamacpp"),
		EmbeddingModel:    envStr("EMBEDDING_MODEL", ""),

		BM25Weight:   envFloat("BM25_WEIGHT", 0.4),
		VectorWeight: envFloat("VECTOR_WEIGHT", 0.4),
		GraphWeight:  envFloat("GRAPH_WEIGHT", 0.2),

		CompressWorkers:      envInt("COMPRESS_WORKERS", 4),
		ConsolidationEnabled: envBool("CONSOLIDATION_ENABLED", true),
		ClaudeBridgeEnabled:  envBool("CLAUDE_MEMORY_BRIDGE", false),
		PipelineIntervalMin:  envInt("PIPELINE_INTERVAL_MIN", 5),

		MaxObservationsPerSession: envInt("MAX_OBS_PER_SESSION", 500),
		ContextTokenBudget:        envInt("CONTEXT_TOKEN_BUDGET", 2000),
		ToolOutputMaxLen:          envInt("TOOL_OUTPUT_MAX_LEN", 8000),

		MCPToolsMode: envStr("IMPRINT_TOOLS", "core"),

		ViewerAllowedOrigins: envList("VIEWER_ALLOWED_ORIGINS", nil),
	}

	// Auto-detect Anthropic auth: if no API key, try Claude Code OAuth token
	if cfg.AnthropicAPIKey != "" {
		cfg.AnthropicAuthMode = "api_key"
	} else {
		token := detectClaudeCodeOAuth()
		if token != "" {
			cfg.AnthropicAPIKey = token
			cfg.AnthropicAuthMode = "oauth"
		}
	}

	return cfg, nil
}

// detectClaudeCodeOAuth reads the Claude Code OAuth token from ~/.claude/.credentials.json
func detectClaudeCodeOAuth() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	credPath := filepath.Join(home, ".claude", ".credentials.json")
	data, err := os.ReadFile(credPath)
	if err != nil {
		return ""
	}

	// Minimal JSON parsing to extract claudeAiOauth.accessToken
	type oauthCred struct {
		ClaudeAiOauth struct {
			AccessToken string `json:"accessToken"`
			ExpiresAt   int64  `json:"expiresAt"`
		} `json:"claudeAiOauth"`
	}
	var cred oauthCred
	if err := json.Unmarshal(data, &cred); err != nil {
		return ""
	}

	// Check token is not empty and not expired
	if cred.ClaudeAiOauth.AccessToken == "" {
		return ""
	}
	// expiresAt is in milliseconds
	if cred.ClaudeAiOauth.ExpiresAt > 0 {
		expiresAt := cred.ClaudeAiOauth.ExpiresAt / 1000
		if expiresAt < currentUnixTime() {
			return ""
		}
	}

	return cred.ClaudeAiOauth.AccessToken
}

// currentUnixTime returns the current Unix timestamp in seconds.
func currentUnixTime() int64 {
	return time.Now().Unix()
}

// defaultDataDir returns ~/.imprint as the default data directory.
func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".imprint"
	}
	return filepath.Join(home, ".imprint")
}

// loadEnvFile reads a .env file and sets environment variables.
// Existing environment variables are never overridden.
func loadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		// Strip surrounding quotes (single or double)
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		// Don't override existing environment variables
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}

// envStr returns the value of the environment variable or the default.
func envStr(key, defaultVal string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return defaultVal
}

// envInt returns the integer value of the environment variable or the default.
func envInt(key string, defaultVal int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

// envFloat returns the float64 value of the environment variable or the default.
func envFloat(key string, defaultVal float64) float64 {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return defaultVal
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return defaultVal
	}
	return f
}

// envBool returns the boolean value of the environment variable or the default.
func envBool(key string, defaultVal bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return defaultVal
	}
	return b
}

// envList returns the comma-separated list from the environment variable or the default.
func envList(key string, defaultVal []string) []string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return defaultVal
	}
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return defaultVal
	}
	return result
}
