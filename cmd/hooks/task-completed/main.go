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
	subject := hooks.GetString(input, "subject")
	description := hooks.GetString(input, "description")

	if subject == "" {
		os.Exit(0)
	}

	// 1. Create/update an action to sync the kanban with Claude Code tasks
	hooks.Post(cfg, "/imprint/actions/from-task", map[string]any{
		"sessionId":   sessionID,
		"title":       subject,
		"description": description,
		"status":      "done",
	})

	// 2. Also log as observation for the memory pipeline
	toolName := "TaskCompleted"
	hooks.Post(cfg, "/imprint/observe", map[string]any{
		"session_id":  sessionID,
		"hook_type":   "task_completion",
		"tool_name":   &toolName,
		"tool_input":  nil,
		"tool_output": subject + ": " + description,
	})
}
