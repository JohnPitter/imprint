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

// HandleEvolve handles POST /imprint/evolve.
func (h *MemoryHandler) HandleEvolve(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       string `json:"id"`
		Content  string `json:"content"`
		Strength int    `json:"strength"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	mem, err := h.svc.Evolve(req.ID, req.Content, req.Strength)
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

	writeJSON(w, http.StatusOK, map[string]any{
		"memories": memories,
	})
}
