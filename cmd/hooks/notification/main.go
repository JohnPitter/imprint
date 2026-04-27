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

	notifType := hooks.GetString(input, "type")
	if notifType != "permission_prompt" {
		os.Exit(0)
	}

	sessionID := hooks.GetString(input, "session_id")
	message := hooks.GetString(input, "message")

	hooks.Post(cfg, "/imprint/observe", map[string]any{
		"session_id": sessionID,
		"hook_type":  "notification",
		"type":       notifType,
	})

	// Permission prompts surface in the Actions kanban as "pending" so the user
	// can see at a glance that Claude is waiting on them.
	title := message
	if title == "" {
		title = "Awaiting your authorization"
	}
	hooks.Post(cfg, "/imprint/actions/from-task", map[string]any{
		"sessionId":   sessionID,
		"title":       title,
		"description": "Claude is waiting for permission to proceed.",
		"status":      "pending",
	})
}
