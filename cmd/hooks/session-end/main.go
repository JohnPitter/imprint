package main

import (
	"os"
	"time"

	"imprint/internal/hooks"
)

func main() {
	cfg := hooks.LoadConfig()
	cfg.Timeout = 60 * time.Second

	input, err := hooks.ReadStdin()
	if err != nil {
		os.Exit(0)
	}

	sessionID := hooks.GetString(input, "session_id")
	if sessionID == "" {
		os.Exit(0)
	}

	// End the session — the background scheduler already handles
	// summarize + consolidate during the session lifetime.
	// We only need to mark the session as completed here.
	hooks.Post(cfg, "/imprint/session/end", map[string]string{"sessionId": sessionID})

	// Run a final lightweight pipeline pass for things the scheduler doesn't cover:
	// graph extraction, actions, crystal, reflect.
	// This is much faster than the old full pipeline.
	hooks.Post(cfg, "/imprint/finalize", map[string]string{"sessionId": sessionID})
}
