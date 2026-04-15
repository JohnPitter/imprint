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

	// Only observe permission_prompt notifications
	notifType := hooks.GetString(input, "type")
	if notifType != "permission_prompt" {
		os.Exit(0)
	}

	hooks.Post(cfg, "/imprint/observe", map[string]any{
		"session_id": hooks.GetString(input, "session_id"),
		"hook_type":  "notification",
		"type":       notifType,
	})
}
