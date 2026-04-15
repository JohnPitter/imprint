package privacy

import (
	"fmt"
	"regexp"
)

// secretPatterns matches sensitive data like API keys, tokens, and credentials.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_-]?key|apikey)\s*[:=]\s*['"]?[\w\-]{20,}['"]?`),
	regexp.MustCompile(`(?i)(secret|password|passwd|pwd)\s*[:=]\s*['"]?[\w\-]{8,}['"]?`),
	regexp.MustCompile(`(?i)(token|auth[_-]?token|access[_-]?token|bearer)\s*[:=]\s*['"]?[\w\-\.]{20,}['"]?`),
	regexp.MustCompile(`sk-proj-[\w\-]{20,}`),          // OpenAI project keys
	regexp.MustCompile(`sk-[\w]{20,}`),                 // OpenAI keys
	regexp.MustCompile(`ghp_[\w]{36}`),                 // GitHub PAT
	regexp.MustCompile(`gho_[\w]{36}`),                 // GitHub OAuth
	regexp.MustCompile(`github_pat_[\w]{22}_[\w]{59}`), // GitHub fine-grained PAT
	regexp.MustCompile(`xoxb-[\w\-]+`),                 // Slack bot token
	regexp.MustCompile(`xoxp-[\w\-]+`),                 // Slack user token
	regexp.MustCompile(`AKIA[\w]{16}`),                 // AWS access key
	regexp.MustCompile(`AIza[\w\-]{35}`),               // Google API key
	regexp.MustCompile(`eyJ[\w\-]*\.eyJ[\w\-]*\.[\w\-]*`), // JWT tokens
	regexp.MustCompile(`npm_[\w]{36}`),                 // npm tokens
	regexp.MustCompile(`glpat-[\w\-]{20,}`),            // GitLab PAT
	regexp.MustCompile(`dop_v1_[\w]{64}`),              // DigitalOcean PAT
}

// privateTagPattern matches <private>...</private> XML tags.
var privateTagPattern = regexp.MustCompile(`(?s)<private>.*?</private>`)

// StripPrivateData removes all secret patterns and private tags from text.
func StripPrivateData(text string) string {
	result := text
	for _, pattern := range secretPatterns {
		result = pattern.ReplaceAllString(result, "[REDACTED]")
	}
	result = privateTagPattern.ReplaceAllString(result, "[PRIVATE]")
	return result
}

// StripFromMap recursively strips secrets from all string values in a map.
// Returns a new map; the original is not modified.
func StripFromMap(data map[string]any) map[string]any {
	out := make(map[string]any, len(data))
	for k, v := range data {
		out[k] = stripValue(v)
	}
	return out
}

// stripValue recursively processes a value, stripping secrets from strings
// and descending into maps and slices.
func stripValue(v any) any {
	switch val := v.(type) {
	case string:
		return StripPrivateData(val)
	case map[string]any:
		return StripFromMap(val)
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = stripValue(item)
		}
		return result
	case fmt.Stringer:
		return StripPrivateData(val.String())
	default:
		return v
	}
}
