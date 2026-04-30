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

	toolName := hooks.GetString(input, "tool_name")

	// Extract tool response (can be string or object)
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

	// Extract tool input for error context
	var toolInput json.RawMessage
	if v, ok := input["tool_input"]; ok {
		b, _ := json.Marshal(v)
		toolInput = b
	}

	// Failures get a distinct hook_type so the pipeline can detect
	// repeated error patterns and auto-generate lessons.
	// The error output is the key signal for learning.
	hooks.Post(cfg, "/imprint/observe", map[string]any{
		"session_id":  sessionID,
		"hook_type":   "tool_error",
		"tool_name":   &toolName,
		"tool_input":  toolInput,
		"tool_output": "ERROR: " + toolOutput,
	})
}
