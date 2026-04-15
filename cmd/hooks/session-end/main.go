package main

import (
	"os"
	"time"

	"imprint/internal/hooks"
)

func main() {
	cfg := hooks.LoadConfig()
	// Use longer timeout for session-end since it triggers LLM pipelines
	cfg.Timeout = 120 * time.Second

	input, err := hooks.ReadStdin()
	if err != nil {
		os.Exit(0)
	}

	sessionID := hooks.GetString(input, "session_id")
	if sessionID == "" {
		os.Exit(0)
	}

	// 1. End the session
	hooks.Post(cfg, "/imprint/session/end", map[string]string{"sessionId": sessionID})

	// 2. Summarize (LLM generates session summary)
	hooks.Post(cfg, "/imprint/summarize", map[string]string{"sessionId": sessionID})

	// 3. Consolidate (LLM groups observations into memories + detects patterns as lessons)
	hooks.Post(cfg, "/imprint/consolidate-pipeline", map[string]string{"sessionId": sessionID})
}
