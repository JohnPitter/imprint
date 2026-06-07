package config

import (
	"os"
	"path/filepath"
	"testing"
)

// homeEnvVar is the env var os.UserHomeDir reads on each platform.
func setFakeHome(t *testing.T, dir string) {
	t.Helper()
	if os.PathSeparator == '\\' { // Windows
		t.Setenv("USERPROFILE", dir)
	} else {
		t.Setenv("HOME", dir)
	}
}

func TestDetectCodexAPIKey(t *testing.T) {
	home := t.TempDir()
	setFakeHome(t, home)
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexDir, "auth.json"),
		[]byte(`{"OPENAI_API_KEY":"sk-codex-123"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := detectCodexAPIKey(); got != "sk-codex-123" {
		t.Errorf("detectCodexAPIKey() = %q, want sk-codex-123", got)
	}
}

func TestDetectCodexAPIKey_MissingFile(t *testing.T) {
	setFakeHome(t, t.TempDir()) // no .codex/auth.json
	if got := detectCodexAPIKey(); got != "" {
		t.Errorf("expected empty when auth.json absent, got %q", got)
	}
}

func TestDetectCodexAPIKey_ChatGPTMode(t *testing.T) {
	home := t.TempDir()
	setFakeHome(t, home)
	codexDir := filepath.Join(home, ".codex")
	_ = os.MkdirAll(codexDir, 0o755)
	// ChatGPT-OAuth login has no OPENAI_API_KEY field — not usable here.
	_ = os.WriteFile(filepath.Join(codexDir, "auth.json"), []byte(`{"tokens":{"access_token":"x"}}`), 0o600)
	if got := detectCodexAPIKey(); got != "" {
		t.Errorf("expected empty for ChatGPT-mode auth.json, got %q", got)
	}
}
