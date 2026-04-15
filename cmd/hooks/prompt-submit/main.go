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
		"session_id":  hooks.GetString(input, "session_id"),
		"hook_type":   "prompt_submit",
		"tool_name":   "",
		"tool_input":  nil,
		"tool_output": nil,
		"user_prompt": hooks.GetString(input, "prompt"),
	})
}
