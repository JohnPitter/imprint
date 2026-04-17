package search

import (
	"log"
	"regexp"
	"strings"
)

// Sanitizer constants — tuned to match the MemPalace query sanitizer behavior.
const (
	safeQueryLength = 300 // Below this, query is almost certainly clean.
	maxQueryLength  = 500 // Above this, we force-truncate.
	minQueryLength  = 10  // Extracted result shorter than this = extraction failed.
)

var (
	// sentenceSplitRe splits on . ! ? (including fullwidth) and newlines.
	sentenceSplitRe = regexp.MustCompile(`[.!?\x{3002}\x{FF01}\x{FF1F}\n]+`)

	// questionMarkRe detects sentences ending with ? or fullwidth ?.
	questionMarkRe = regexp.MustCompile(`[?\x{FF1F}]\s*["']?\s*$`)
)

// SanitizeResult holds the sanitized query and metadata about what happened.
type SanitizeResult struct {
	CleanQuery     string `json:"cleanQuery"`
	WasSanitized   bool   `json:"wasSanitized"`
	OriginalLength int    `json:"originalLength"`
	CleanLength    int    `json:"cleanLength"`
	Method         string `json:"method"` // "passthrough", "question_extraction", "tail_sentence", "tail_truncation"
}

// SanitizeQuery cleans a search query that may have system prompt contamination.
//
// When Claude searches, it sometimes prepends its system prompt (2000+ chars)
// to the actual query, causing catastrophic search failure. This function
// extracts the actual question from the contaminated input.
//
// Steps:
//  1. If query <= 300 chars, return as-is (passthrough)
//  2. Try to extract the sentence containing '?' (the actual question)
//  3. If no '?', take the last meaningful sentence
//  4. If still too long (>500), take last 300 chars
func SanitizeQuery(query string) string {
	r := SanitizeQueryDetailed(query)
	return r.CleanQuery
}

// SanitizeQueryDetailed is like SanitizeQuery but returns full metadata.
func SanitizeQueryDetailed(query string) SanitizeResult {
	if query == "" || strings.TrimSpace(query) == "" {
		return SanitizeResult{
			CleanQuery:     query,
			WasSanitized:   false,
			OriginalLength: len(query),
			CleanLength:    len(query),
			Method:         "passthrough",
		}
	}

	query = strings.TrimSpace(query)
	originalLength := len(query)

	// Step 1: Short query passthrough.
	if originalLength <= safeQueryLength {
		return SanitizeResult{
			CleanQuery:     query,
			WasSanitized:   false,
			OriginalLength: originalLength,
			CleanLength:    originalLength,
			Method:         "passthrough",
		}
	}

	// Step 2: Question extraction.
	// Split on newlines and find segments ending with '?'
	segments := splitSegments(query)

	// Search from the end (actual query is usually last).
	for i := len(segments) - 1; i >= 0; i-- {
		seg := strings.TrimSpace(segments[i])
		if questionMarkRe.MatchString(seg) && len(seg) >= minQueryLength {
			candidate := trimCandidate(seg)
			if len(candidate) >= minQueryLength {
				log.Printf("[sanitizer] Query sanitized: %d -> %d chars (method=question_extraction)",
					originalLength, len(candidate))
				return SanitizeResult{
					CleanQuery:     candidate,
					WasSanitized:   true,
					OriginalLength: originalLength,
					CleanLength:    len(candidate),
					Method:         "question_extraction",
				}
			}
		}
	}

	// Also check sentence-split results for question marks.
	sentences := sentenceSplitRe.Split(query, -1)
	for i := len(sentences) - 1; i >= 0; i-- {
		sent := strings.TrimSpace(sentences[i])
		if (strings.Contains(sent, "?") || strings.Contains(sent, "\uFF1F")) && len(sent) >= minQueryLength {
			candidate := trimCandidate(sent)
			if len(candidate) >= minQueryLength {
				log.Printf("[sanitizer] Query sanitized: %d -> %d chars (method=question_extraction)",
					originalLength, len(candidate))
				return SanitizeResult{
					CleanQuery:     candidate,
					WasSanitized:   true,
					OriginalLength: originalLength,
					CleanLength:    len(candidate),
					Method:         "question_extraction",
				}
			}
		}
	}

	// Step 3: Tail sentence extraction.
	for i := len(segments) - 1; i >= 0; i-- {
		seg := strings.TrimSpace(segments[i])
		if len(seg) >= minQueryLength {
			candidate := trimCandidate(seg)
			if len(candidate) >= minQueryLength {
				log.Printf("[sanitizer] Query sanitized: %d -> %d chars (method=tail_sentence)",
					originalLength, len(candidate))
				return SanitizeResult{
					CleanQuery:     candidate,
					WasSanitized:   true,
					OriginalLength: originalLength,
					CleanLength:    len(candidate),
					Method:         "tail_sentence",
				}
			}
		}
	}

	// Step 4: Tail truncation (fallback).
	candidate := query
	if len(candidate) > safeQueryLength {
		candidate = candidate[len(candidate)-safeQueryLength:]
	}
	candidate = strings.TrimSpace(candidate)
	log.Printf("[sanitizer] Query sanitized: %d -> %d chars (method=tail_truncation)",
		originalLength, len(candidate))
	return SanitizeResult{
		CleanQuery:     candidate,
		WasSanitized:   true,
		OriginalLength: originalLength,
		CleanLength:    len(candidate),
		Method:         "tail_truncation",
	}
}

// splitSegments splits text by newlines into non-empty trimmed segments.
func splitSegments(text string) []string {
	lines := strings.Split(text, "\n")
	var segments []string
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if s != "" {
			segments = append(segments, s)
		}
	}
	return segments
}

// trimCandidate strips wrapping quotes and trims to maxQueryLength if needed.
func trimCandidate(candidate string) string {
	candidate = stripWrappingQuotes(candidate)
	if len(candidate) <= maxQueryLength {
		return candidate
	}

	// Try to find a sub-sentence within bounds.
	fragments := sentenceSplitRe.Split(candidate, -1)
	for i := len(fragments) - 1; i >= 0; i-- {
		frag := strings.TrimSpace(stripWrappingQuotes(fragments[i]))
		if len(frag) >= minQueryLength && len(frag) <= maxQueryLength {
			return frag
		}
	}

	// Hard truncate from the end.
	return strings.TrimSpace(candidate[len(candidate)-safeQueryLength:])
}

// stripWrappingQuotes removes surrounding quote characters from a string.
func stripWrappingQuotes(s string) string {
	s = strings.TrimSpace(s)
	for len(s) >= 2 {
		first := s[0]
		last := s[len(s)-1]
		if (first == '\'' || first == '"') && first == last {
			s = strings.TrimSpace(s[1 : len(s)-1])
			if s == "" {
				return ""
			}
			continue
		}
		break
	}
	// Strip leading-only or trailing-only quotes.
	if len(s) > 0 && (s[0] == '\'' || s[0] == '"') {
		s = strings.TrimSpace(s[1:])
	}
	if len(s) > 0 && (s[len(s)-1] == '\'' || s[len(s)-1] == '"') {
		s = strings.TrimSpace(s[:len(s)-1])
	}
	return s
}
