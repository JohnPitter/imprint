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

	prompt := hooks.GetString(input, "prompt")
	if prompt == "" {
		os.Exit(0)
	}

	// The user's prompt is the most valuable signal — it captures intent.
	// Store as a conversation observation with high importance so the
	// compression pipeline uses it as context anchor for subsequent tool calls.
	toolName := "UserPrompt"
	hooks.Post(cfg, "/imprint/observe", map[string]any{
		"session_id":  hooks.GetString(input, "session_id"),
		"hook_type":   "prompt_submit",
		"tool_name":   &toolName,
		"tool_input":  nil,
		"tool_output": prompt,
		"user_prompt": prompt,
	})
}
