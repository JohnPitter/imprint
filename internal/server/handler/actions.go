package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"imprint/internal/eventbus"
	"imprint/internal/service"
)

// ActionHandler holds HTTP handlers for action, lease, and routine endpoints.
type ActionHandler struct {
	svc *service.ActionService
	bus *eventbus.Bus // optional; when set, /actions/stream pushes via SSE
}

// NewActionHandler creates a new ActionHandler. The bus argument is optional;
// pass nil if SSE streaming is not needed (the kanban will fall back to its
// poll loop and still work).
func NewActionHandler(svc *service.ActionService, bus *eventbus.Bus) *ActionHandler {
	return &ActionHandler{svc: svc, bus: bus}
}

// HandleActionsStream is GET /actions/stream — long-lived Server-Sent Events
// connection that emits a "data: changed\n\n" line every time an action is
// created, updated, or moved between statuses. Clients use this as a hint
// to immediately re-fetch the kanban instead of waiting for the next poll
// tick. Falls back gracefully: a 503 is returned when the bus is missing
// (legacy install) and the frontend keeps polling at its default cadence.
func (h *ActionHandler) HandleActionsStream(w http.ResponseWriter, r *http.Request) {
	if h.bus == nil {
		http.Error(w, "event bus disabled", http.StatusServiceUnavailable)
		return
	}

	// http.NewResponseController is the Go 1.20+ way to access Flush even
	// when the ResponseWriter has been wrapped by middleware (logger, etc.)
	// that hide the http.Flusher interface from a direct type assertion.
	rc := http.NewResponseController(w)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Initial event so the client knows the connection is live before the
	// first publish. Clients use this to immediately do a fresh fetch on
	// reconnect rather than waiting for the next mutation.
	if _, err := fmt.Fprint(w, "event: hello\ndata: connected\n\n"); err != nil {
		return
	}
	if err := rc.Flush(); err != nil {
		// Underlying writer doesn't support flushing; close the connection.
		return
	}

	ch, cancel := h.bus.Subscribe()
	defer cancel()

	// Keepalive: SSE clients (and any intermediate proxy) drop the
	// connection after long idle periods. A comment line every 25s
	// passes through SSE parsers as a no-op while keeping the TCP and
	// HTTP-level timers alive.
	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", ev); err != nil {
				return
			}
			_ = rc.Flush()
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			_ = rc.Flush()
		}
	}
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

// HandleFromTask handles POST /actions/from-task.
// Creates or updates an action from a Claude Code task completion event.
func (h *ActionHandler) HandleFromTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID   string `json:"sessionId"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if req.Status == "" {
		req.Status = "done"
	}

	action, err := h.svc.UpsertFromTask(req.Title, req.Description, req.Status, req.SessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": action})
}

// HandleCompleteInProgress handles POST /actions/complete-in-progress.
// Marks every in_progress action in the session's project as done. Used by the
// Stop hook to close out the action(s) opened by the matching prompt-submit.
func (h *ActionHandler) HandleCompleteInProgress(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updated, err := h.svc.CompleteInProgressForSession(req.SessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"completed": updated})
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
