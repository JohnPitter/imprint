package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"imprint/internal/privacy"
	"imprint/internal/service"
	"imprint/internal/store"
)

// SearchHandler holds HTTP handlers for search and context endpoints.
type SearchHandler struct {
	searchSvc  *service.SearchService
	contextSvc *service.ContextService
	evalStore  *store.EvalStore // optional; nil-safe
	captureOn  bool
}

// NewSearchHandler creates a new SearchHandler. Eval capture is gated on the
// IMPRINT_EVAL_CAPTURE environment variable so installs that haven't opted in
// pay nothing for it. The flag is read once at construction; flipping it
// requires a server restart, which is fine — capture is a deliberate, audited
// data-collection feature, not a runtime toggle.
func NewSearchHandler(searchSvc *service.SearchService, contextSvc *service.ContextService, evalStore *store.EvalStore) *SearchHandler {
	return &SearchHandler{
		searchSvc:  searchSvc,
		contextSvc: contextSvc,
		evalStore:  evalStore,
		captureOn:  evalCaptureEnabled(),
	}
}

func evalCaptureEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("IMPRINT_EVAL_CAPTURE")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
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

	h.captureSearch(req.Query, results)

	writeJSON(w, http.StatusOK, map[string]any{"results": results, "count": len(results)})
}

// captureSearch records the (query, returned ids) pair into eval_candidates
// when capture is enabled. Best-effort: a failure here never affects the
// search response. The query is run through ScrubAll so the eval table never
// holds raw secrets or injection payloads even on opt-in installs.
func (h *SearchHandler) captureSearch(query string, results any) {
	if !h.captureOn || h.evalStore == nil {
		return
	}
	ids := extractResultIDs(results)
	_ = h.evalStore.Append(store.EvalCandidate{
		Source:      "http",
		Operation:   "search",
		QueryText:   privacy.ScrubAll(query),
		ReturnedIDs: ids,
		ResultCount: len(ids),
	})
}

// extractResultIDs pulls the `id` field out of a slice-of-objects results
// payload. The search service returns []service.SearchResult (or similar)
// but we can't depend on its exact type here without a circular import, so
// we round-trip through JSON. Capture is best-effort; if extraction fails
// the candidate is just skipped.
func extractResultIDs(results any) []string {
	raw, err := json.Marshal(results)
	if err != nil {
		return nil
	}
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if id, ok := r["id"].(string); ok && id != "" {
			out = append(out, id)
		}
	}
	return out
}

// HandleEvalExport handles GET /imprint/eval/export.
// Returns recent eval candidates as NDJSON (one JSON object per line) so the
// stream is friendly to large captures and to standard unix tooling.
func (h *SearchHandler) HandleEvalExport(w http.ResponseWriter, r *http.Request) {
	if h.evalStore == nil {
		writeError(w, http.StatusServiceUnavailable, "eval capture disabled")
		return
	}
	limit := 1000
	if v := r.URL.Query().Get("limit"); v != "" {
		fmt.Sscanf(v, "%d", &limit)
	}
	rows, err := h.evalStore.List(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	enc := json.NewEncoder(w)
	for _, c := range rows {
		_ = enc.Encode(c)
	}
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
