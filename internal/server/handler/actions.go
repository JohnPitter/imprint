package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"imprint/internal/service"
)

// ActionHandler holds HTTP handlers for action, lease, and routine endpoints.
type ActionHandler struct {
	svc *service.ActionService
}

// NewActionHandler creates a new ActionHandler.
func NewActionHandler(svc *service.ActionService) *ActionHandler {
	return &ActionHandler{svc: svc}
}

// HandleCreateAction handles POST /actions.
func (h *ActionHandler) HandleCreateAction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Status      string   `json:"status"`
		Priority    int      `json:"priority"`
		Project     *string  `json:"project"`
		Tags        []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	action, err := h.svc.CreateAction(req.Title, req.Description, req.Status, req.Priority, req.Project, req.Tags)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"action": action,
	})
}

// HandleListActions handles GET /actions.
func (h *ActionHandler) HandleListActions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := q.Get("status")
	project := q.Get("project")

	limit := 50
	if v := q.Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if v := q.Get("offset"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	actions, err := h.svc.ListActions(status, project, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"actions": actions,
	})
}

// HandleUpdateAction handles POST /actions/update.
func (h *ActionHandler) HandleUpdateAction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID      string         `json:"id"`
		Updates map[string]any `json:"updates"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.UpdateAction(req.ID, req.Updates); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleGetAction handles GET /actions/get.
func (h *ActionHandler) HandleGetAction(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id query parameter is required")
		return
	}

	action, err := h.svc.GetAction(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"action": action,
	})
}

// HandleCreateEdge handles POST /actions/edges.
func (h *ActionHandler) HandleCreateEdge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceID string `json:"sourceId"`
		TargetID string `json:"targetId"`
		Type     string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	edge, err := h.svc.CreateEdge(req.SourceID, req.TargetID, req.Type)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"edge": edge,
	})
}

// HandleGetFrontier handles GET /frontier.
func (h *ActionHandler) HandleGetFrontier(w http.ResponseWriter, r *http.Request) {
	actions, err := h.svc.GetFrontier()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"actions": actions,
	})
}

// HandleGetNext handles GET /next.
func (h *ActionHandler) HandleGetNext(w http.ResponseWriter, r *http.Request) {
	action, err := h.svc.GetNext()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"action": action,
	})
}

// HandleAcquireLease handles POST /leases/acquire.
func (h *ActionHandler) HandleAcquireLease(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ActionID   string `json:"actionId"`
		AgentID    string `json:"agentId"`
		TTLSeconds int    `json:"ttlSeconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	lease, err := h.svc.AcquireLease(req.ActionID, req.AgentID, req.TTLSeconds)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"lease": lease,
	})
}

// HandleReleaseLease handles POST /leases/release.
func (h *ActionHandler) HandleReleaseLease(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LeaseID string  `json:"leaseId"`
		AgentID string  `json:"agentId"`
		Result  *string `json:"result"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.ReleaseLease(req.LeaseID, req.AgentID, req.Result); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleRenewLease handles POST /leases/renew.
func (h *ActionHandler) HandleRenewLease(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LeaseID    string `json:"leaseId"`
		AgentID    string `json:"agentId"`
		TTLSeconds int    `json:"ttlSeconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.RenewLease(req.LeaseID, req.AgentID, req.TTLSeconds); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleCreateRoutine handles POST /routines.
func (h *ActionHandler) HandleCreateRoutine(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string          `json:"name"`
		Steps  json.RawMessage `json:"steps"`
		Tags   json.RawMessage `json:"tags"`
		Frozen int             `json:"frozen"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	routine, err := h.svc.CreateRoutine(req.Name, req.Steps, req.Tags, req.Frozen)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"routine": routine,
	})
}

// HandleListRoutines handles GET /routines.
func (h *ActionHandler) HandleListRoutines(w http.ResponseWriter, r *http.Request) {
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

	routines, err := h.svc.ListRoutines(limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"routines": routines,
	})
}

// HandleRunRoutine handles POST /routines/run.
func (h *ActionHandler) HandleRunRoutine(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RoutineID string `json:"routineId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	actions, err := h.svc.RunRoutine(req.RoutineID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"actions": actions,
	})
}

// HandleRoutineStatus handles GET /routines/status.
func (h *ActionHandler) HandleRoutineStatus(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id query parameter is required")
		return
	}

	routine, err := h.svc.GetRoutine(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"routine": routine,
	})
}
