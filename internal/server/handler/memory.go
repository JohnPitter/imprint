package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"imprint/internal/service"
	"imprint/internal/types"
)

// MemoryHandler holds HTTP handlers for memory endpoints.
type MemoryHandler struct {
	svc *service.RememberService
}

// NewMemoryHandler creates a new MemoryHandler.
func NewMemoryHandler(svc *service.RememberService) *MemoryHandler {
	return &MemoryHandler{svc: svc}
}

// HandleRemember handles POST /imprint/remember.
func (h *MemoryHandler) HandleRemember(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type     types.MemoryType `json:"type"`
		Title    string           `json:"title"`
		Content  string           `json:"content"`
		Concepts []string         `json:"concepts"`
		Files    []string         `json:"files"`
		Strength int              `json:"strength"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	mem, err := h.svc.Remember(req.Type, req.Title, req.Content, req.Concepts, req.Files, req.Strength)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"memory": mem,
	})
}

// HandleForget handles POST /imprint/forget.
func (h *MemoryHandler) HandleForget(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.Forget(req.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleEvolve handles POST /imprint/evolve. Body fields content / title /
// type / strength are all optional; missing ones inherit from the previous
// version.
func (h *MemoryHandler) HandleEvolve(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       string `json:"id"`
		Content  string `json:"content"`
		Title    string `json:"title"`
		Type     string `json:"type"`
		Strength int    `json:"strength"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	mem, err := h.svc.Evolve(req.ID, service.EvolveInput{
		Content:  req.Content,
		Title:    req.Title,
		Type:     req.Type,
		Strength: req.Strength,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"memory": mem,
	})
}

// HandleList handles GET /imprint/memories.
func (h *MemoryHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	memType := r.URL.Query().Get("type")

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	memories, err := h.svc.List(memType, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	total, _ := h.svc.Count()

	writeJSON(w, http.StatusOK, map[string]any{
		"memories": memories,
		"total":    total,
	})
}

// HandleGraph handles GET /imprint/memories/graph?topN=200&minShared=1.
// Returns memory-centric graph nodes + edges for the Graph tab's "memories"
// view.
func (h *MemoryHandler) HandleGraph(w http.ResponseWriter, r *http.Request) {
	topN := 200
	minShared := 1
	if v := r.URL.Query().Get("topN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			topN = n
		}
	}
	if v := r.URL.Query().Get("minShared"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			minShared = n
		}
	}
	nodes, edges, err := h.svc.MemoryGraph(topN, minShared)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"nodes": nodes,
		"edges": edges,
	})
}

// HandleHistory handles GET /imprint/memories/history?id=mem_xxx. Returns
// every version of the memory in oldest-first order so the UI can render a
// timeline.
func (h *MemoryHandler) HandleHistory(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id query parameter is required")
		return
	}
	versions, err := h.svc.History(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": versions})
}

// HandleConcepts handles GET /imprint/memories/concepts. Returns the top
// concepts aggregated server-side from all latest memories.
func (h *MemoryHandler) HandleConcepts(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	concepts, err := h.svc.TopConcepts(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"concepts": concepts})
}
