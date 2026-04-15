package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"imprint/internal/types"
)

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
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
