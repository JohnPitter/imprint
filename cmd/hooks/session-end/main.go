package main

import (
	"os"
	"time"

	"imprint/internal/hooks"
)

func main() {
	cfg := hooks.LoadConfig()
	cfg.Timeout = 300 * time.Second // Full pipeline needs time

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

	// 2. Run full pipeline (summarize + consolidate + graph + actions + crystal + reflect)
	hooks.Post(cfg, "/imprint/consolidate", map[string]string{"sessionId": sessionID})
}
