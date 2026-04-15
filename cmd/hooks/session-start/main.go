package main

import (
	"fmt"
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
	cwd := hooks.GetString(input, "cwd")
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	result, err := hooks.Post(cfg, "/imprint/session/start", map[string]string{
		"sessionId": sessionID,
		"project":   cwd,
		"cwd":       cwd,
	})
	if err != nil {
		os.Exit(0)
	}

	// Write context to stdout for injection into Claude
	if ctx, ok := result["context"].(string); ok && ctx != "" {
		fmt.Print(ctx)
	}
}
