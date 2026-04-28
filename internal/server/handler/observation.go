package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"imprint/internal/service"
	"imprint/internal/types"
)

// ObservationHandler holds HTTP handlers for observation endpoints.
type ObservationHandler struct {
	svc *service.ObserveService
}

// NewObservationHandler creates a new ObservationHandler.
func NewObservationHandler(svc *service.ObserveService) *ObservationHandler {
	return &ObservationHandler{svc: svc}
}

// HandleObserve handles POST /imprint/observe.
func (h *ObservationHandler) HandleObserve(w http.ResponseWriter, r *http.Request) {
	var payload types.HookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	obs, err := h.svc.Observe(&payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if obs == nil {
		// Duplicate or rate-limited; acknowledge silently.
		writeJSON(w, http.StatusOK, map[string]string{"status": "skipped"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"observation": obs,
	})
}

// HandleCount handles GET /imprint/observations/count.
// Returns the total number of raw observations across every session.
func (h *ObservationHandler) HandleCount(w http.ResponseWriter, r *http.Request) {
	total, err := h.svc.CountAll()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"total": total})
}

// HandleList handles GET /imprint/observations.
func (h *ObservationHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "sessionId query parameter is required")
		return
	}

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

	// Prefer compressed observations; fall back to raw if none exist.
	compressed, err := h.svc.ListCompressed(sessionID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if len(compressed) > 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"observations": compressed,
			"type":         "compressed",
		})
		return
	}

	raw, err := h.svc.ListRaw(sessionID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"observations": raw,
		"type":         "raw",
	})
}
