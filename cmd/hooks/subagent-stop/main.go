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

	hooks.Post(cfg, "/imprint/observe", map[string]any{
		"session_id": hooks.GetString(input, "session_id"),
		"hook_type":  "subagent_stop",
		"agent_id":   hooks.GetString(input, "agent_id"),
	})
}
