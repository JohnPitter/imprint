package handler

import (
	"encoding/json"
	"net/http"

	"imprint/internal/service"
)

// RecallHandler exposes the natural-language recall endpoint.
type RecallHandler struct {
	svc *service.RecallService
}

// NewRecallHandler wires a RecallHandler.
func NewRecallHandler(svc *service.RecallService) *RecallHandler {
	return &RecallHandler{svc: svc}
}

// HandleRecall handles POST /imprint/recall. Body: { query: string, limit?: int }.
// Returns { answer, sources, used, skipped }.
func (h *RecallHandler) HandleRecall(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	res, err := h.svc.Recall(r.Context(), req.Query, req.Limit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}
