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

	result, err := hooks.Post(cfg, "/imprint/context", map[string]any{
		"sessionId": sessionID,
		"project":   cwd,
		"budget":    1500,
	})
	if err != nil {
		os.Exit(0)
	}

	if ctx, ok := result["context"].(string); ok && ctx != "" {
		fmt.Print(ctx)
	}
}
