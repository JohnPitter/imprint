package pipeline

import "strings"

// ClassifyMemoryType uses regex-free keyword heuristics to classify text into
// a memory type. This is a Go port of MemPalace's general_extractor.py,
// adapted for the Imprint memory type vocabulary.
//
// Returns one of: "decision", "preference", "pattern", "bug", "architecture", "workflow", "fact".
//
// Use as a fallback when the LLM doesn't specify a type, or to pre-classify
// observations before LLM processing (saving tokens on obvious classifications).
func ClassifyMemoryType(text string) string {
	lower := strings.ToLower(text)

	// Decision patterns — choices made, trade-offs evaluated.
	if containsAny(lower,
		"decided to", "we chose", "going with", "decision:",
		"opted for", "agreed to", "let's use", "let's go with",
		"settled on", "went with", "picked", "trade-off",
		"pros and cons", "better approach", "instead of using",
	) {
		return "decision"
	}

	// Preference patterns — personal/team style rules.
	if containsAny(lower,
		"i prefer", "always use", "never use", "better to",
		"instead of", "rather than", "don't ever use",
		"my rule is", "convention is", "we always", "we never",
		"please always", "please never",
	) {
		return "preference"
	}

	// Bug patterns — errors, fixes, root causes.
	if containsAny(lower,
		"bug", "fix", "error", "crash", "broken", "regression",
		"issue", "root cause", "workaround", "the problem was",
		"doesn't work", "not working", "keeps failing",
	) {
		return "bug"
	}

	// Architecture patterns — system design, structure, layers.
	if containsAny(lower,
		"architecture", "design pattern", "structure",
		"layer", "module", "interface", "abstraction",
		"dependency", "coupling", "separation of concerns",
		"infrastructure", "framework",
	) {
		return "architecture"
	}

	// Workflow patterns — processes, steps, procedures.
	if containsAny(lower,
		"workflow", "process", "pipeline", "steps to",
		"how to", "procedure", "deploy", "release",
		"ci/cd", "build step", "run the",
	) {
		return "workflow"
	}

	// Pattern — observations about code behavior, recurring truths.
	if containsAny(lower,
		"when", "always", "typically", "usually",
		"should", "must", "tends to", "turns out",
		"the trick is", "the key is", "realized that",
	) {
		return "pattern"
	}

	// Default: fact — neutral information.
	return "fact"
}

// containsAny returns true if text contains any of the given substrings.
func containsAny(text string, patterns ...string) bool {
	for _, p := range patterns {
		if strings.Contains(text, p) {
			return true
		}
	}
	return false
}
