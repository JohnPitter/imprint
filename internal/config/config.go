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
	AnthropicAPIKey       string   // ANTHROPIC_API_KEY (or auto-detected from Claude Code OAuth)
	AnthropicBaseURL      string   // ANTHROPIC_BASE_URL
	AnthropicModel        string   // ANTHROPIC_MODEL, default "claude-haiku-4-5-20251001"
	AnthropicAuthMode     string   // "api_key" or "oauth" (auto-detected)
	OpenAIAPIKey          string   // OPENAI_API_KEY (or auto-detected from ~/.codex/auth.json)
	OpenAIBaseURL         string   // OPENAI_BASE_URL, default "https://api.openai.com"
	OpenAIModel           string   // OPENAI_MODEL, default "gpt-5-mini" (cheap GPT-5 tier, Codex's Haiku equivalent)
	OpenAIReasoningEffort string   // OPENAI_REASONING_EFFORT, default "minimal" (cheapest); "" lets the model decide
	OpenAIAuthMode        string   // "api_key" or "codex" (auto-detected)
	OpenAIOAuthModel      string   // OPENAI_OAUTH_MODEL, model for the Codex ChatGPT-OAuth path, default "gpt-5"
	CodexOAuthAvailable   bool     // true when ~/.codex/auth.json has ChatGPT OAuth tokens (auto-detected)
	OpenRouterAPIKey      string   // OPENROUTER_API_KEY
	OpenRouterModel       string   // OPENROUTER_MODEL, default "anthropic/claude-haiku-4-5-20251001"
	LlamaCppURL           string   // LLAMACPP_URL, default "http://localhost:8080"
	LlamaCppModel         string   // LLAMACPP_MODEL, default "" (server decides)
	LLMProviderOrder      []string // LLM_PROVIDER_ORDER, default ["anthropic","openrouter","llamacpp"]

	// Embedding
	EmbeddingProvider string // EMBEDDING_PROVIDER, default "llamacpp"
	EmbeddingModel    string // EMBEDDING_MODEL, default ""

	// Search
	BM25Weight   float64 // BM25_WEIGHT, default 0.4
	VectorWeight float64 // VECTOR_WEIGHT, default 0.4
	GraphWeight  float64 // GRAPH_WEIGHT, default 0.2

	// Pipeline
	CompressWorkers      int    // COMPRESS_WORKERS, default 4
	ConsolidationEnabled bool   // CONSOLIDATION_ENABLED, default true
	ClaudeBridgeEnabled  bool   // CLAUDE_MEMORY_BRIDGE, default false
	PipelineIntervalMin  int    // PIPELINE_INTERVAL_MIN, default 5 (0 = disabled)
	ExtractionMode       string // IMPRINT_EXTRACTION_MODE, "hybrid" (default) | "llm-only"
	//                          // hybrid:   regex pre-pass for files/concepts/etc, LLM only for narrative
	//                          // llm-only: legacy behavior, send everything to the LLM

	// Phase 3 importance gate: skip the LLM for trivial observations (capture
	// them deterministically into the base layer), spending Haiku only on what
	// can become a refined memory. Calibrate the threshold against the saldo.
	CompressFilterEnabled bool // COMPRESS_FILTER, default true
	CompressMinImportance int  // COMPRESS_MIN_IMPORTANCE, score below which the LLM is skipped, default 4

	// Limits
	MaxObservationsPerSession int // MAX_OBS_PER_SESSION, default 500
	ContextTokenBudget        int // CONTEXT_TOKEN_BUDGET, default 2000
	ToolOutputMaxLen          int // TOOL_OUTPUT_MAX_LEN, default 8000

	// Token economy (Phase 1) — budget ceiling + plan-aware display.
	// The ceiling protects *before* spend; defaults are high enough not to touch
	// a normal session but catch a runaway loop. 0 = unlimited. When a cap is hit
	// the background pauses non-essential Haiku and injection falls to the
	// minimum, without breaking the main path (invariant 6).
	Plan                     string  // IMPRINT_PLAN: "api"|"pro"|"max", "" = auto-detect
	MaxHaikuTokensPerSession int     // IMPRINT_MAX_HAIKU_TOKENS_SESSION, default 500000 (0 = unlimited)
	MaxHaikuTokensPerDay     int     // IMPRINT_MAX_HAIKU_TOKENS_DAY, default 3000000 (0 = unlimited)
	MaxInjectionTokens       int     // IMPRINT_MAX_INJECTION_TOKENS, default 0 (use layer budgets)
	HaikuPriceInPerMTok      float64 // IMPRINT_HAIKU_PRICE_IN, USD per 1M input tokens (Haiku 4.5 ≈ 1.0)
	HaikuPriceOutPerMTok     float64 // IMPRINT_HAIKU_PRICE_OUT, USD per 1M output tokens (Haiku 4.5 ≈ 5.0)
	OpenAIPriceInPerMTok     float64 // OPENAI_PRICE_IN, USD per 1M input tokens (gpt-5-mini ≈ 0.25)
	OpenAIPriceOutPerMTok    float64 // OPENAI_PRICE_OUT, USD per 1M output tokens (gpt-5-mini ≈ 2.0)

	// Memory decay
	DecayMinStrength int // DECAY_MIN_STRENGTH, default 3 — memórias com strength <= este valor viram candidatas a archive
	DecayMaxAgeDays  int // DECAY_MAX_AGE_DAYS, default 30 — idade mínima pra arquivar uma memória fraca

	// Intuition / rooted layer (Phase 2). Birth bar is deliberately high (3.5):
	// an intuition only forms when many refined insights converge across distinct
	// sessions. Residency is capped so the always-on context cost stays bounded.
	IntuitionMinStrength      int     // INTUITION_MIN_STRENGTH, refined memories considered, default 6
	IntuitionMinConvergence   int     // INTUITION_MIN_CONVERGENCE, min converging insights, default 4
	IntuitionMinSessions      int     // INTUITION_MIN_SESSIONS, distinct sessions spanned, default 2
	IntuitionMaxActive        int     // INTUITION_MAX_ACTIVE, hard residency cap per repo, default 5
	IntuitionContradictionHit float64 // INTUITION_CONTRADICTION_HIT, strength drop per contradiction, default 2.0
	IntuitionDemoteFloor      float64 // INTUITION_DEMOTE_FLOOR, strength at/below which it demotes, default 3.0

	// Lazy injection (Phase 2) — pull refined memory on demand when a turn touches
	// a theme, instead of dumping it all at session start (the biggest economy lever).
	LazyInjectMax int // LAZY_INJECT_MAX, max refined memories pulled per lazy call, default 5

	// Phase 4 — code graph as a relevance signal. Blast radius depth: when a turn
	// touches a file, memories about files within this many graph hops are pulled.
	BlastRadiusDepth int // BLAST_RADIUS_DEPTH, default 2 (0 disables the boost)

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

		AnthropicAPIKey:       envStr("ANTHROPIC_API_KEY", ""),
		AnthropicBaseURL:      envStr("ANTHROPIC_BASE_URL", "https://api.anthropic.com"),
		AnthropicModel:        envStr("ANTHROPIC_MODEL", "claude-haiku-4-5-20251001"),
		OpenAIAPIKey:          envStr("OPENAI_API_KEY", ""),
		OpenAIBaseURL:         envStr("OPENAI_BASE_URL", "https://api.openai.com"),
		OpenAIModel:           envStr("OPENAI_MODEL", "gpt-5-mini"),
		OpenAIReasoningEffort: envStr("OPENAI_REASONING_EFFORT", "minimal"),
		OpenAIOAuthModel:      envStr("OPENAI_OAUTH_MODEL", "gpt-5"),
		OpenRouterAPIKey:      envStr("OPENROUTER_API_KEY", ""),
		OpenRouterModel:       envStr("OPENROUTER_MODEL", "anthropic/claude-haiku-4-5-20251001"),
		LlamaCppURL:           envStr("LLAMACPP_URL", "http://localhost:8080"),
		LlamaCppModel:         envStr("LLAMACPP_MODEL", ""),
		LLMProviderOrder:      envList("LLM_PROVIDER_ORDER", []string{"anthropic", "codex-oauth", "openai", "openrouter", "llamacpp"}),

		EmbeddingProvider: envStr("EMBEDDING_PROVIDER", "llamacpp"),
		EmbeddingModel:    envStr("EMBEDDING_MODEL", ""),

		BM25Weight:   envFloat("BM25_WEIGHT", 0.4),
		VectorWeight: envFloat("VECTOR_WEIGHT", 0.4),
		GraphWeight:  envFloat("GRAPH_WEIGHT", 0.2),

		CompressWorkers:      envInt("COMPRESS_WORKERS", 4),
		ConsolidationEnabled: envBool("CONSOLIDATION_ENABLED", true),
		ClaudeBridgeEnabled:  envBool("CLAUDE_MEMORY_BRIDGE", false),
		PipelineIntervalMin:  envInt("PIPELINE_INTERVAL_MIN", 5),
		ExtractionMode:       envStr("IMPRINT_EXTRACTION_MODE", "hybrid"),

		CompressFilterEnabled: envBool("COMPRESS_FILTER", true),
		CompressMinImportance: envInt("COMPRESS_MIN_IMPORTANCE", 4),

		MaxObservationsPerSession: envInt("MAX_OBS_PER_SESSION", 500),
		ContextTokenBudget:        envInt("CONTEXT_TOKEN_BUDGET", 2000),
		ToolOutputMaxLen:          envInt("TOOL_OUTPUT_MAX_LEN", 8000),

		Plan:                     envStr("IMPRINT_PLAN", ""),
		MaxHaikuTokensPerSession: envInt("IMPRINT_MAX_HAIKU_TOKENS_SESSION", 500000),
		MaxHaikuTokensPerDay:     envInt("IMPRINT_MAX_HAIKU_TOKENS_DAY", 3000000),
		MaxInjectionTokens:       envInt("IMPRINT_MAX_INJECTION_TOKENS", 0),
		HaikuPriceInPerMTok:      envFloat("IMPRINT_HAIKU_PRICE_IN", 1.0),
		HaikuPriceOutPerMTok:     envFloat("IMPRINT_HAIKU_PRICE_OUT", 5.0),
		OpenAIPriceInPerMTok:     envFloat("OPENAI_PRICE_IN", 0.25),
		OpenAIPriceOutPerMTok:    envFloat("OPENAI_PRICE_OUT", 2.0),

		DecayMinStrength: envInt("DECAY_MIN_STRENGTH", 3),
		DecayMaxAgeDays:  envInt("DECAY_MAX_AGE_DAYS", 30),

		IntuitionMinStrength:      envInt("INTUITION_MIN_STRENGTH", 6),
		IntuitionMinConvergence:   envInt("INTUITION_MIN_CONVERGENCE", 4),
		IntuitionMinSessions:      envInt("INTUITION_MIN_SESSIONS", 2),
		IntuitionMaxActive:        envInt("INTUITION_MAX_ACTIVE", 5),
		IntuitionContradictionHit: envFloat("INTUITION_CONTRADICTION_HIT", 2.0),
		IntuitionDemoteFloor:      envFloat("INTUITION_DEMOTE_FLOOR", 3.0),

		LazyInjectMax:    envInt("LAZY_INJECT_MAX", 5),
		BlastRadiusDepth: envInt("BLAST_RADIUS_DEPTH", 2),

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

	// Auto-detect OpenAI auth so Codex users get cheap GPT-5 background work with
	// zero config: an explicit OPENAI_API_KEY wins; otherwise reuse the API key
	// Codex stored in ~/.codex/auth.json (api-key login mode only).
	if cfg.OpenAIAPIKey != "" {
		cfg.OpenAIAuthMode = "api_key"
	} else if key := detectCodexAPIKey(); key != "" {
		cfg.OpenAIAPIKey = key
		cfg.OpenAIAuthMode = "codex"
	}

	// Detect Codex ChatGPT-OAuth login (subscription tokens). The provider reads
	// the tokens itself; this flag just drives logging and the economy plan view.
	cfg.CodexOAuthAvailable = detectCodexOAuth()

	return cfg, nil
}

// detectCodexOAuth reports whether ~/.codex/auth.json holds ChatGPT OAuth tokens
// (a `codex login` with a ChatGPT account, not an API key). Best-effort.
func detectCodexOAuth() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(filepath.Join(home, ".codex", "auth.json"))
	if err != nil {
		return false
	}
	var auth struct {
		Tokens *struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(data, &auth); err != nil || auth.Tokens == nil {
		return false
	}
	return auth.Tokens.AccessToken != "" || auth.Tokens.RefreshToken != ""
}

// detectCodexAPIKey reads an OpenAI API key from Codex's credential file at
// ~/.codex/auth.json (written by `codex login` in api-key mode). Returns "" if
// the file is absent or uses ChatGPT-OAuth mode (those tokens aren't usable on
// the standard Chat Completions API). Best-effort and never fatal.
func detectCodexAPIKey() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(home, ".codex", "auth.json"))
	if err != nil {
		return ""
	}
	var auth struct {
		OpenAIAPIKey string `json:"OPENAI_API_KEY"`
	}
	if err := json.Unmarshal(data, &auth); err != nil {
		return ""
	}
	return auth.OpenAIAPIKey
}

// detectClaudeCodeOAuth reads the Claude Code OAuth token from the
// platform-appropriate credential store (Keychain on macOS, file on Linux,
// file fallback on Windows). Returns "" if missing or expired.
func detectClaudeCodeOAuth() string {
	data, err := readClaudeCodeCredentialsRaw()
	if err != nil || len(data) == 0 {
		return ""
	}
	return extractOAuthToken(data, currentUnixTime())
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
