package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func resetSettingsCache() {
	settingsMu.Lock()
	cachedSettings = nil
	settingsMu.Unlock()
}

func TestSettingsPath(t *testing.T) {
	got := SettingsPath("/tmp/data")
	want := filepath.Join("/tmp/data", "settings.json")
	if got != want {
		t.Errorf("SettingsPath: got %q, want %q", got, want)
	}
}

func TestLoadUserSettings_MissingFile(t *testing.T) {
	resetSettingsCache()
	dir := t.TempDir()

	s, err := LoadUserSettings(dir)
	if err != nil {
		t.Fatalf("LoadUserSettings: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil UserSettings")
	}
	if s.LLMProvider != "" {
		t.Errorf("expected zero-value settings, got provider %q", s.LLMProvider)
	}
}

func TestLoadUserSettings_ValidFile(t *testing.T) {
	resetSettingsCache()
	dir := t.TempDir()

	input := UserSettings{
		LLMProvider:     "anthropic",
		AnthropicModel:  "claude-haiku-4-5",
		AnthropicAPIKey: "sk-ant-test",
	}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(SettingsPath(dir), data, 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	s, err := LoadUserSettings(dir)
	if err != nil {
		t.Fatalf("LoadUserSettings: %v", err)
	}
	if s.LLMProvider != "anthropic" {
		t.Errorf("LLMProvider = %q, want anthropic", s.LLMProvider)
	}
	if s.AnthropicModel != "claude-haiku-4-5" {
		t.Errorf("AnthropicModel = %q", s.AnthropicModel)
	}
}

func TestLoadUserSettings_MalformedJSON(t *testing.T) {
	resetSettingsCache()
	dir := t.TempDir()

	if err := os.WriteFile(SettingsPath(dir), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := LoadUserSettings(dir)
	if err == nil {
		t.Fatal("expected error on malformed JSON, got nil")
	}
}

func TestLoadUserSettings_UsesCache(t *testing.T) {
	resetSettingsCache()
	dir := t.TempDir()

	first := UserSettings{LLMProvider: "anthropic"}
	data, _ := json.Marshal(first)
	if err := os.WriteFile(SettingsPath(dir), data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadUserSettings(dir); err != nil {
		t.Fatal(err)
	}

	// Overwrite file; Load should still return cached copy.
	second := UserSettings{LLMProvider: "openrouter"}
	data2, _ := json.Marshal(second)
	if err := os.WriteFile(SettingsPath(dir), data2, 0o644); err != nil {
		t.Fatalf("write2: %v", err)
	}

	s, err := LoadUserSettings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.LLMProvider != "anthropic" {
		t.Errorf("expected cached value 'anthropic', got %q", s.LLMProvider)
	}
}

func TestSaveUserSettings_PersistsAndUpdatesCache(t *testing.T) {
	resetSettingsCache()
	dir := t.TempDir()

	s := &UserSettings{
		LLMProvider:      "openrouter",
		OpenRouterAPIKey: "sk-or-test",
	}
	if err := SaveUserSettings(dir, s); err != nil {
		t.Fatalf("SaveUserSettings: %v", err)
	}

	data, err := os.ReadFile(SettingsPath(dir))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var got UserSettings
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.LLMProvider != "openrouter" {
		t.Errorf("on disk: LLMProvider = %q", got.LLMProvider)
	}

	loaded, err := LoadUserSettings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.OpenRouterAPIKey != "sk-or-test" {
		t.Errorf("cache not updated after save")
	}
}

func TestApplyUserSettings_OverridesSetFields(t *testing.T) {
	cfg := &Config{
		AnthropicModel:   "default-model",
		LLMProviderOrder: []string{"anthropic"},
		BM25Weight:       0.5,
		CompressWorkers:  1,
	}
	bm := 0.75
	vw := 0.25
	workers := 4
	enabled := false
	budget := 8000
	interval := 15

	s := &UserSettings{
		AnthropicModel:       "user-model",
		AnthropicAPIKey:      "sk-user",
		OpenRouterModel:      "or-model",
		OpenRouterAPIKey:     "sk-or",
		LlamaCppURL:          "http://localhost:9999",
		LlamaCppModel:        "llama-user",
		ProviderOrder:        []string{"llamacpp", "anthropic"},
		BM25Weight:           &bm,
		VectorWeight:         &vw,
		CompressWorkers:      &workers,
		ConsolidationEnabled: &enabled,
		ContextTokenBudget:   &budget,
		PipelineIntervalMin:  &interval,
	}

	ApplyUserSettings(cfg, s)

	if cfg.AnthropicModel != "user-model" {
		t.Errorf("AnthropicModel = %q", cfg.AnthropicModel)
	}
	if cfg.AnthropicAPIKey != "sk-user" {
		t.Errorf("AnthropicAPIKey = %q", cfg.AnthropicAPIKey)
	}
	if cfg.AnthropicAuthMode != "api_key" {
		t.Errorf("AnthropicAuthMode = %q, want api_key", cfg.AnthropicAuthMode)
	}
	if cfg.OpenRouterModel != "or-model" || cfg.OpenRouterAPIKey != "sk-or" {
		t.Errorf("openrouter fields not applied")
	}
	if cfg.LlamaCppURL != "http://localhost:9999" || cfg.LlamaCppModel != "llama-user" {
		t.Errorf("llamacpp fields not applied")
	}
	if len(cfg.LLMProviderOrder) != 2 || cfg.LLMProviderOrder[0] != "llamacpp" {
		t.Errorf("provider order not applied: %v", cfg.LLMProviderOrder)
	}
	if cfg.BM25Weight != 0.75 || cfg.VectorWeight != 0.25 {
		t.Errorf("search weights not applied")
	}
	if cfg.CompressWorkers != 4 {
		t.Errorf("CompressWorkers = %d", cfg.CompressWorkers)
	}
	if cfg.ConsolidationEnabled != false {
		t.Errorf("ConsolidationEnabled not applied")
	}
	if cfg.ContextTokenBudget != 8000 || cfg.PipelineIntervalMin != 15 {
		t.Errorf("pipeline fields not applied")
	}
}

func TestApplyUserSettings_EmptyLeavesCfgUnchanged(t *testing.T) {
	cfg := &Config{
		AnthropicModel:  "original",
		BM25Weight:      0.5,
		CompressWorkers: 2,
	}
	ApplyUserSettings(cfg, &UserSettings{})
	if cfg.AnthropicModel != "original" {
		t.Errorf("AnthropicModel changed: %q", cfg.AnthropicModel)
	}
	if cfg.BM25Weight != 0.5 {
		t.Errorf("BM25Weight changed: %v", cfg.BM25Weight)
	}
	if cfg.CompressWorkers != 2 {
		t.Errorf("CompressWorkers changed: %d", cfg.CompressWorkers)
	}
}

func TestConfigToPublicView_MasksKeys(t *testing.T) {
	cfg := &Config{
		LLMProviderOrder:   []string{"anthropic", "llamacpp"},
		AnthropicModel:     "claude-haiku",
		AnthropicAPIKey:    "sk-ant-abcdefghijklmnop",
		AnthropicAuthMode:  "api_key",
		OpenRouterModel:    "or-m",
		OpenRouterAPIKey:   "short",
		LlamaCppURL:        "http://localhost:8080",
		LlamaCppModel:      "llama-3",
		BM25Weight:         0.5,
		VectorWeight:       0.3,
		GraphWeight:        0.2,
		CompressWorkers:    2,
		ContextTokenBudget: 6000,
		Port:               8765,
		DataDir:            "/home/user/.imprint",
	}

	view := ConfigToPublicView(cfg)
	llm, ok := view["llm"].(map[string]any)
	if !ok {
		t.Fatal("llm section missing")
	}
	masked := llm["anthropicApiKey"].(string)
	if masked == cfg.AnthropicAPIKey {
		t.Errorf("API key not masked: %q", masked)
	}
	if !strings.Contains(masked, "...") {
		t.Errorf("masked key should contain '...': %q", masked)
	}
	if llm["openrouterApiKey"].(string) != "****" {
		t.Errorf("short key should be full-masked, got %q", llm["openrouterApiKey"])
	}
	if llm["anthropicModel"] != "claude-haiku" {
		t.Errorf("model should be visible")
	}
	if _, ok := view["search"].(map[string]any); !ok {
		t.Error("search section missing")
	}
	if _, ok := view["pipeline"].(map[string]any); !ok {
		t.Error("pipeline section missing")
	}
	if _, ok := view["server"].(map[string]any); !ok {
		t.Error("server section missing")
	}
}

func TestConfigToPublicView_EmptyKey(t *testing.T) {
	cfg := &Config{}
	view := ConfigToPublicView(cfg)
	llm := view["llm"].(map[string]any)
	if llm["anthropicApiKey"] != "" {
		t.Errorf("empty key should remain empty, got %q", llm["anthropicApiKey"])
	}
}
