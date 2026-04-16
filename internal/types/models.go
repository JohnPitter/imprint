package types

import (
	"encoding/json"
	"time"
)

// ContextBlock represents a segment of context assembled for injection.
type ContextBlock struct {
	Type     string `json:"type"`
	Label    string `json:"label"`
	Content  string `json:"content"`
	Priority int    `json:"priority"`
}

// HookPayload is the JSON structure sent by hook binaries to the /observe endpoint.
type HookPayload struct {
	SessionID    string          `json:"session_id"`
	HookType     HookType        `json:"hook_type"`
	ProjectDir   string          `json:"project_dir"`
	ToolName     *string         `json:"tool_name,omitempty"`
	ToolInput    json.RawMessage `json:"tool_input,omitempty"`
	ToolOutput   json.RawMessage `json:"tool_output,omitempty"`
	UserPrompt   *string         `json:"user_prompt,omitempty"`
	FilePath     *string         `json:"file_path,omitempty"`
	Error        *string         `json:"error,omitempty"`
	Prompt       *string         `json:"prompt,omitempty"`
	Notification *string         `json:"notification,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	Timestamp    time.Time       `json:"timestamp"`
}
