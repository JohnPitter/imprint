package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"imprint/internal/service"
)

// SearchHandler holds HTTP handlers for search and context endpoints.
type SearchHandler struct {
	searchSvc  *service.SearchService
	contextSvc *service.ContextService
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(searchSvc *service.SearchService, contextSvc *service.ContextService) *SearchHandler {
	return &SearchHandler{searchSvc: searchSvc, contextSvc: contextSvc}
}

// HandleSearch handles POST /imprint/search.
func (h *SearchHandler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Query == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}

	results, err := h.searchSvc.Search(req.Query, req.Limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": results, "count": len(results)})
}

// HandleContext handles POST /imprint/context.
func (h *SearchHandler) HandleContext(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"sessionId"`
		Project   string `json:"project"`
		Budget    int    `json:"budget"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	blocks, err := h.contextSvc.BuildContext(req.SessionID, req.Project, req.Budget)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	contextStr := buildContextXML(blocks, req.Project)
	writeJSON(w, http.StatusOK, map[string]any{"context": contextStr, "blocks": blocks})
}

// HandleEnrich handles POST /imprint/enrich.
func (h *SearchHandler) HandleEnrich(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string   `json:"sessionId"`
		Files     []string `json:"files"`
		Terms     []string `json:"terms"`
		ToolName  string   `json:"toolName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	query := strings.TrimSpace(strings.Join(append(req.Files, req.Terms...), " "))
	if query == "" {
		writeJSON(w, http.StatusOK, map[string]any{"context": ""})
		return
	}

	results, err := h.searchSvc.Search(query, 5)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"context": ""})
		return
	}

	var sb strings.Builder
	for _, r := range results {
		narrative := ""
		if r.Narrative != nil {
			narrative = *r.Narrative
		}
		fmt.Fprintf(&sb, "- [%s] %s: %s\n", r.Type, r.Title, narrative)
	}

	writeJSON(w, http.StatusOK, map[string]any{"context": sb.String(), "count": len(results)})
}
