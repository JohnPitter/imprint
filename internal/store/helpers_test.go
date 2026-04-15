package store

import (
	"database/sql"
	"testing"
	"time"
)

func TestMarshalJSON_StringSlice(t *testing.T) {
	input := []string{"a", "b", "c"}
	got := MarshalJSON(input)
	want := `["a","b","c"]`
	if got != want {
		t.Errorf("MarshalJSON(%v) = %q, want %q", input, got, want)
	}
}

func TestMarshalJSON_Map(t *testing.T) {
	input := map[string]any{"key": "value", "num": 42.0}
	got := MarshalJSON(input)

	// Verify roundtrip rather than exact string (map key order is non-deterministic).
	result := UnmarshalMap(got)
	if result == nil {
		t.Fatalf("MarshalJSON produced invalid JSON: %q", got)
	}
	if result["key"] != "value" {
		t.Errorf("roundtrip key = %v, want %q", result["key"], "value")
	}
	if result["num"] != 42.0 {
		t.Errorf("roundtrip num = %v, want 42.0", result["num"])
	}
}

func TestUnmarshalStringSlice(t *testing.T) {
	original := []string{"x", "y", "z"}
	jsonStr := MarshalJSON(original)
	got := UnmarshalStringSlice(jsonStr)

	if len(got) != len(original) {
		t.Fatalf("len = %d, want %d", len(got), len(original))
	}
	for i, v := range got {
		if v != original[i] {
			t.Errorf("index %d = %q, want %q", i, v, original[i])
		}
	}
}

func TestUnmarshalStringSlice_Empty(t *testing.T) {
	got := UnmarshalStringSlice("")
	if got != nil {
		t.Errorf("UnmarshalStringSlice(\"\") = %v, want nil", got)
	}
}

func TestUnmarshalStringSlice_Invalid(t *testing.T) {
	got := UnmarshalStringSlice("{not valid json")
	if got != nil {
		t.Errorf("UnmarshalStringSlice(invalid) = %v, want nil", got)
	}
}

func TestUnmarshalMap(t *testing.T) {
	original := map[string]any{"foo": "bar"}
	jsonStr := MarshalJSON(original)
	got := UnmarshalMap(jsonStr)

	if got == nil {
		t.Fatal("UnmarshalMap returned nil")
	}
	if got["foo"] != "bar" {
		t.Errorf("got[\"foo\"] = %v, want \"bar\"", got["foo"])
	}
}

func TestNullString(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		ns := NullString(nil)
		if ns.Valid {
			t.Error("NullString(nil).Valid = true, want false")
		}
	})

	t.Run("non-nil", func(t *testing.T) {
		s := "hello"
		ns := NullString(&s)
		if !ns.Valid {
			t.Error("NullString(&s).Valid = false, want true")
		}
		if ns.String != "hello" {
			t.Errorf("NullString(&s).String = %q, want %q", ns.String, "hello")
		}
	})
}

func TestNullTime(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		ns := NullTime(nil)
		if ns.Valid {
			t.Error("NullTime(nil).Valid = true, want false")
		}
	})

	t.Run("non-nil", func(t *testing.T) {
		ts := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		ns := NullTime(&ts)
		if !ns.Valid {
			t.Error("NullTime(&ts).Valid = false, want true")
		}
		if ns.String != "2025-06-15T12:00:00Z" {
			t.Errorf("NullTime(&ts).String = %q, want %q", ns.String, "2025-06-15T12:00:00Z")
		}
	})
}

func TestParseNullTime(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		ns := sql.NullString{String: "2025-06-15T12:00:00Z", Valid: true}
		got := ParseNullTime(ns)
		if got == nil {
			t.Fatal("ParseNullTime returned nil for valid input")
		}
		want := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Errorf("ParseNullTime = %v, want %v", *got, want)
		}
	})

	t.Run("invalid_null", func(t *testing.T) {
		ns := sql.NullString{Valid: false}
		got := ParseNullTime(ns)
		if got != nil {
			t.Errorf("ParseNullTime(null) = %v, want nil", *got)
		}
	})

	t.Run("invalid_format", func(t *testing.T) {
		ns := sql.NullString{String: "not-a-date", Valid: true}
		got := ParseNullTime(ns)
		if got != nil {
			t.Errorf("ParseNullTime(bad format) = %v, want nil", *got)
		}
	})

	t.Run("empty_string", func(t *testing.T) {
		ns := sql.NullString{String: "", Valid: true}
		got := ParseNullTime(ns)
		if got != nil {
			t.Errorf("ParseNullTime(empty) = %v, want nil", *got)
		}
	})
}

func TestTimeToString(t *testing.T) {
	ts := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	got := TimeToString(ts)
	want := "2025-01-02T03:04:05Z"
	if got != want {
		t.Errorf("TimeToString = %q, want %q", got, want)
	}
}

func TestParseTime(t *testing.T) {
	t.Run("roundtrip", func(t *testing.T) {
		original := time.Date(2025, 3, 10, 15, 30, 0, 0, time.UTC)
		s := TimeToString(original)
		got := ParseTime(s)
		if !got.Equal(original) {
			t.Errorf("ParseTime(TimeToString(%v)) = %v", original, got)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		got := ParseTime("garbage")
		if !got.IsZero() {
			t.Errorf("ParseTime(invalid) = %v, want zero time", got)
		}
	})
}
