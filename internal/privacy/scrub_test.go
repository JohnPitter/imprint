package privacy

import (
	"testing"
)

func TestStripPrivateData_APIKeys(t *testing.T) {
	input := `config: api_key=abc12345678901234567890`
	result := StripPrivateData(input)
	if result != "config: [REDACTED]" {
		t.Errorf("expected api_key redacted, got %q", result)
	}
}

func TestStripPrivateData_OpenAIKey(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"sk-proj key", "key is sk-proj-abcdefghijklmnopqrstuvwx"},
		{"sk key", "key is sk-abcdefghijklmnopqrstuvwx"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripPrivateData(tt.input)
			if result == tt.input {
				t.Errorf("expected OpenAI key to be redacted, got %q", result)
			}
			if !contains(result, "[REDACTED]") {
				t.Errorf("expected [REDACTED] in output, got %q", result)
			}
		})
	}
}

func TestStripPrivateData_GitHubPAT(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"ghp token", "token: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"},
		{"gho token", "token: gho_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"},
		{"github_pat token", "token: github_pat_1234567890abcdefghijkl_12345678901234567890123456789012345678901234567890123456789"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripPrivateData(tt.input)
			if result == tt.input {
				t.Errorf("expected GitHub PAT to be redacted, got %q", result)
			}
			if !contains(result, "[REDACTED]") {
				t.Errorf("expected [REDACTED] in output, got %q", result)
			}
		})
	}
}

func TestStripPrivateData_SlackTokens(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"xoxb token", "slack: xoxb-1234567890-abcdefghij"},
		{"xoxp token", "slack: xoxp-9876543210-zyxwvutsrq"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripPrivateData(tt.input)
			if result == tt.input {
				t.Errorf("expected Slack token to be redacted, got %q", result)
			}
			if !contains(result, "[REDACTED]") {
				t.Errorf("expected [REDACTED] in output, got %q", result)
			}
		})
	}
}

func TestStripPrivateData_AWSKey(t *testing.T) {
	input := "aws key: AKIAIOSFODNN7EXAMPLE"
	result := StripPrivateData(input)
	if result == input {
		t.Errorf("expected AWS key to be redacted, got %q", result)
	}
	if !contains(result, "[REDACTED]") {
		t.Errorf("expected [REDACTED] in output, got %q", result)
	}
}

func TestStripPrivateData_JWT(t *testing.T) {
	input := "auth: eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abc123def456"
	result := StripPrivateData(input)
	if result == input {
		t.Errorf("expected JWT to be redacted, got %q", result)
	}
	if !contains(result, "[REDACTED]") {
		t.Errorf("expected [REDACTED] in output, got %q", result)
	}
}

func TestStripPrivateData_PrivateTag(t *testing.T) {
	input := "public info <private>secret stuff here</private> more public"
	result := StripPrivateData(input)
	expected := "public info [PRIVATE] more public"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestStripPrivateData_NoSecrets(t *testing.T) {
	inputs := []string{
		"just a normal string",
		"hello world 12345",
		"no secrets here, move along",
		"",
	}
	for _, input := range inputs {
		result := StripPrivateData(input)
		if result != input {
			t.Errorf("expected unchanged %q, got %q", input, result)
		}
	}
}

func TestStripPrivateData_MultipleSecrets(t *testing.T) {
	input := "key1: sk-abcdefghijklmnopqrstuvwx and slack: xoxb-1234567890-abcdefghij"
	result := StripPrivateData(input)
	if contains(result, "sk-") {
		t.Errorf("expected OpenAI key redacted, got %q", result)
	}
	if contains(result, "xoxb-") {
		t.Errorf("expected Slack token redacted, got %q", result)
	}
}

func TestStripFromMap(t *testing.T) {
	data := map[string]any{
		"name":    "safe value",
		"secret":  "key is sk-abcdefghijklmnopqrstuvwx",
		"count":   42,
		"enabled": true,
		"nested": map[string]any{
			"token": "slack: xoxb-1234567890-abcdefghij",
			"safe":  "no secrets",
		},
		"list": []any{
			"normal",
			"has key: AKIAIOSFODNN7EXAMPLE",
		},
	}

	result := StripFromMap(data)

	// Original should be unchanged
	if original, ok := data["secret"].(string); !ok || !contains(original, "sk-") {
		t.Error("original map should not be modified")
	}

	// Top-level string with secret
	if val, ok := result["secret"].(string); !ok || !contains(val, "[REDACTED]") {
		t.Errorf("expected secret redacted, got %q", result["secret"])
	}

	// Safe string passes through
	if val, ok := result["name"].(string); !ok || val != "safe value" {
		t.Errorf("expected safe value unchanged, got %q", result["name"])
	}

	// Non-string types pass through
	if val, ok := result["count"].(int); !ok || val != 42 {
		t.Errorf("expected int unchanged, got %v", result["count"])
	}

	// Nested map
	nested, ok := result["nested"].(map[string]any)
	if !ok {
		t.Fatal("expected nested map")
	}
	if val, ok := nested["token"].(string); !ok || !contains(val, "[REDACTED]") {
		t.Errorf("expected nested token redacted, got %q", nested["token"])
	}
	if val, ok := nested["safe"].(string); !ok || val != "no secrets" {
		t.Errorf("expected nested safe unchanged, got %q", nested["safe"])
	}

	// Slice
	list, ok := result["list"].([]any)
	if !ok {
		t.Fatal("expected slice")
	}
	if val, ok := list[0].(string); !ok || val != "normal" {
		t.Errorf("expected slice[0] unchanged, got %q", list[0])
	}
	if val, ok := list[1].(string); !ok || !contains(val, "[REDACTED]") {
		t.Errorf("expected slice[1] redacted, got %q", list[1])
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
