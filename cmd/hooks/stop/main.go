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
	// Use longer timeout for stop hook since it processes transcript + triggers pipelines
	cfg.Timeout = 120 * time.Second

	input, err := hooks.ReadStdin()
	if err != nil {
		os.Exit(0)
	}

	sessionID := hooks.GetString(input, "session_id")
	if sessionID == "" {
		os.Exit(0)
	}

	// 1. Summarize the session (existing behavior)
	hooks.Post(cfg, "/imprint/summarize", map[string]string{"sessionId": sessionID})

	// 2. Read and process the transcript JSONL if available
	transcriptPath := hooks.GetString(input, "transcript_path")
	if transcriptPath != "" {
		processTranscript(cfg, transcriptPath, sessionID)
	}

	// 3. Trigger consolidation pipeline
	hooks.Post(cfg, "/imprint/consolidate-pipeline", map[string]string{"sessionId": sessionID})
}

// processTranscript reads the Claude Code JSONL transcript, extracts tool_use
// entries, and POSTs each as an observation. This processes the session transcript
// silently without spending MCP tokens.
func processTranscript(cfg hooks.Config, transcriptPath, sessionID string) {
	f, err := os.Open(transcriptPath)
	if err != nil {
		return // silently skip if file not readable
	}
	defer f.Close()

	// Use a shorter timeout for individual observe calls
	observeCfg := cfg
	observeCfg.Timeout = 10 * time.Second

	scanner := bufio.NewScanner(f)
	// Allow large lines (transcripts can have big tool outputs)
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

		// Only process tool_use result entries
		if entry.Type != "result" || entry.Subtype != "tool_use" {
			continue
		}
		if entry.ToolName == "" {
			continue
		}

		// Extract tool output as string
		toolOutput := extractToolOutput(entry.ToolResponse)
		if len(toolOutput) > 8000 {
			toolOutput = toolOutput[:8000]
		}

		payload := map[string]any{
			"session_id":  sessionID,
			"hook_type":   "post_tool_use",
			"project_dir": filepath.Dir(transcriptPath),
			"tool_name":   entry.ToolName,
			"tool_input":  entry.ToolInput,
			"tool_output": toolOutput,
		}

		// POST silently; ignore failures
		hooks.Post(observeCfg, "/imprint/observe", payload)
	}
}

// extractToolOutput converts the tool_response JSON to a string.
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
