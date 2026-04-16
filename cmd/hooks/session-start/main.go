package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"imprint/internal/hooks"
)

func main() {
	cfg := hooks.LoadConfig()
	input, err := hooks.ReadStdin()
	if err != nil {
		hookLog(fmt.Sprintf("session-start: failed to read stdin: %v", err))
		os.Exit(0)
	}

	sessionID := hooks.GetString(input, "session_id")
	cwd := hooks.GetString(input, "cwd")
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	payload := map[string]string{
		"sessionId": sessionID,
		"project":   cwd,
		"cwd":       cwd,
	}

	// Retry up to 3 times with backoff to handle server still starting up
	var result map[string]any
	var lastErr error
	for attempt := range 3 {
		result, lastErr = hooks.Post(cfg, "/imprint/session/start", payload)
		if lastErr == nil {
			break
		}
		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}

	if lastErr != nil {
		hookLog(fmt.Sprintf("session-start: failed after 3 attempts: %v (sessionId=%s, cwd=%s)", lastErr, sessionID, cwd))
		os.Exit(0)
	}

	// Write context to stdout for injection into Claude
	if ctx, ok := result["context"].(string); ok && ctx != "" {
		fmt.Print(ctx)
	}
}

// hookLog appends a timestamped line to ~/.imprint/hooks.log.
func hookLog(msg string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	logPath := filepath.Join(home, ".imprint", "hooks.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().UTC().Format(time.RFC3339), msg)
}
