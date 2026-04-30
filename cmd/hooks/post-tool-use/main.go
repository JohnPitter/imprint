package main

import (
	"encoding/json"
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
	if sessionID == "" {
		os.Exit(0)
	}

	// Claude Code sends "tool_response" (can be string or object)
	toolOutput := ""
	if v, ok := input["tool_response"]; ok {
		switch t := v.(type) {
		case string:
			toolOutput = hooks.TruncateString(t, 8000)
		default:
			b, _ := json.Marshal(t)
			toolOutput = hooks.TruncateString(string(b), 8000)
		}
	}

	hooks.Post(cfg, "/imprint/observe", map[string]any{
		"session_id":  sessionID,
		"hook_type":   "post_tool_use",
		"tool_name":   hooks.GetString(input, "tool_name"),
		"tool_input":  input["tool_input"],
		"tool_output": toolOutput,
	})
}
