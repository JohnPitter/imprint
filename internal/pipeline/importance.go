package pipeline

import (
	"strings"

	"imprint/internal/extract"
)

// ScoreImportance estimates, with zero LLM cost, how likely a raw observation is
// to become a refined memory — the Phase 3 gate that decides whether to spend
// Haiku on it. Returns 1–10. The signal is deliberately conservative: when in
// doubt it scores high (compress), so the filter only skips the LLM on
// observations that are clearly trivial. The threshold is calibrated against the
// Phase 1 saldo, not guessed.
//
// High signal: errors/stack traces, edits/writes, decisions/bugs/architecture.
// Low signal: read-only navigation (ls/cd/pwd/cat/grep/find), tiny output.
func ScoreImportance(toolName, input, output string, pre extract.Result) int {
	score := 5 // neutral default — unknown tools get compressed

	tool := strings.ToLower(strings.TrimSpace(toolName))

	// Mutations are almost always worth a memory.
	switch {
	case containsAny(tool, "edit", "write", "notebookedit", "multiedit", "applypatch", "str_replace"):
		score += 3
	case containsAny(tool, "bash", "shell", "run", "exec"):
		score += 1 // commands vary wildly; lean on content signals below
	case containsAny(tool, "read", "glob", "grep", "search", "ls", "cat", "find", "list"):
		score -= 2 // read-only navigation rarely becomes a lasting insight
	}

	// Errors are high-signal regardless of tool.
	if len(pre.Errors) > 0 {
		score += 3
	}
	// Files and git refs touched → more likely to matter later.
	if len(pre.Files) > 0 {
		score++
	}
	if len(pre.GitRefs) > 0 {
		score++
	}

	// Content classification: decisions/bugs/architecture/preferences are the
	// stuff refined memories are made of; neutral "fact" gets nothing extra.
	switch ClassifyMemoryType(strings.ToLower(input + " " + output)) {
	case "decision", "bug", "architecture", "preference":
		score += 2
	case "pattern", "workflow":
		score++
	}

	// Trivial size: almost no output and nothing extracted → very low signal.
	if len(strings.TrimSpace(output)) < 40 && len(pre.Files) == 0 && len(pre.Errors) == 0 {
		score -= 2
	}

	if score < 1 {
		score = 1
	}
	if score > 10 {
		score = 10
	}
	return score
}
