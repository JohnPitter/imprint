package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"imprint/internal/hooks"
)

// transcriptEntry represents a single line in a Claude Code JSONL transcript.
type transcriptEntry struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`

	ToolName     string          `json:"tool_name,omitempty"`
	ToolInput    json.RawMessage `json:"tool_input,omitempty"`
	ToolResponse json.RawMessage `json:"tool_response,omitempty"`
}

func main() {
	cfg := hooks.LoadConfig()
	cfg.Timeout = 30 * time.Second

	input, err := hooks.ReadStdin()
	if err != nil {
		os.Exit(0)
	}

	sessionID := hooks.GetString(input, "session_id")
	if sessionID == "" {
		os.Exit(0)
	}

	// Close the kanban turn: every action this turn opened (in_progress) is
	// finalized as done. Stop fires at the end of each assistant response, so
	// this matches the lifecycle the user sees in chat.
	hooks.Post(cfg, "/imprint/actions/complete-in-progress", map[string]any{
		"sessionId": sessionID,
	})

	// Process transcript JSONL to capture any missed observations.
	// This is lightweight: just POSTs raw observations that the compression
	// worker will handle asynchronously.
	transcriptPath := hooks.GetString(input, "transcript_path")
	if transcriptPath != "" {
		processTranscript(cfg, transcriptPath, sessionID)
	}

	// The background scheduler already handles summarize + consolidate
	// during the session. No heavy pipeline calls needed here.
}

// processTranscript reads the Claude Code JSONL transcript, extracts tool_use
// entries, and POSTs each as an observation.
func processTranscript(cfg hooks.Config, transcriptPath, sessionID string) {
	f, err := os.Open(transcriptPath)
	if err != nil {
		return
	}
	defer f.Close()

	observeCfg := cfg
	observeCfg.Timeout = 10 * time.Second

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry transcriptEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		if entry.Type != "result" || entry.Subtype != "tool_use" {
			continue
		}
		if entry.ToolName == "" {
			continue
		}

		toolOutput := hooks.TruncateString(extractToolOutput(entry.ToolResponse), 8000)

		payload := map[string]any{
			"session_id":  sessionID,
			"hook_type":   "post_tool_use",
			"project_dir": filepath.Dir(transcriptPath),
			"tool_name":   entry.ToolName,
			"tool_input":  entry.ToolInput,
			"tool_output": toolOutput,
		}

		hooks.Post(observeCfg, "/imprint/observe", payload)
	}
}

func extractToolOutput(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}
