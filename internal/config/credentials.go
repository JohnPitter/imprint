package config

import (
	"encoding/json"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

// readClaudeCodeCredentialsRaw returns the parsed contents of the Claude Code
// OAuth credentials store, regardless of where the host platform stores them.
//
//	macOS    → Keychain item "Claude Code-credentials"
//	Windows  → Credential Manager (not yet supported; falls back to file)
//	Linux    → ~/.claude/.credentials.json
//
// On macOS we still fall back to the file path so existing Linux-style installs
// (or users who maintain the file manually) keep working.
func readClaudeCodeCredentialsRaw() ([]byte, error) {
	if runtime.GOOS == "darwin" {
		if data, err := readMacKeychain(); err == nil && len(data) > 0 {
			return data, nil
		}
	}
	return readCredentialsFile()
}

func readCredentialsFile() ([]byte, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(home, ".claude", ".credentials.json"))
}

func readMacKeychain() ([]byte, error) {
	args := []string{"find-generic-password", "-s", "Claude Code-credentials"}
	if u, err := user.Current(); err == nil && u.Username != "" {
		args = append(args, "-a", u.Username)
	}
	args = append(args, "-w")
	out, err := exec.Command("security", args...).Output()
	if err != nil {
		return nil, err
	}
	return []byte(strings.TrimSpace(string(out))), nil
}

// extractOAuthToken pulls the (still-valid) accessToken out of the credentials
// blob. Returns "" if the token is missing or expired.
func extractOAuthToken(data []byte, nowUnix int64) string {
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
	if cred.ClaudeAiOauth.AccessToken == "" {
		return ""
	}
	if cred.ClaudeAiOauth.ExpiresAt > 0 {
		// expiresAt is in milliseconds since epoch
		if cred.ClaudeAiOauth.ExpiresAt/1000 < nowUnix {
			return ""
		}
	}
	return cred.ClaudeAiOauth.AccessToken
}
