// Package extract pulls deterministic entities out of raw tool observations
// without calling an LLM. It is a regex/heuristic pre-pass for the
// compression pipeline: most file paths, function references, URLs, errors,
// and git refs in tool input/output are mechanically identifiable, and
// shipping them to Haiku just for re-extraction wastes tokens.
//
// All exported functions are pure: they take input strings and return
// deduplicated, sorted slices. Callers are expected to merge the result with
// whatever the LLM later returns.
package extract

import (
	"regexp"
	"sort"
	"strings"
)

// Result aggregates everything an extractor pass found. Each slice is
// deduplicated and (when ordering matters) lexicographically sorted so two
// runs over the same input produce byte-identical output — important for
// dedup_cache hashing.
type Result struct {
	Files    []string
	Concepts []string
	URLs     []string
	Errors   []string
	GitRefs  []string
}

// Extract runs every detector over the combined tool input/output of a raw
// observation. The toolName argument is currently advisory but lets future
// detectors specialize (e.g. only look for file paths inside Edit/Write).
func Extract(toolName, input, output string) Result {
	combined := input + "\n" + output

	// Strip fenced code blocks and inline code spans so detectors do not
	// match against transient command snippets, sample diffs, etc. that
	// were never real entities. Mirrors gbrain's defense-in-depth strip.
	cleaned := stripCodeFences(combined)

	files := extractFiles(cleaned)
	urls := extractURLs(cleaned)
	gitRefs := extractGitRefs(cleaned)
	errs := extractErrors(cleaned)
	concepts := extractConcepts(cleaned, files)

	return Result{
		Files:    dedupe(files),
		Concepts: dedupe(concepts),
		URLs:     dedupe(urls),
		Errors:   dedupe(errs),
		GitRefs:  dedupe(gitRefs),
	}
}

// ─── File paths ───────────────────────────────────────────

// File path heuristic: a forward-slash separated path with a recognizable
// extension. Avoids matching URLs (those contain `://`) by the leading
// negative lookbehind workaround (we drop matches whose char before is `/`).
//
// Permits relative paths (./foo, ../foo), absolute (/foo), and bare (foo/bar).
var fileRE = regexp.MustCompile(`(?:^|[\s"'(\[<` + "`" + `])((?:\.{0,2}/)?[\w.-]+(?:/[\w.-]+)+\.[a-zA-Z][a-zA-Z0-9]{0,9})\b`)

// Allowed extensions — limit to common dev file types so we don't ingest
// every dotted token in arbitrary stdout. Keep alphabetical for review.
var fileExtAllow = map[string]struct{}{
	"c": {}, "cc": {}, "cjs": {}, "conf": {}, "cpp": {}, "cs": {}, "css": {},
	"csv": {}, "dart": {}, "go": {}, "gradle": {}, "h": {}, "hpp": {},
	"html": {}, "ini": {}, "java": {}, "js": {}, "json": {}, "jsx": {},
	"kt": {}, "kts": {}, "less": {}, "lock": {}, "md": {}, "mjs": {},
	"mod": {}, "php": {}, "proto": {}, "py": {}, "rb": {}, "rs": {},
	"sass": {}, "scala": {}, "scss": {}, "sh": {}, "sql": {}, "sum": {},
	"svelte": {}, "swift": {}, "toml": {}, "ts": {}, "tsx": {}, "txt": {},
	"vue": {}, "xml": {}, "yaml": {}, "yml": {}, "zig": {},
}

func extractFiles(s string) []string {
	out := []string{}
	for _, m := range fileRE.FindAllStringSubmatch(s, -1) {
		path := m[1]
		// Drop URL fragments that the leading-char trick missed.
		if strings.Contains(path, "://") {
			continue
		}
		dot := strings.LastIndex(path, ".")
		if dot < 0 {
			continue
		}
		ext := strings.ToLower(path[dot+1:])
		if _, ok := fileExtAllow[ext]; !ok {
			continue
		}
		out = append(out, path)
	}
	return out
}

// ─── URLs ─────────────────────────────────────────────────

// urlRE: scheme + `//` + host + optional path. Trailing punctuation
// (`.`, `,`, `)`, `]`, `>`) is stripped so URLs in prose don't ingest a stop.
var urlRE = regexp.MustCompile(`https?://[^\s"'<>` + "`" + `]+`)

func extractURLs(s string) []string {
	out := []string{}
	for _, u := range urlRE.FindAllString(s, -1) {
		u = strings.TrimRight(u, ".,;:)]}>'\"")
		if len(u) >= 10 {
			out = append(out, u)
		}
	}
	return out
}

// ─── Git refs ─────────────────────────────────────────────

// 7–40 char hex sequence, word-bounded. Matches commit hashes from `git log`,
// abbreviated SHAs, and full SHA-1 / SHA-256 refs. Won't match arbitrary
// hex tokens because we require length >= 7.
var gitHashRE = regexp.MustCompile(`\b[a-f0-9]{7,40}\b`)

// `branch: feature/foo` or `branch foo`, plus PR/issue refs `#123`.
var prRefRE = regexp.MustCompile(`#\d{1,6}\b`)

func extractGitRefs(s string) []string {
	out := []string{}
	for _, h := range gitHashRE.FindAllString(s, -1) {
		// Filter pure-decimal hex (e.g. "1234567" timestamps) by requiring
		// at least one a-f letter; pure-numeric hashes do exist but are
		// rare enough that the false-positive cost outweighs the recall.
		if !containsHexLetter(h) {
			continue
		}
		out = append(out, h)
	}
	for _, r := range prRefRE.FindAllString(s, -1) {
		out = append(out, r)
	}
	return out
}

// ─── Errors ───────────────────────────────────────────────

// Match common error markers across languages. We capture the type (e.g.
// "TypeError", "panic", "SyntaxError") so the entity is normalized.
var errorRE = regexp.MustCompile(`(?:^|\s)(panic|TypeError|ReferenceError|SyntaxError|ValueError|KeyError|NullPointerException|RuntimeError|AttributeError|IndexError|ImportError|ModuleNotFoundError|fatal error|error: \w+|Error: \w+):?`)

func extractErrors(s string) []string {
	out := []string{}
	for _, m := range errorRE.FindAllStringSubmatch(s, -1) {
		out = append(out, strings.TrimSpace(m[1]))
	}
	return out
}

// ─── Concepts (heuristic only) ────────────────────────────

// Concepts are the squishiest category. We pull them from:
//   - file basenames without extension (foo/bar/Baz.tsx -> "Baz")
//   - PascalCase tokens (likely class/component names)
//
// We deliberately do NOT try to invent semantic concepts — that is what the
// LLM is for. This pass exists so the LLM does not have to repeat the easy
// mechanical work.
// PascalCase: at least two camel-segments. The body of each segment is one
// uppercase letter followed by lowercase letters, so "FooBar" matches and
// "FooBARQux" also matches via the [A-Za-z]+ tail catch-all in the second
// segment. We also require the first segment to be ≥2 chars to avoid
// matching things like "API" alone (which gets caught by other heuristics).
var pascalRE = regexp.MustCompile(`\b[A-Z][a-z]+[A-Z][A-Za-z]+\b`)

func extractConcepts(s string, files []string) []string {
	concepts := []string{}

	// Basenames of files (without extension).
	for _, f := range files {
		base := f
		if i := strings.LastIndex(base, "/"); i >= 0 {
			base = base[i+1:]
		}
		if i := strings.LastIndex(base, "."); i > 0 {
			base = base[:i]
		}
		if len(base) >= 3 {
			concepts = append(concepts, base)
		}
	}

	// PascalCase tokens (classes, components, types).
	seen := map[string]struct{}{}
	for _, m := range pascalRE.FindAllString(s, -1) {
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		concepts = append(concepts, m)
	}

	return concepts
}

// ─── Helpers ──────────────────────────────────────────────

// stripCodeFences removes ```fenced``` and `inline` code spans, replacing
// them with whitespace of equal length so subsequent regex offsets remain
// accurate (irrelevant for our use, but cheap to preserve).
func stripCodeFences(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		// Triple backtick fence.
		if i+2 < len(s) && s[i] == '`' && s[i+1] == '`' && s[i+2] == '`' {
			end := strings.Index(s[i+3:], "```")
			if end < 0 {
				b.WriteString(strings.Repeat(" ", len(s)-i))
				return b.String()
			}
			b.WriteString(strings.Repeat(" ", end+6))
			i += end + 6
			continue
		}
		// Inline code: single backtick, no newline inside.
		if s[i] == '`' {
			end := strings.Index(s[i+1:], "`")
			if end < 0 {
				b.WriteByte(s[i])
				i++
				continue
			}
			if strings.Contains(s[i+1:i+1+end], "\n") {
				b.WriteByte(s[i])
				i++
				continue
			}
			b.WriteString(strings.Repeat(" ", end+2))
			i += end + 2
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func containsHexLetter(s string) bool {
	for _, c := range s {
		if c >= 'a' && c <= 'f' {
			return true
		}
	}
	return false
}

func dedupe(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
