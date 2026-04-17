package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// UserSettings holds user-configurable settings saved to ~/.imprint/settings.json.
// These override the defaults from env vars. Only non-zero values override.
type UserSettings struct {
	// LLM Provider
	LLMProvider      string `json:"llmProvider"`     // "anthropic", "openrouter", "llamacpp"
	AnthropicModel   string `json:"anthropicModel"`  // e.g. "claude-haiku-4-5-20251001"
	AnthropicAPIKey  string `json:"anthropicApiKey"` // manual API key (not shown in UI)
	OpenRouterModel  string `json:"openrouterModel"`
	OpenRouterAPIKey string `json:"openrouterApiKey"`
	LlamaCppURL      string `json:"llamacppUrl"`
	LlamaCppModel    string `json:"llamacppModel"`

	// Provider priority order
	ProviderOrder []string `json:"providerOrder"` // e.g. ["anthropic", "llamacpp"]

	// Search weights
	BM25Weight   *float64 `json:"bm25Weight,omitempty"`
	VectorWeight *float64 `json:"vectorWeight,omitempty"`

	// Pipeline
	CompressWorkers      *int  `json:"compressWorkers,omitempty"`
	ConsolidationEnabled *bool `json:"consolidationEnabled,omitempty"`
	ContextTokenBudget   *int  `json:"contextTokenBudget,omitempty"`
	PipelineIntervalMin  *int  `json:"pipelineIntervalMin,omitempty"` // periodic pipeline interval in minutes (0 = disabled)
}

var (
	settingsMu     sync.RWMutex
	cachedSettings *UserSettings
)

// SettingsPath returns the path to the user settings file.
func SettingsPath(dataDir string) string {
	return filepath.Join(dataDir, "settings.json")
}

// LoadUserSettings reads settings from ~/.imprint/settings.json.
func LoadUserSettings(dataDir string) (*UserSettings, error) {
	settingsMu.RLock()
	if cachedSettings != nil {
		defer settingsMu.RUnlock()
		return cachedSettings, nil
	}
	settingsMu.RUnlock()

	path := SettingsPath(dataDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &UserSettings{}, nil
		}
		return nil, err
	}

	var s UserSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}

	settingsMu.Lock()
	cachedSettings = &s
	settingsMu.Unlock()

	return &s, nil
}

// SaveUserSettings writes settings to ~/.imprint/settings.json.
func SaveUserSettings(dataDir string, s *UserSettings) error {
	path := SettingsPath(dataDir)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}

	settingsMu.Lock()
	cachedSettings = s
	settingsMu.Unlock()

	return nil
}

// ApplyUserSettings merges user settings into the Config.
// User settings override env-based config where set.
func ApplyUserSettings(cfg *Config, s *UserSettings) {
	if s.AnthropicModel != "" {
		cfg.AnthropicModel = s.AnthropicModel
	}
	if s.AnthropicAPIKey != "" {
		cfg.AnthropicAPIKey = s.AnthropicAPIKey
		cfg.AnthropicAuthMode = "api_key"
	}
	if s.OpenRouterModel != "" {
		cfg.OpenRouterModel = s.OpenRouterModel
	}
	if s.OpenRouterAPIKey != "" {
		cfg.OpenRouterAPIKey = s.OpenRouterAPIKey
	}
	if s.LlamaCppURL != "" {
		cfg.LlamaCppURL = s.LlamaCppURL
	}
	if s.LlamaCppModel != "" {
		cfg.LlamaCppModel = s.LlamaCppModel
	}
	if len(s.ProviderOrder) > 0 {
		cfg.LLMProviderOrder = s.ProviderOrder
	}
	if s.BM25Weight != nil {
		cfg.BM25Weight = *s.BM25Weight
	}
	if s.VectorWeight != nil {
		cfg.VectorWeight = *s.VectorWeight
	}
	if s.CompressWorkers != nil {
		cfg.CompressWorkers = *s.CompressWorkers
	}
	if s.ConsolidationEnabled != nil {
		cfg.ConsolidationEnabled = *s.ConsolidationEnabled
	}
	if s.ContextTokenBudget != nil {
		cfg.ContextTokenBudget = *s.ContextTokenBudget
	}
	if s.PipelineIntervalMin != nil {
		cfg.PipelineIntervalMin = *s.PipelineIntervalMin
	}
}

// ConfigToPublicView returns a sanitized view of the config for the UI.
// API keys are masked.
func ConfigToPublicView(cfg *Config) map[string]any {
	maskKey := func(key string) string {
		if key == "" {
			return ""
		}
		if len(key) <= 8 {
			return "****"
		}
		return key[:4] + "..." + key[len(key)-4:]
	}

	return map[string]any{
		"llm": map[string]any{
			"providerOrder":    cfg.LLMProviderOrder,
			"anthropicModel":   cfg.AnthropicModel,
			"anthropicApiKey":  maskKey(cfg.AnthropicAPIKey),
			"anthropicAuth":    cfg.AnthropicAuthMode,
			"openrouterModel":  cfg.OpenRouterModel,
			"openrouterApiKey": maskKey(cfg.OpenRouterAPIKey),
			"llamacppUrl":      cfg.LlamaCppURL,
			"llamacppModel":    cfg.LlamaCppModel,
		},
		"search": map[string]any{
			"bm25Weight":   cfg.BM25Weight,
			"vectorWeight": cfg.VectorWeight,
			"graphWeight":  cfg.GraphWeight,
		},
		"pipeline": map[string]any{
			"compressWorkers":      cfg.CompressWorkers,
			"consolidationEnabled": cfg.ConsolidationEnabled,
			"contextTokenBudget":   cfg.ContextTokenBudget,
			"pipelineIntervalMin":  cfg.PipelineIntervalMin,
		},
		"server": map[string]any{
			"port":    cfg.Port,
			"dataDir": cfg.DataDir,
		},
	}
}
