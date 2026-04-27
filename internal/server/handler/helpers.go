package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"imprint/internal/types"
)

// orEmpty returns the slice as-is if non-nil, or an empty slice for clean JSON serialization.
// Go serializes nil slices as "null" but we want "[]".
func orEmpty(v any) any {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() || (rv.Kind() == reflect.Slice && rv.IsNil()) {
		return []any{}
	}
	return v
}

// writeJSON writes a JSON response with the given status code.
// Marshals first so a serialization failure (e.g. invalid json.RawMessage from
// the DB) returns 500 instead of a silent empty 200 with broken Content-Length.
func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"response serialization failed"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// buildContextXML builds the context XML string from context blocks.
func buildContextXML(blocks []types.ContextBlock, project string) string {
	if len(blocks) == 0 {
		return ""
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "<imprint-context project=%q>\n", project)
	for _, b := range blocks {
		fmt.Fprintf(&sb, "<%s>\n%s\n</%s>\n", b.Type, b.Content, b.Type)
	}
	sb.WriteString("</imprint-context>")
	return sb.String()
}
