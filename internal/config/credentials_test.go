package config

import "testing"

func TestExtractOAuthToken(t *testing.T) {
	now := int64(1_700_000_000)
	future := (now + 3600) * 1000 // ms
	past := (now - 3600) * 1000

	cases := []struct {
		name string
		data string
		want string
	}{
		{
			name: "valid token",
			data: `{"claudeAiOauth":{"accessToken":"abc123","expiresAt":` + itoa(future) + `}}`,
			want: "abc123",
		},
		{
			name: "expired token",
			data: `{"claudeAiOauth":{"accessToken":"abc123","expiresAt":` + itoa(past) + `}}`,
			want: "",
		},
		{
			name: "no expiresAt — assume valid",
			data: `{"claudeAiOauth":{"accessToken":"abc123"}}`,
			want: "abc123",
		},
		{
			name: "empty token",
			data: `{"claudeAiOauth":{"accessToken":"","expiresAt":` + itoa(future) + `}}`,
			want: "",
		},
		{
			name: "no oauth field",
			data: `{"other":"thing"}`,
			want: "",
		},
		{
			name: "malformed json",
			data: `not json`,
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractOAuthToken([]byte(tc.data), now)
			if got != tc.want {
				t.Errorf("extractOAuthToken: got %q, want %q", got, tc.want)
			}
		})
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
