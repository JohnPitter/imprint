package store

import (
	"database/sql"
	"encoding/json"
	"time"
)

// MarshalJSON converts a value to a JSON string for storing in TEXT columns.
// Returns "[]" on error as a safe fallback for slice-typed columns.
func MarshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// UnmarshalStringSlice parses a JSON TEXT column into []string.
// Returns nil on empty/invalid input.
func UnmarshalStringSlice(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		return nil
	}
	return result
}

// UnmarshalMap parses a JSON TEXT column into map[string]any.
// Returns nil on empty/invalid input.
func UnmarshalMap(s string) map[string]any {
	if s == "" {
		return nil
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		return nil
	}
	return result
}

// NullString converts *string to sql.NullString.
func NullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

// NullTime converts *time.Time to sql.NullString using ISO 8601 format.
func NullTime(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: t.UTC().Format(time.RFC3339), Valid: true}
}

// ParseNullTime converts sql.NullString back to *time.Time.
// Returns nil when the column is NULL or the value cannot be parsed.
func ParseNullTime(ns sql.NullString) *time.Time {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, ns.String)
	if err != nil {
		return nil
	}
	return &t
}

// TimeToString converts time.Time to an ISO 8601 (RFC 3339) string in UTC.
func TimeToString(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// ParseTime parses an ISO 8601 (RFC 3339) string into time.Time.
// Returns the zero value on parse failure.
func ParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
