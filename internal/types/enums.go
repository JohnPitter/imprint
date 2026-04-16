package types

// HookType identifies the Claude Code lifecycle event that triggered a hook.
type HookType string

const (
	HookSessionStart    HookType = "session_start"
	HookPromptSubmit    HookType = "prompt_submit"
	HookPreToolUse      HookType = "pre_tool_use"
	HookPostToolUse     HookType = "post_tool_use"
	HookPostToolFailure HookType = "post_tool_failure"
	HookPreCompact      HookType = "pre_compact"
	HookSubagentStart   HookType = "subagent_start"
	HookSubagentStop    HookType = "subagent_stop"
	HookNotification    HookType = "notification"
	HookTaskCompleted   HookType = "task_completion"
	HookStop            HookType = "stop"
	HookSessionEnd      HookType = "session_end"
)

// SessionStatus represents the lifecycle state of a session.
type SessionStatus = string

const (
	SessionActive    SessionStatus = "active"
	SessionCompleted SessionStatus = "completed"
	SessionAbandoned SessionStatus = "abandoned"
)

// MemoryType categorizes long-term memories.
type MemoryType = string

const (
	MemPattern      MemoryType = "pattern"
	MemPreference   MemoryType = "preference"
	MemArchitecture MemoryType = "architecture"
	MemBug          MemoryType = "bug"
	MemWorkflow     MemoryType = "workflow"
	MemFact         MemoryType = "fact"
)
