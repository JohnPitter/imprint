package main

import (
	"os"
	"unicode/utf8"

	"imprint/internal/hooks"
)

func main() {
	cfg := hooks.LoadConfig()
	input, err := hooks.ReadStdin()
	if err != nil {
		os.Exit(0)
	}

	prompt := hooks.GetString(input, "prompt")
	if prompt == "" {
		os.Exit(0)
	}

	sessionID := hooks.GetString(input, "session_id")
	if sessionID == "" {
		os.Exit(0)
	}

	// The user's prompt is the most valuable signal — it captures intent.
	// Store as a conversation observation with high importance so the
	// compression pipeline uses it as context anchor for subsequent tool calls.
	toolName := "UserPrompt"
	hooks.Post(cfg, "/imprint/observe", map[string]any{
		"session_id":  sessionID,
		"hook_type":   "prompt_submit",
		"tool_name":   &toolName,
		"tool_input":  nil,
		"tool_output": prompt,
		"user_prompt": prompt,
	})

	// Surface the active task in the kanban as "in_progress" so the user
	// can see what Claude is working on right now.
	// Truncate by rune count, not bytes, so multi-byte chars (acentos, emoji)
	// don't get sliced in half and produce invalid UTF-8.
	title := prompt
	if utf8.RuneCountInString(title) > 80 {
		runes := []rune(title)
		title = string(runes[:77]) + "..."
	}
	hooks.Post(cfg, "/imprint/actions/from-task", map[string]any{
		"sessionId":   sessionID,
		"title":       title,
		"description": "Working on it…",
		"status":      "in_progress",
	})
}
