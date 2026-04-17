package hooks

import (
	"io"
	"os"
	"strings"
	"testing"
)

// withStdin temporarily replaces os.Stdin with content, runs fn, and restores.
func withStdin(t *testing.T, content string, fn func()) {
	t.Helper()
	orig := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdin = r
	go func() {
		_, _ = io.WriteString(w, content)
		_ = w.Close()
	}()
	defer func() { os.Stdin = orig }()
	fn()
}

func TestReadStdin_ValidJSON(t *testing.T) {
	withStdin(t, `{"event":"test","value":42}`, func() {
		got, err := ReadStdin()
		if err != nil {
			t.Fatalf("ReadStdin: %v", err)
		}
		if got["event"] != "test" {
			t.Errorf("event = %v", got["event"])
		}
		if got["value"].(float64) != 42 {
			t.Errorf("value = %v", got["value"])
		}
	})
}

func TestReadStdin_EmptyInput(t *testing.T) {
	withStdin(t, "", func() {
		_, err := ReadStdin()
		if err == nil {
			t.Fatal("expected error on empty stdin")
		}
		if !strings.Contains(err.Error(), "empty") {
			t.Errorf("error message: %v", err)
		}
	})
}

func TestReadStdin_InvalidJSON(t *testing.T) {
	withStdin(t, `{not valid`, func() {
		_, err := ReadStdin()
		if err == nil {
			t.Fatal("expected parse error")
		}
		if !strings.Contains(err.Error(), "parse") {
			t.Errorf("error should mention parse: %v", err)
		}
	})
}
