package handler

import (
	"encoding/json"
	"net/http"

	"imprint/internal/service"
)

// MemoryAdminHandler serves lazy on-demand injection (Phase 2 economy lever) and
// the A5 controls over one's own memory: export, purge a repo, full reset.
type MemoryAdminHandler struct {
	contextSvc *service.ContextService
	adminSvc   *service.MemoryAdminService
	lazyMax    int
}

func NewMemoryAdminHandler(contextSvc *service.ContextService, adminSvc *service.MemoryAdminService, lazyMax int) *MemoryAdminHandler {
	if lazyMax <= 0 {
		lazyMax = 5
	}
	return &MemoryAdminHandler{contextSvc: contextSvc, adminSvc: adminSvc, lazyMax: lazyMax}
}

// HandleLazyInject serves POST /imprint/inject/lazy. Given the files/concepts a
// turn is actually touching, it returns just the refined memories that match —
// memory the turn didn't need is memory not injected.
func (h *MemoryAdminHandler) HandleLazyInject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string   `json:"sessionId"`
		Project   string   `json:"project"`
		Files     []string `json:"files"`
		Concepts  []string `json:"concepts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	blocks, err := h.contextSvc.LazyContext(req.SessionID, req.Project, req.Files, req.Concepts, h.lazyMax)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"context": buildContextXML(blocks, req.Project),
		"blocks":  orEmpty(blocks),
		"count":   len(blocks),
	})
}

// HandleExport serves GET /imprint/memory/export?project= — a portable snapshot.
func (h *MemoryAdminHandler) HandleExport(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	if project == "" {
		writeError(w, http.StatusBadRequest, "project is required")
		return
	}
	exp, err := h.adminSvc.Export(project)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, exp)
}

// HandlePurge serves POST /imprint/memory/purge {project} — delete a repo's
// memory across every layer and the indexes (A5).
func (h *MemoryAdminHandler) HandlePurge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Project string `json:"project"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Project == "" {
		writeError(w, http.StatusBadRequest, "project is required")
		return
	}
	counts, err := h.adminSvc.PurgeProject(req.Project)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project": req.Project, "deleted": counts})
}

// HandleReset serves POST /imprint/memory/reset {confirm:true} — wipe everything
// back to cold start. Requires explicit confirmation.
func (h *MemoryAdminHandler) HandleReset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Confirm bool `json:"confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !req.Confirm {
		writeError(w, http.StatusBadRequest, "confirm:true is required")
		return
	}
	if err := h.adminSvc.ResetAll(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "reset", "note": "memory is back to cold start"})
}
