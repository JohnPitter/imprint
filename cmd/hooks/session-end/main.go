package main

import (
	"os"

	"imprint/internal/hooks"
)

func main() {
	cfg := hooks.LoadConfig()
	input, err := hooks.ReadStdin()
	if err != nil {
		os.Exit(0)
	}

	sessionID := hooks.GetString(input, "session_id")
	if sessionID == "" {
		os.Exit(0)
	}

	hooks.Post(cfg, "/imprint/session/end", map[string]string{"sessionId": sessionID})
	// Optionally trigger summarization
	hooks.Post(cfg, "/imprint/summarize", map[string]string{"sessionId": sessionID})
}
