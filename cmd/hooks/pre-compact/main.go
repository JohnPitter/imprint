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

	// 1. Save a snapshot observation before context is lost.
	//    This captures the fact that compaction happened (the scheduler will
	//    summarize/consolidate the observations that existed before this point).
	toolName := "ContextCompaction"
	hooks.Post(cfg, "/imprint/observe", map[string]any{
		"session_id":  sessionID,
		"hook_type":   "pre_compact",
		"tool_name":   &toolName,
		"tool_input":  nil,
		"tool_output": "Context window compacted. Observations before this point may lose conversation context.",
	})

	// 2. Retrieve relevant context to inject back after compaction.
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
