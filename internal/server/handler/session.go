package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"imprint/internal/service"
)

// SessionHandler holds HTTP handlers for session endpoints.
type SessionHandler struct {
	svc *service.SessionService
}

// NewSessionHandler creates a new SessionHandler.
func NewSessionHandler(svc *service.SessionService) *SessionHandler {
	return &SessionHandler{svc: svc}
}

// HandleStart handles POST /imprint/session/start.
func (h *SessionHandler) HandleStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"sessionId"`
		Project   string `json:"project"`
		Cwd       string `json:"cwd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	session, context, err := h.svc.Start(req.SessionID, req.Project, req.Cwd)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	contextStr := buildContextXML(context, req.Project)
	writeJSON(w, http.StatusOK, map[string]any{
		"session": session,
		"context": contextStr,
	})
}

// HandleEnd handles POST /imprint/session/end.
func (h *SessionHandler) HandleEnd(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.End(req.SessionID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleList handles GET /imprint/sessions.
func (h *SessionHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")

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

	sessions, err := h.svc.List(project, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Total is independent of pagination — clients use it to render counts
	// without fetching every row.
	total, _ := h.svc.Count(project)

	writeJSON(w, http.StatusOK, map[string]any{
		"sessions": sessions,
		"total":    total,
	})
}
