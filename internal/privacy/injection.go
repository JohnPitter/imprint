package privacy

import "regexp"

// Prompt-injection mitigation. Tool outputs (file contents, stdout, search
// results) can contain attacker-authored text crafted to manipulate the LLM
// when the observation is later replayed as memory context. We can't reliably
// detect every variant, but we can neutralize the most common surface
// patterns by replacing them with a marker so the original text can no
// longer hijack instruction parsing.
//
// This is defense in depth, not a guarantee. Three principles:
//
//  1. False positives are cheap (the marker still preserves narrative meaning),
//     false negatives are expensive (an injected instruction may execute).
//     Patterns are tuned aggressive — we'd rather over-flag than miss.
//
//  2. We scrub on write, not on read. By the time text reaches a memory it
//     has already been neutralized, so every consumer (search hit, context
//     injection, MCP recall) inherits the protection without each one
//     having to re-implement it.
//
//  3. We do not strip; we replace with `[FLAGGED:reason]`. Stripping would
//     destroy useful narrative context; flagging keeps the surrounding
//     content searchable while making the suspicious span obvious to a
//     human reviewing the audit log.
var injectionPatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	// Direct override attempts.
	{"override", regexp.MustCompile(`(?i)\b(?:ignore|disregard|forget)\s+(?:all\s+)?(?:prior|previous|above|earlier|preceding|the)\s+(?:instructions?|prompt|context|rules?|directives?|messages?)`)},
	{"override", regexp.MustCompile(`(?i)\bnew\s+(?:instructions?|directives?|rules?|task)\s*:`)},
	{"override", regexp.MustCompile(`(?i)\b(?:override|cancel|terminate)\s+(?:the\s+)?(?:above|previous|system|user)`)},

	// Role/identity hijack ("you are now ...").
	{"role-hijack", regexp.MustCompile(`(?i)\byou\s+are\s+(?:now|actually|really)\s+(?:a|an|the)?\s*\w+`)},
	{"role-hijack", regexp.MustCompile(`(?i)\b(?:assume|adopt|switch\s+to)\s+(?:the\s+)?(?:role|persona|identity)\s+of\b`)},
	{"role-hijack", regexp.MustCompile(`(?i)\bact\s+as\s+(?:if\s+you\s+were\s+|a\s+|an\s+|the\s+)`)},

	// System / developer message spoofing.
	{"spoof", regexp.MustCompile(`(?i)<\s*/?\s*(?:system|admin|developer|sudo|root)\s*[:>]`)},
	{"spoof", regexp.MustCompile(`(?i)\[\s*(?:system|admin|developer|root)\s+(?:override|message|note|reminder)`)},
	{"spoof", regexp.MustCompile(`(?i)^\s*(?:SYSTEM|ADMIN|DEVELOPER)\s*[:>]\s*`)},

	// Tool / MCP invocation impersonation.
	{"tool-impersonation", regexp.MustCompile(`(?i)\b(?:please\s+)?(?:run|execute|invoke|call)\s+(?:tool|function|command)\s+\w+\s*\(`)},
	{"tool-impersonation", regexp.MustCompile(`(?i)\b(?:memory_save|memory_forget|memory_recall)\s*\(`)},

	// Credential exfiltration prompts.
	{"exfil", regexp.MustCompile(`(?i)\b(?:print|reveal|show|display|output|dump)\s+(?:the\s+)?(?:system\s+prompt|api\s*key|secret|credentials?|env(?:ironment)?\s+variables?)`)},
	{"exfil", regexp.MustCompile(`(?i)\bwhat\s+(?:are|is)\s+your\s+(?:instructions|system\s+prompt|original\s+prompt|secret)`)},

	// Instruction-fence breakouts ("```" closing then injecting).
	{"fence-breakout", regexp.MustCompile("```\\s*\\n+\\s*(?i)(?:human|user|assistant|system)\\s*:")},
}

// ScrubInjection replaces suspected prompt-injection spans with a flagged
// marker. Returns the cleaned text. Counts are not surfaced; the audit log
// already records the original raw observation if forensics are needed.
func ScrubInjection(text string) string {
	out := text
	for _, p := range injectionPatterns {
		out = p.re.ReplaceAllString(out, "[FLAGGED:"+p.name+"]")
	}
	return out
}

// ScrubAll runs every privacy filter (secrets + injection) in the order
// callers should normally apply them. Secret stripping first so a redacted
// API key cannot accidentally satisfy an injection pattern that would have
// matched only the surrounding text.
func ScrubAll(text string) string {
	return ScrubInjection(StripPrivateData(text))
}
