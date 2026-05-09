package extract

import (
	"reflect"
	"testing"
)

func TestExtractFiles(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"absolute path", "Edited /tmp/foo/bar.go yesterday", []string{"/tmp/foo/bar.go"}},
		{"relative path", "see ./internal/llm/anthropic.go for details", []string{"./internal/llm/anthropic.go"}},
		{"backtick fenced", "Look at `src/main.go` here", []string(nil)},
		{"in code block", "```\nfile.go\n```\nplain.txt", []string(nil)},
		{"multiple", "files: foo/bar.go, baz/qux.ts", []string{"baz/qux.ts", "foo/bar.go"}},
		{"unknown ext", "weird/path.xyzzy ignored", []string(nil)},
		{"url not file", "open https://example.com/foo/bar.go", []string(nil)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Extract("Read", tc.in, "").Files
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("files: got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExtractURLs(t *testing.T) {
	got := Extract("Bash", "see https://example.com/page and http://localhost:3000/api.", "").URLs
	want := []string{"http://localhost:3000/api", "https://example.com/page"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("urls: got %v, want %v", got, want)
	}
}

func TestExtractGitRefs(t *testing.T) {
	in := "commit a1b2c3d fixes #123 and 1234567 (numeric, ignored)"
	got := Extract("Bash", in, "").GitRefs
	want := []string{"#123", "a1b2c3d"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("gitrefs: got %v, want %v", got, want)
	}
}

func TestExtractErrors(t *testing.T) {
	in := "got TypeError: foo is undefined and panic: nil pointer"
	got := Extract("Bash", in, "").Errors
	if len(got) < 2 {
		t.Errorf("errors: expected at least 2 distinct error markers, got %v", got)
	}
}

func TestExtractConcepts(t *testing.T) {
	in := "the UserAccount class talks to PaymentProvider via ApiClient"
	got := Extract("Read", in, "").Concepts
	want := map[string]bool{"UserAccount": true, "PaymentProvider": true, "ApiClient": true}
	for _, c := range got {
		delete(want, c)
	}
	if len(want) > 0 {
		t.Errorf("concepts: missing %v from result %v", want, got)
	}
}

func TestExtractIsDeterministic(t *testing.T) {
	in := "open foo.go, /tmp/bar.ts, and see UserService class. https://example.com/x and #42"
	a := Extract("Read", in, "")
	b := Extract("Read", in, "")
	if !reflect.DeepEqual(a, b) {
		t.Errorf("non-deterministic: a=%+v, b=%+v", a, b)
	}
}

func TestStripCodeFences(t *testing.T) {
	in := "before ```\ncode\n``` after `inline` end"
	got := stripCodeFences(in)
	if got == in {
		t.Errorf("expected code blocks to be replaced, got %q", got)
	}
	if len(got) != len(in) {
		t.Errorf("length mismatch: in=%d out=%d", len(in), len(got))
	}
}
