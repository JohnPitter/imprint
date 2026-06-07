// codex-hook adapts the official Codex hook wire format to Imprint's
// observation/session HTTP API.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"imprint/internal/hooks"
)

type hookInput struct {
	SessionID             string          `json:"session_id"`
	TranscriptPath        *string         `json:"transcript_path"`
	Cwd                   string          `json:"cwd"`
	HookEventName         string          `json:"hook_event_name"`
	Model                 string          `json:"model"`
	TurnID                string          `json:"turn_id"`
	Source                string          `json:"source"`
	ToolName              string          `json:"tool_name"`
	ToolUseID             string          `json:"tool_use_id"`
	ToolInput             json.RawMessage `json:"tool_input"`
	ToolResponse          json.RawMessage `json:"tool_response"`
	Prompt                string          `json:"prompt"`
	LastAssistantMessage  *string         `json:"last_assistant_message"`
	StopHookAlreadyActive bool            `json:"stop_hook_active"`
}

func main() {
	var input hookInput
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		logLine("codex-hook: failed to parse stdin: " + err.Error())
		emitContinue(input.HookEventName)
		return
	}

	cfg := hooks.LoadConfig()
	sessionID := codexSessionID(input.SessionID)

	switch input.HookEventName {
	case "SessionStart":
		handleSessionStart(cfg, input, sessionID)
	case "UserPromptSubmit":
		handleUserPromptSubmit(cfg, input, sessionID)
	case "PreToolUse":
		handlePreToolUse(cfg, input, sessionID)
	case "PostToolUse":
		handlePostToolUse(cfg, input, sessionID)
	case "Stop":
		handleStop(cfg, input, sessionID)
	default:
		emitContinue(input.HookEventName)
	}
}

func handleSessionStart(cfg hooks.Config, input hookInput, sessionID string) {
	project := filepath.Base(input.Cwd)
	if project == "." || project == string(filepath.Separator) || project == "" {
		project = "codex"
	}
	result, err := hooks.Post(cfg, "/imprint/session/start", map[string]any{
		"sessionId": sessionID,
		"project":   project,
		"cwd":       input.Cwd,
	})
	if err != nil {
		logLine("codex-hook: session start failed: " + err.Error())
		emitContinue("SessionStart")
		return
	}
	_, _ = hooks.Post(cfg, "/imprint/session/heartbeat", map[string]any{"sessionId": sessionID})

	context, _ := result["context"].(string)
	if context == "" {
		emitContinue("SessionStart")
		return
	}
	emitContext("SessionStart", context)
}

func handleUserPromptSubmit(cfg hooks.Config, input hookInput, sessionID string) {
	if input.Prompt != "" {
		observeWithSession(cfg, input, sessionID, map[string]any{
			"session_id":  sessionID,
			"hook_type":   "prompt_submit",
			"project_dir": input.Cwd,
			"user_prompt": hooks.TruncateString(input.Prompt, 8000),
			"metadata":    metadata(input),
			"timestamp":   time.Now(),
		})
	}
	_, _ = hooks.Post(cfg, "/imprint/session/heartbeat", map[string]any{"sessionId": sessionID})
	emitContinue("UserPromptSubmit")
}

func handlePreToolUse(cfg hooks.Config, input hookInput, sessionID string) {
	observeWithSession(cfg, input, sessionID, map[string]any{
		"session_id":  sessionID,
		"hook_type":   "pre_tool_use",
		"project_dir": input.Cwd,
		"tool_name":   input.ToolName,
		"tool_input":  input.ToolInput,
		"metadata":    metadata(input),
		"timestamp":   time.Now(),
	})
	emitContinue("PreToolUse")
}

func handlePostToolUse(cfg hooks.Config, input hookInput, sessionID string) {
	observeWithSession(cfg, input, sessionID, map[string]any{
		"session_id":  sessionID,
		"hook_type":   "post_tool_use",
		"project_dir": input.Cwd,
		"tool_name":   input.ToolName,
		"tool_input":  input.ToolInput,
		"tool_output": input.ToolResponse,
		"metadata":    metadata(input),
		"timestamp":   time.Now(),
	})
	emitContinue("PostToolUse")
}

func handleStop(cfg hooks.Config, input hookInput, sessionID string) {
	if input.LastAssistantMessage != nil && *input.LastAssistantMessage != "" {
		out, _ := json.Marshal(hooks.TruncateString(*input.LastAssistantMessage, 8000))
		observeWithSession(cfg, input, sessionID, map[string]any{
			"session_id":  sessionID,
			"hook_type":   "stop",
			"project_dir": input.Cwd,
			"tool_name":   "codex_assistant",
			"tool_output": json.RawMessage(out),
			"metadata":    metadata(input),
			"timestamp":   time.Now(),
		})
	}
	_, _ = hooks.Post(cfg, "/imprint/session/heartbeat", map[string]any{"sessionId": sessionID})
	emitContinue("Stop")
}

func observeWithSession(cfg hooks.Config, input hookInput, sessionID string, payload map[string]any) {
	if postObserve(cfg, payload) {
		return
	}
	ensureSession(cfg, input, sessionID)
	postObserve(cfg, payload)
}

func ensureSession(cfg hooks.Config, input hookInput, sessionID string) {
	project := filepath.Base(input.Cwd)
	if project == "." || project == string(filepath.Separator) || project == "" {
		project = "codex"
	}
	_, _ = hooks.Post(cfg, "/imprint/session/start", map[string]any{
		"sessionId": sessionID,
		"project":   project,
		"cwd":       input.Cwd,
	})
}

func postObserve(cfg hooks.Config, payload map[string]any) bool {
	if _, err := hooks.Post(cfg, "/imprint/observe", payload); err != nil {
		logLine("codex-hook: observe failed: " + err.Error())
		return false
	}
	return true
}

func metadata(input hookInput) json.RawMessage {
	data, _ := json.Marshal(map[string]any{
		"source":          "codex-hook",
		"hook_event_name": input.HookEventName,
		"turn_id":         input.TurnID,
		"tool_use_id":     input.ToolUseID,
		"transcript_path": input.TranscriptPath,
		"model":           input.Model,
	})
	return data
}

func codexSessionID(id string) string {
	if id == "" {
		return "codex_unknown"
	}
	if len(id) >= 6 && id[:6] == "codex_" {
		return id
	}
	return "codex_" + id
}

func emitContext(event, context string) {
	emit(map[string]any{
		"continue": true,
		"hookSpecificOutput": map[string]any{
			"hookEventName":     event,
			"additionalContext": context,
		},
	})
}

func emitContinue(event string) {
	if event == "" {
		return
	}
	emit(map[string]any{"continue": true})
}

func emit(v any) {
	data, _ := json.Marshal(v)
	fmt.Println(string(data))
}

func logLine(msg string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".imprint")
	_ = os.MkdirAll(dir, 0o755)
	f, err := os.OpenFile(filepath.Join(dir, "codex-hook.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s %s\n", time.Now().UTC().Format(time.RFC3339), msg)
}
