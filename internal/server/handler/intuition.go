package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"imprint/internal/service"
)

// IntuitionHandler serves the rooted-memory (intuition) inspection screen and
// its manual controls (invariant 11: always inspectable, always removable).
type IntuitionHandler struct {
	svc *service.IntuitionService
}

func NewIntuitionHandler(svc *service.IntuitionService) *IntuitionHandler {
	return &IntuitionHandler{svc: svc}
}

// HandleList serves GET /imprint/intuitions?project=&limit= — every intuition
// (any status) with force, evidence, status and contradiction count.
func (h *IntuitionHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	limit := 200
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	items, err := h.svc.ListAll(project, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"intuitions": orEmpty(items), "total": len(items)})
}

// HandleContradictions serves GET /imprint/intuitions/contradictions?id= — the
// audit trail behind an intuition's auto-weakening.
func (h *IntuitionHandler) HandleContradictions(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	items, err := h.svc.Contradictions(id, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"contradictions": orEmpty(items), "total": len(items)})
}

// HandleDemote serves POST /imprint/intuitions/demote {id} — manual demotion.
func (h *IntuitionHandler) HandleDemote(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if err := h.svc.Demote(req.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": req.ID, "status": "demoted"})
}

// HandleDelete serves POST /imprint/intuitions/delete {id} — manual removal (A5).
func (h *IntuitionHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if err := h.svc.Delete(req.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": req.ID, "status": "deleted"})
}

// HandleDetect serves POST /imprint/intuitions/detect {project} — runs the
// convergence detector now (also runs periodically in the background).
func (h *IntuitionHandler) HandleDetect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Project string `json:"project"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Project == "" {
		writeError(w, http.StatusBadRequest, "project is required")
		return
	}
	created, err := h.svc.DetectAndRoot(context.Background(), req.Project)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rooted": orEmpty(created), "count": len(created)})
}
