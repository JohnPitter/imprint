package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"imprint/internal/service"
)

// AdvancedHandler handles HTTP requests for signals, checkpoints, sentinels,
// sketches, crystals, lessons, insights, facets, audit, and governance.
type AdvancedHandler struct {
	svc *service.AdvancedService
}

// NewAdvancedHandler creates a new AdvancedHandler.
func NewAdvancedHandler(svc *service.AdvancedService) *AdvancedHandler {
	return &AdvancedHandler{svc: svc}
}

// queryInt extracts an integer query parameter with a default value.
func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

// ---------------------------------------------------------------------------
// Signals
// ---------------------------------------------------------------------------

// HandleSendSignal handles POST /signals/send.
func (h *AdvancedHandler) HandleSendSignal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From    string `json:"from"`
		To      string `json:"to"`
		Content string `json:"content"`
		Type    string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	sig, err := h.svc.SendSignal(req.From, req.To, req.Content, req.Type)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, sig)
}

// HandleListSignals handles GET /signals.
func (h *AdvancedHandler) HandleListSignals(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agentId")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agentId query parameter is required")
		return
	}
	limit := queryInt(r, "limit", 50)

	signals, err := h.svc.ListSignals(agentID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"signals": orEmpty(signals)})
}

// ---------------------------------------------------------------------------
// Checkpoints
// ---------------------------------------------------------------------------

// HandleCreateCheckpoint handles POST /checkpoints.
func (h *AdvancedHandler) HandleCreateCheckpoint(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		Type        string  `json:"type"`
		ActionID    *string `json:"actionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	cp, err := h.svc.CreateCheckpoint(req.Name, req.Description, req.Type, req.ActionID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, cp)
}

// HandleResolveCheckpoint handles POST /checkpoints/resolve.
func (h *AdvancedHandler) HandleResolveCheckpoint(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID         string `json:"id"`
		ResolvedBy string `json:"resolvedBy"`
		Result     string `json:"result"`
		Status     string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.ResolveCheckpoint(req.ID, req.ResolvedBy, req.Result, req.Status); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}

// HandleListCheckpoints handles GET /checkpoints.
func (h *AdvancedHandler) HandleListCheckpoints(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limit := queryInt(r, "limit", 50)

	cps, err := h.svc.ListCheckpoints(status, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"checkpoints": orEmpty(cps)})
}

// ---------------------------------------------------------------------------
// Sentinels
// ---------------------------------------------------------------------------

// HandleCreateSentinel handles POST /sentinels.
func (h *AdvancedHandler) HandleCreateSentinel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string         `json:"name"`
		Type   string         `json:"type"`
		Config map[string]any `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	sen, err := h.svc.CreateSentinel(req.Name, req.Type, req.Config)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, sen)
}

// HandleListSentinels handles GET /sentinels.
func (h *AdvancedHandler) HandleListSentinels(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limit := queryInt(r, "limit", 50)

	sents, err := h.svc.ListSentinels(status, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sentinels": orEmpty(sents)})
}

// HandleTriggerSentinel handles POST /sentinels/trigger.
func (h *AdvancedHandler) HandleTriggerSentinel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string `json:"id"`
		Result string `json:"result"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.TriggerSentinel(req.ID, req.Result); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "triggered"})
}

// HandleCheckSentinel handles POST /sentinels/check.
func (h *AdvancedHandler) HandleCheckSentinel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	sen, err := h.svc.CheckSentinel(req.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sen)
}

// HandleCancelSentinel handles POST /sentinels/cancel.
func (h *AdvancedHandler) HandleCancelSentinel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.CancelSentinel(req.ID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// ---------------------------------------------------------------------------
// Sketches
// ---------------------------------------------------------------------------

// HandleCreateSketch handles POST /sketches.
func (h *AdvancedHandler) HandleCreateSketch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title          string  `json:"title"`
		Description    string  `json:"description"`
		Project        *string `json:"project"`
		ExpiresInHours int     `json:"expiresInHours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	sk, err := h.svc.CreateSketch(req.Title, req.Description, req.Project, req.ExpiresInHours)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, sk)
}

// HandleListSketches handles GET /sketches.
func (h *AdvancedHandler) HandleListSketches(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limit := queryInt(r, "limit", 50)

	sketches, err := h.svc.ListSketches(status, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sketches": orEmpty(sketches)})
}

// HandleAddToSketch handles POST /sketches/add.
func (h *AdvancedHandler) HandleAddToSketch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SketchID string `json:"sketchId"`
		ActionID string `json:"actionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.AddToSketch(req.SketchID, req.ActionID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "added"})
}

// HandlePromoteSketch handles POST /sketches/promote.
func (h *AdvancedHandler) HandlePromoteSketch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.PromoteSketch(req.ID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "promoted"})
}

// HandleDiscardSketch handles POST /sketches/discard.
func (h *AdvancedHandler) HandleDiscardSketch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.DiscardSketch(req.ID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "discarded"})
}

// HandleGarbageCollectSketches handles POST /sketches/gc.
func (h *AdvancedHandler) HandleGarbageCollectSketches(w http.ResponseWriter, r *http.Request) {
	count, err := h.svc.GarbageCollectSketches()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"removed": count})
}

// ---------------------------------------------------------------------------
// Lessons
// ---------------------------------------------------------------------------

// HandleCreateLesson handles POST /lessons.
func (h *AdvancedHandler) HandleCreateLesson(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string   `json:"content"`
		Context string   `json:"context"`
		Source  string   `json:"source"`
		Project *string  `json:"project"`
		Tags    []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	lesson, err := h.svc.CreateLesson(req.Content, req.Context, req.Source, req.Project, req.Tags)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, lesson)
}

// HandleListLessons handles GET /lessons.
func (h *AdvancedHandler) HandleListLessons(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)

	lessons, err := h.svc.ListLessons(project, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"lessons": orEmpty(lessons)})
}

// HandleSearchLessons handles POST /lessons/search.
func (h *AdvancedHandler) HandleSearchLessons(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	lessons, err := h.svc.SearchLessons(req.Query, req.Limit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"lessons": orEmpty(lessons)})
}

// HandleStrengthenLesson handles POST /lessons/strengthen.
func (h *AdvancedHandler) HandleStrengthenLesson(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.StrengthenLesson(req.ID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "strengthened"})
}

// ---------------------------------------------------------------------------
// Insights
// ---------------------------------------------------------------------------

// HandleListInsights handles GET /insights.
func (h *AdvancedHandler) HandleListInsights(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)

	insights, err := h.svc.ListInsights(project, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"insights": orEmpty(insights)})
}

// HandleSearchInsights handles POST /insights/search.
func (h *AdvancedHandler) HandleSearchInsights(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	insights, err := h.svc.SearchInsights(req.Query, req.Limit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"insights": orEmpty(insights)})
}

// ---------------------------------------------------------------------------
// Facets
// ---------------------------------------------------------------------------

// HandleCreateFacet handles POST /facets.
func (h *AdvancedHandler) HandleCreateFacet(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TargetID   string `json:"targetId"`
		TargetType string `json:"targetType"`
		Dimension  string `json:"dimension"`
		Value      string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	facet, err := h.svc.CreateFacet(req.TargetID, req.TargetType, req.Dimension, req.Value)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, facet)
}

// HandleGetFacets handles GET /facets.
func (h *AdvancedHandler) HandleGetFacets(w http.ResponseWriter, r *http.Request) {
	targetID := r.URL.Query().Get("targetId")
	targetType := r.URL.Query().Get("targetType")
	if targetID == "" || targetType == "" {
		writeError(w, http.StatusBadRequest, "targetId and targetType query parameters are required")
		return
	}

	facets, err := h.svc.GetFacets(targetID, targetType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"facets": orEmpty(facets)})
}

// HandleRemoveFacet handles POST /facets/remove.
func (h *AdvancedHandler) HandleRemoveFacet(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.RemoveFacet(req.ID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// HandleQueryFacets handles POST /facets/query.
func (h *AdvancedHandler) HandleQueryFacets(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Dimension string `json:"dimension"`
		Value     string `json:"value"`
		Limit     int    `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	facets, err := h.svc.QueryFacets(req.Dimension, req.Value, req.Limit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"facets": orEmpty(facets)})
}

// HandleFacetStats handles GET /facets/stats.
func (h *AdvancedHandler) HandleFacetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.FacetStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// ---------------------------------------------------------------------------
// Audit
// ---------------------------------------------------------------------------

// HandleListAudit handles GET /audit.
func (h *AdvancedHandler) HandleListAudit(w http.ResponseWriter, r *http.Request) {
	action := r.URL.Query().Get("action")
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)

	entries, err := h.svc.ListAudit(action, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": orEmpty(entries)})
}

// ---------------------------------------------------------------------------
// Governance
// ---------------------------------------------------------------------------

// HandleGovernanceDeleteMemory handles DELETE /governance/memories.
func (h *AdvancedHandler) HandleGovernanceDeleteMemory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.GovernanceDeleteMemory(req.ID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// HandleGovernanceBulkDelete handles POST /governance/bulk-delete.
func (h *AdvancedHandler) HandleGovernanceBulkDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	count, err := h.svc.GovernanceBulkDelete(req.IDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"deleted": count})
}
