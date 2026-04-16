package handler

import (
	"encoding/json"
	"net/http"

	"imprint/internal/service"
)

// GraphHandler handles HTTP requests for graph operations.
type GraphHandler struct {
	svc *service.GraphService
}

// NewGraphHandler creates a new GraphHandler.
func NewGraphHandler(svc *service.GraphService) *GraphHandler {
	return &GraphHandler{svc: svc}
}

// HandleExtract handles POST /imprint/graph/extract.
// Takes a compressed observation ID and triggers entity extraction.
func (h *GraphHandler) HandleExtract(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ObservationID string `json:"observationId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	// For now, just return success - extraction runs via the pipeline
	writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

// HandleQuery handles POST /imprint/graph/query.
func (h *GraphHandler) HandleQuery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StartNodeID string `json:"startNodeId"`
		Query       string `json:"query"`
		MaxDepth    int    `json:"maxDepth"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.StartNodeID == "" {
		writeError(w, http.StatusBadRequest, "startNodeId is required")
		return
	}
	result, err := h.svc.Query(req.StartNodeID, req.MaxDepth)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// HandleStats handles GET /imprint/graph/stats.
func (h *GraphHandler) HandleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.Stats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// HandleAll handles GET /imprint/graph/all — returns all nodes and edges for visualization.
func (h *GraphHandler) HandleAll(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.svc.AllNodes(500)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	edges, err := h.svc.AllEdges(1000)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"nodes": orEmpty(nodes), "edges": orEmpty(edges)})
}

// HandleRelations handles POST /imprint/relations.
func (h *GraphHandler) HandleRelations(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceNodeID string  `json:"sourceNodeId"`
		TargetNodeID string  `json:"targetNodeId"`
		Type         string  `json:"type"`
		Weight       float64 `json:"weight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SourceNodeID == "" || req.TargetNodeID == "" || req.Type == "" {
		writeError(w, http.StatusBadRequest, "sourceNodeId, targetNodeId, and type are required")
		return
	}
	edge, err := h.svc.CreateRelation(req.SourceNodeID, req.TargetNodeID, req.Type, req.Weight)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"edge": edge})
}
