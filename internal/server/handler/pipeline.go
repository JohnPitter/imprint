package handler

import (
	"encoding/json"
	"net/http"

	"imprint/internal/service"
)

// PipelineHandler handles summarization and consolidation endpoints.
type PipelineHandler struct {
	svc *service.PipelineService
}

// NewPipelineHandler creates a new PipelineHandler.
func NewPipelineHandler(svc *service.PipelineService) *PipelineHandler {
	return &PipelineHandler{svc: svc}
}

// HandleSummarize handles POST /imprint/summarize.
func (h *PipelineHandler) HandleSummarize(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SessionID == "" {
		writeError(w, http.StatusBadRequest, "sessionId is required")
		return
	}

	summary, err := h.svc.Summarize(r.Context(), req.SessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"summary": summary})
}

// HandleConsolidatePipeline handles POST /imprint/consolidate-pipeline.
func (h *PipelineHandler) HandleConsolidatePipeline(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SessionID == "" {
		writeError(w, http.StatusBadRequest, "sessionId is required")
		return
	}

	created, err := h.svc.Consolidate(r.Context(), req.SessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"memoriesCreated": created})
}

// HandleFullPipeline handles POST /imprint/consolidate (alias for full pipeline).
func (h *PipelineHandler) HandleFullPipeline(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SessionID == "" {
		writeError(w, http.StatusBadRequest, "sessionId is required")
		return
	}

	if err := h.svc.RunFullPipeline(r.Context(), req.SessionID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
