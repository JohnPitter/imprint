package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"imprint/internal/config"
	"imprint/internal/service"
	"imprint/internal/store"
	"imprint/internal/types"
)

// ---------------------------------------------------------------------------
// Helpers to build handlers from an in-memory test container.
// ---------------------------------------------------------------------------

func newTestContainer(t *testing.T) *service.Container {
	t.Helper()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return service.NewContainer(db)
}

func newActionHandler(t *testing.T) *ActionHandler {
	t.Helper()
	c := newTestContainer(t)
	return NewActionHandler(service.NewActionService(c))
}

func newAdvancedHandler(t *testing.T) (*AdvancedHandler, *service.Container) {
	t.Helper()
	c := newTestContainer(t)
	return NewAdvancedHandler(service.NewAdvancedService(c)), c
}

func newGraphHandler(t *testing.T) (*GraphHandler, *service.Container) {
	t.Helper()
	c := newTestContainer(t)
	// extractor is nil — only HandleExtract calls it and that handler doesn't reach extractor.
	return NewGraphHandler(service.NewGraphService(c, nil)), c
}

func newPipelineHandler(t *testing.T) (*PipelineHandler, *service.Container) {
	t.Helper()
	c := newTestContainer(t)
	// All pipeline services are nil — we only test validation paths that don't invoke them.
	return NewPipelineHandler(service.NewPipelineService(c, nil, nil, nil, nil)), c
}

// postRaw sends raw bytes to a handler — useful for malformed JSON tests.
func postRaw(handler http.HandlerFunc, path string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

// ---------------------------------------------------------------------------
// helpers.go
// ---------------------------------------------------------------------------

func TestOrEmpty(t *testing.T) {
	// Nil slice -> empty array.
	var nilSlice []string
	if got := orEmpty(nilSlice); got == nil {
		t.Error("orEmpty(nil slice) returned nil")
	}
	// Non-nil slice passes through.
	s := []string{"a", "b"}
	got := orEmpty(s)
	if gs, ok := got.([]string); !ok || len(gs) != 2 {
		t.Errorf("orEmpty(slice) = %v, want original slice", got)
	}
	// Invalid value -> empty array.
	if got := orEmpty(nil); got == nil {
		t.Error("orEmpty(nil) returned nil")
	}
	// Non-slice value passes through.
	if got := orEmpty(42); got != 42 {
		t.Errorf("orEmpty(int) = %v, want 42", got)
	}
}

func TestBuildContextXML_Empty(t *testing.T) {
	if got := buildContextXML(nil, "proj"); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
	if got := buildContextXML([]types.ContextBlock{}, "proj"); got != "" {
		t.Errorf("expected empty for empty blocks, got %q", got)
	}
}

func TestBuildContextXML_WithBlocks(t *testing.T) {
	blocks := []types.ContextBlock{
		{Type: "summary", Content: "hello"},
		{Type: "memory", Content: "world"},
	}
	got := buildContextXML(blocks, "myproj")
	if !strings.Contains(got, `project="myproj"`) {
		t.Errorf("missing project attr: %s", got)
	}
	if !strings.Contains(got, "<summary>") || !strings.Contains(got, "</summary>") {
		t.Errorf("missing summary tags: %s", got)
	}
	if !strings.Contains(got, "</imprint-context>") {
		t.Errorf("missing closing tag: %s", got)
	}
}

// ---------------------------------------------------------------------------
// ActionHandler
// ---------------------------------------------------------------------------

func TestActionHandler_CreateAndList(t *testing.T) {
	h := newActionHandler(t)

	rec := postJSON(h.HandleCreateAction, "/actions", map[string]any{
		"title":    "Build handler tests",
		"status":   "pending",
		"priority": 7,
		"tags":     []string{"go", "test"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("CreateAction: %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	action := body["action"].(map[string]any)
	id := action["id"].(string)

	// List
	rec = getJSON(h.HandleListActions, "/actions?limit=10&offset=0")
	if rec.Code != http.StatusOK {
		t.Fatalf("ListActions: %d", rec.Code)
	}
	body = decodeBody(t, rec)
	actions := body["actions"].([]any)
	if len(actions) != 1 {
		t.Errorf("expected 1 action, got %d", len(actions))
	}

	// Filter by status
	rec = getJSON(h.HandleListActions, "/actions?status=pending&project=&limit=abc&offset=-5")
	if rec.Code != http.StatusOK {
		t.Fatalf("ListActions with filters: %d", rec.Code)
	}

	// Get by id
	rec = getJSON(h.HandleGetAction, "/actions/get?id="+id)
	if rec.Code != http.StatusOK {
		t.Fatalf("GetAction: %d", rec.Code)
	}

	// Get with missing id
	rec = getJSON(h.HandleGetAction, "/actions/get")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("GetAction missing id: expected 400, got %d", rec.Code)
	}

	// Update
	rec = postJSON(h.HandleUpdateAction, "/actions/update", map[string]any{
		"id": id,
		"updates": map[string]any{
			"status":   "done",
			"priority": 9,
			"title":    "Updated",
		},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("UpdateAction: %d: %s", rec.Code, rec.Body.String())
	}

	// Update with missing id -> internal error
	rec = postJSON(h.HandleUpdateAction, "/actions/update", map[string]any{"id": ""})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("UpdateAction missing id: expected 500, got %d", rec.Code)
	}
}

func TestActionHandler_CreateAction_BadJSON(t *testing.T) {
	h := newActionHandler(t)
	rec := postRaw(h.HandleCreateAction, "/actions", []byte("not json"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad JSON, got %d", rec.Code)
	}
}

func TestActionHandler_CreateAction_MissingTitle(t *testing.T) {
	h := newActionHandler(t)
	rec := postJSON(h.HandleCreateAction, "/actions", map[string]any{
		"title": "",
	})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for missing title, got %d", rec.Code)
	}
}

func TestActionHandler_FromTask(t *testing.T) {
	h := newActionHandler(t)

	rec := postJSON(h.HandleFromTask, "/actions/from-task", map[string]any{
		"title":       "Task title",
		"description": "desc",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("FromTask: %d: %s", rec.Code, rec.Body.String())
	}

	// Missing title.
	rec = postJSON(h.HandleFromTask, "/actions/from-task", map[string]any{"title": ""})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 missing title, got %d", rec.Code)
	}

	// Bad JSON.
	rec = postRaw(h.HandleFromTask, "/actions/from-task", []byte("{"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 bad JSON, got %d", rec.Code)
	}
}

func TestActionHandler_Edges_Frontier_Next(t *testing.T) {
	h := newActionHandler(t)

	// Create two actions.
	rec := postJSON(h.HandleCreateAction, "/actions", map[string]any{"title": "A"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("A: %d", rec.Code)
	}
	aID := decodeBody(t, rec)["action"].(map[string]any)["id"].(string)
	rec = postJSON(h.HandleCreateAction, "/actions", map[string]any{"title": "B"})
	bID := decodeBody(t, rec)["action"].(map[string]any)["id"].(string)

	// Create edge
	rec = postJSON(h.HandleCreateEdge, "/actions/edges", map[string]any{
		"sourceId": aID,
		"targetId": bID,
		"type":     "blocks",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("CreateEdge: %d: %s", rec.Code, rec.Body.String())
	}

	// Bad JSON edge
	rec = postRaw(h.HandleCreateEdge, "/actions/edges", []byte("{"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 bad JSON, got %d", rec.Code)
	}

	// Missing fields edge
	rec = postJSON(h.HandleCreateEdge, "/actions/edges", map[string]any{})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 missing fields, got %d", rec.Code)
	}

	// Frontier
	rec = getJSON(h.HandleGetFrontier, "/frontier")
	if rec.Code != http.StatusOK {
		t.Fatalf("frontier: %d", rec.Code)
	}

	// Next
	rec = getJSON(h.HandleGetNext, "/next")
	if rec.Code != http.StatusOK {
		t.Fatalf("next: %d", rec.Code)
	}
}

func TestActionHandler_Leases(t *testing.T) {
	h := newActionHandler(t)

	// Create action
	rec := postJSON(h.HandleCreateAction, "/actions", map[string]any{"title": "Leasable"})
	aID := decodeBody(t, rec)["action"].(map[string]any)["id"].(string)

	// Acquire
	rec = postJSON(h.HandleAcquireLease, "/leases/acquire", map[string]any{
		"actionId":   aID,
		"agentId":    "agent-1",
		"ttlSeconds": 60,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("acquire: %d: %s", rec.Code, rec.Body.String())
	}
	leaseID := decodeBody(t, rec)["lease"].(map[string]any)["id"].(string)

	// Conflict on second acquire
	rec = postJSON(h.HandleAcquireLease, "/leases/acquire", map[string]any{
		"actionId":   aID,
		"agentId":    "agent-2",
		"ttlSeconds": 60,
	})
	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409 on second acquire, got %d", rec.Code)
	}

	// Bad JSON
	rec = postRaw(h.HandleAcquireLease, "/leases/acquire", []byte("}"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}

	// Renew
	rec = postJSON(h.HandleRenewLease, "/leases/renew", map[string]any{
		"leaseId":    leaseID,
		"agentId":    "agent-1",
		"ttlSeconds": 120,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("renew: %d: %s", rec.Code, rec.Body.String())
	}

	// Bad JSON renew
	rec = postRaw(h.HandleRenewLease, "/leases/renew", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("renew bad JSON: expected 400, got %d", rec.Code)
	}

	// Renew with missing fields -> error
	rec = postJSON(h.HandleRenewLease, "/leases/renew", map[string]any{})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("renew missing: expected 500, got %d", rec.Code)
	}

	// Release
	rec = postJSON(h.HandleReleaseLease, "/leases/release", map[string]any{
		"leaseId": leaseID,
		"agentId": "agent-1",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("release: %d: %s", rec.Code, rec.Body.String())
	}

	// Bad JSON release
	rec = postRaw(h.HandleReleaseLease, "/leases/release", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("release bad JSON: expected 400, got %d", rec.Code)
	}

	// Release missing fields -> error
	rec = postJSON(h.HandleReleaseLease, "/leases/release", map[string]any{})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("release missing: expected 500, got %d", rec.Code)
	}
}

func TestActionHandler_Routines(t *testing.T) {
	h := newActionHandler(t)

	// Create routine.
	rec := postJSON(h.HandleCreateRoutine, "/routines", map[string]any{
		"name":  "morning",
		"steps": []map[string]any{{"title": "Step 1", "priority": 5}, {"title": "Step 2", "priority": 6}},
		"tags":  []string{"daily"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("CreateRoutine: %d: %s", rec.Code, rec.Body.String())
	}
	rID := decodeBody(t, rec)["routine"].(map[string]any)["id"].(string)

	// Missing name
	rec = postJSON(h.HandleCreateRoutine, "/routines", map[string]any{"name": ""})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("CreateRoutine missing name: %d", rec.Code)
	}

	// Bad JSON
	rec = postRaw(h.HandleCreateRoutine, "/routines", []byte(")"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("CreateRoutine bad JSON: %d", rec.Code)
	}

	// List
	rec = getJSON(h.HandleListRoutines, "/routines?limit=5&offset=0")
	if rec.Code != http.StatusOK {
		t.Fatalf("ListRoutines: %d", rec.Code)
	}

	// List with invalid limit/offset
	rec = getJSON(h.HandleListRoutines, "/routines?limit=x&offset=y")
	if rec.Code != http.StatusOK {
		t.Errorf("ListRoutines bad params: %d", rec.Code)
	}

	// Run
	rec = postJSON(h.HandleRunRoutine, "/routines/run", map[string]any{"routineId": rID})
	if rec.Code != http.StatusOK {
		t.Fatalf("RunRoutine: %d: %s", rec.Code, rec.Body.String())
	}

	// Run bad JSON
	rec = postRaw(h.HandleRunRoutine, "/routines/run", []byte("{"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("RunRoutine bad JSON: %d", rec.Code)
	}

	// Status
	rec = getJSON(h.HandleRoutineStatus, "/routines/status?id="+rID)
	if rec.Code != http.StatusOK {
		t.Fatalf("RoutineStatus: %d", rec.Code)
	}

	// Status missing id
	rec = getJSON(h.HandleRoutineStatus, "/routines/status")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("RoutineStatus missing id: %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// AdvancedHandler — queryInt + signals/checkpoints/sentinels/sketches/lessons/
// insights/facets/audit/governance
// ---------------------------------------------------------------------------

func TestQueryInt(t *testing.T) {
	mk := func(q string) *http.Request {
		return httptest.NewRequest(http.MethodGet, "/?"+q, nil)
	}
	if got := queryInt(mk(""), "k", 10); got != 10 {
		t.Errorf("empty -> default: got %d", got)
	}
	if got := queryInt(mk("k=abc"), "k", 10); got != 10 {
		t.Errorf("invalid -> default: got %d", got)
	}
	if got := queryInt(mk("k=-1"), "k", 10); got != 10 {
		t.Errorf("negative -> default: got %d", got)
	}
	if got := queryInt(mk("k=42"), "k", 10); got != 42 {
		t.Errorf("valid: got %d", got)
	}
}

func TestAdvancedHandler_Signals(t *testing.T) {
	h, _ := newAdvancedHandler(t)

	rec := postJSON(h.HandleSendSignal, "/signals/send", map[string]string{
		"from":    "a",
		"to":      "b",
		"content": "hi",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("send: %d: %s", rec.Code, rec.Body.String())
	}
	// Missing required
	rec = postJSON(h.HandleSendSignal, "/signals/send", map[string]string{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("send missing: %d", rec.Code)
	}
	// Bad JSON
	rec = postRaw(h.HandleSendSignal, "/signals/send", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("send bad JSON: %d", rec.Code)
	}

	// List for b
	rec = getJSON(h.HandleListSignals, "/signals?agentId=b")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}

	// List missing agentId
	rec = getJSON(h.HandleListSignals, "/signals")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("list missing: %d", rec.Code)
	}
}

func TestAdvancedHandler_Checkpoints(t *testing.T) {
	h, _ := newAdvancedHandler(t)

	rec := postJSON(h.HandleCreateCheckpoint, "/checkpoints", map[string]any{
		"name":        "Release approval",
		"description": "Before prod deploy",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d: %s", rec.Code, rec.Body.String())
	}
	id := decodeBody(t, rec)["id"].(string)

	// Missing name
	rec = postJSON(h.HandleCreateCheckpoint, "/checkpoints", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("create missing: %d", rec.Code)
	}
	rec = postRaw(h.HandleCreateCheckpoint, "/checkpoints", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("create bad JSON: %d", rec.Code)
	}

	// List
	rec = getJSON(h.HandleListCheckpoints, "/checkpoints?limit=10")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}

	// Resolve
	rec = postJSON(h.HandleResolveCheckpoint, "/checkpoints/resolve", map[string]any{
		"id":         id,
		"resolvedBy": "me",
		"result":     "ok",
		"status":     "approved",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("resolve: %d: %s", rec.Code, rec.Body.String())
	}
	rec = postJSON(h.HandleResolveCheckpoint, "/checkpoints/resolve", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("resolve missing: %d", rec.Code)
	}
	rec = postRaw(h.HandleResolveCheckpoint, "/checkpoints/resolve", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("resolve bad JSON: %d", rec.Code)
	}
}

func TestAdvancedHandler_Sentinels(t *testing.T) {
	h, _ := newAdvancedHandler(t)

	rec := postJSON(h.HandleCreateSentinel, "/sentinels", map[string]any{
		"name":   "Watch",
		"type":   "file_change",
		"config": map[string]any{"path": "/tmp"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d: %s", rec.Code, rec.Body.String())
	}
	id := decodeBody(t, rec)["id"].(string)

	// Missing
	rec = postJSON(h.HandleCreateSentinel, "/sentinels", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("create missing: %d", rec.Code)
	}
	rec = postRaw(h.HandleCreateSentinel, "/sentinels", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("create bad JSON: %d", rec.Code)
	}

	// List
	rec = getJSON(h.HandleListSentinels, "/sentinels?status=active&limit=10")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}

	// Check
	rec = postJSON(h.HandleCheckSentinel, "/sentinels/check", map[string]any{"id": id})
	if rec.Code != http.StatusOK {
		t.Fatalf("check: %d: %s", rec.Code, rec.Body.String())
	}
	rec = postJSON(h.HandleCheckSentinel, "/sentinels/check", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("check missing: %d", rec.Code)
	}
	rec = postRaw(h.HandleCheckSentinel, "/sentinels/check", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("check bad JSON: %d", rec.Code)
	}

	// Trigger
	rec = postJSON(h.HandleTriggerSentinel, "/sentinels/trigger", map[string]any{
		"id":     id,
		"result": "triggered",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("trigger: %d: %s", rec.Code, rec.Body.String())
	}
	rec = postJSON(h.HandleTriggerSentinel, "/sentinels/trigger", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("trigger missing: %d", rec.Code)
	}
	rec = postRaw(h.HandleTriggerSentinel, "/sentinels/trigger", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("trigger bad JSON: %d", rec.Code)
	}

	// Cancel
	rec = postJSON(h.HandleCancelSentinel, "/sentinels/cancel", map[string]any{"id": id})
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel: %d: %s", rec.Code, rec.Body.String())
	}
	rec = postJSON(h.HandleCancelSentinel, "/sentinels/cancel", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("cancel missing: %d", rec.Code)
	}
	rec = postRaw(h.HandleCancelSentinel, "/sentinels/cancel", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("cancel bad JSON: %d", rec.Code)
	}
}

func TestAdvancedHandler_Sketches(t *testing.T) {
	h, _ := newAdvancedHandler(t)

	rec := postJSON(h.HandleCreateSketch, "/sketches", map[string]any{
		"title":          "Draft",
		"description":    "d",
		"expiresInHours": 2,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d: %s", rec.Code, rec.Body.String())
	}
	id := decodeBody(t, rec)["id"].(string)

	rec = postJSON(h.HandleCreateSketch, "/sketches", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("create missing: %d", rec.Code)
	}
	rec = postRaw(h.HandleCreateSketch, "/sketches", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("create bad JSON: %d", rec.Code)
	}

	// List
	rec = getJSON(h.HandleListSketches, "/sketches?status=active&limit=10")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}

	// Add
	rec = postJSON(h.HandleAddToSketch, "/sketches/add", map[string]any{
		"sketchId": id,
		"actionId": "act_x",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("add: %d: %s", rec.Code, rec.Body.String())
	}
	rec = postJSON(h.HandleAddToSketch, "/sketches/add", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("add missing: %d", rec.Code)
	}
	rec = postRaw(h.HandleAddToSketch, "/sketches/add", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("add bad JSON: %d", rec.Code)
	}

	// Promote
	rec = postJSON(h.HandlePromoteSketch, "/sketches/promote", map[string]any{"id": id})
	if rec.Code != http.StatusOK {
		t.Fatalf("promote: %d: %s", rec.Code, rec.Body.String())
	}
	rec = postJSON(h.HandlePromoteSketch, "/sketches/promote", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("promote missing: %d", rec.Code)
	}
	rec = postRaw(h.HandlePromoteSketch, "/sketches/promote", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("promote bad JSON: %d", rec.Code)
	}

	// Create another for discard
	rec = postJSON(h.HandleCreateSketch, "/sketches", map[string]any{"title": "d2"})
	id2 := decodeBody(t, rec)["id"].(string)

	rec = postJSON(h.HandleDiscardSketch, "/sketches/discard", map[string]any{"id": id2})
	if rec.Code != http.StatusOK {
		t.Fatalf("discard: %d: %s", rec.Code, rec.Body.String())
	}
	rec = postJSON(h.HandleDiscardSketch, "/sketches/discard", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("discard missing: %d", rec.Code)
	}
	rec = postRaw(h.HandleDiscardSketch, "/sketches/discard", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("discard bad JSON: %d", rec.Code)
	}

	// GC
	rec = postJSON(h.HandleGarbageCollectSketches, "/sketches/gc", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("gc: %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdvancedHandler_Lessons(t *testing.T) {
	h, _ := newAdvancedHandler(t)

	rec := postJSON(h.HandleCreateLesson, "/lessons", map[string]any{
		"content": "Always write tests",
		"context": "go",
		"source":  "manual",
		"tags":    []string{"dev"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d: %s", rec.Code, rec.Body.String())
	}
	id := decodeBody(t, rec)["id"].(string)

	// Missing
	rec = postJSON(h.HandleCreateLesson, "/lessons", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("create missing: %d", rec.Code)
	}
	rec = postRaw(h.HandleCreateLesson, "/lessons", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("create bad JSON: %d", rec.Code)
	}

	// List
	rec = getJSON(h.HandleListLessons, "/lessons?limit=10&offset=0")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}

	// Search
	rec = postJSON(h.HandleSearchLessons, "/lessons/search", map[string]any{
		"query": "tests",
		"limit": 10,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("search: %d: %s", rec.Code, rec.Body.String())
	}

	// Search empty
	rec = postJSON(h.HandleSearchLessons, "/lessons/search", map[string]any{"query": ""})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("search empty: %d", rec.Code)
	}
	rec = postRaw(h.HandleSearchLessons, "/lessons/search", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("search bad JSON: %d", rec.Code)
	}

	// Strengthen
	rec = postJSON(h.HandleStrengthenLesson, "/lessons/strengthen", map[string]any{"id": id})
	if rec.Code != http.StatusOK {
		t.Fatalf("strengthen: %d: %s", rec.Code, rec.Body.String())
	}
	rec = postJSON(h.HandleStrengthenLesson, "/lessons/strengthen", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("strengthen missing: %d", rec.Code)
	}
	rec = postRaw(h.HandleStrengthenLesson, "/lessons/strengthen", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("strengthen bad JSON: %d", rec.Code)
	}
}

func TestAdvancedHandler_Insights(t *testing.T) {
	h, _ := newAdvancedHandler(t)

	// List (empty).
	rec := getJSON(h.HandleListInsights, "/insights?limit=10")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}

	// Search with query.
	rec = postJSON(h.HandleSearchInsights, "/insights/search", map[string]any{
		"query": "foo",
		"limit": 5,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("search: %d: %s", rec.Code, rec.Body.String())
	}

	// Search empty
	rec = postJSON(h.HandleSearchInsights, "/insights/search", map[string]any{"query": ""})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("search empty: %d", rec.Code)
	}
	rec = postRaw(h.HandleSearchInsights, "/insights/search", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("search bad JSON: %d", rec.Code)
	}
}

func TestAdvancedHandler_Facets(t *testing.T) {
	h, _ := newAdvancedHandler(t)

	rec := postJSON(h.HandleCreateFacet, "/facets", map[string]any{
		"targetId":   "obj_1",
		"targetType": "memory",
		"dimension":  "lang",
		"value":      "go",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d: %s", rec.Code, rec.Body.String())
	}
	id := decodeBody(t, rec)["id"].(string)

	rec = postJSON(h.HandleCreateFacet, "/facets", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("create missing: %d", rec.Code)
	}
	rec = postRaw(h.HandleCreateFacet, "/facets", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("create bad JSON: %d", rec.Code)
	}

	// Get
	rec = getJSON(h.HandleGetFacets, "/facets?targetId=obj_1&targetType=memory")
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d", rec.Code)
	}
	rec = getJSON(h.HandleGetFacets, "/facets")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("get missing: %d", rec.Code)
	}

	// Query
	rec = postJSON(h.HandleQueryFacets, "/facets/query", map[string]any{
		"dimension": "lang",
		"value":     "go",
		"limit":     5,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("query: %d: %s", rec.Code, rec.Body.String())
	}
	rec = postJSON(h.HandleQueryFacets, "/facets/query", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("query missing: %d", rec.Code)
	}
	rec = postRaw(h.HandleQueryFacets, "/facets/query", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("query bad JSON: %d", rec.Code)
	}

	// Stats
	rec = getJSON(h.HandleFacetStats, "/facets/stats")
	if rec.Code != http.StatusOK {
		t.Fatalf("stats: %d", rec.Code)
	}

	// Remove
	rec = postJSON(h.HandleRemoveFacet, "/facets/remove", map[string]any{"id": id})
	if rec.Code != http.StatusOK {
		t.Fatalf("remove: %d: %s", rec.Code, rec.Body.String())
	}
	rec = postJSON(h.HandleRemoveFacet, "/facets/remove", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("remove missing: %d", rec.Code)
	}
	rec = postRaw(h.HandleRemoveFacet, "/facets/remove", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("remove bad JSON: %d", rec.Code)
	}
}

func TestAdvancedHandler_Audit(t *testing.T) {
	h, _ := newAdvancedHandler(t)

	rec := getJSON(h.HandleListAudit, "/audit?limit=10&offset=0")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}
	rec = getJSON(h.HandleListAudit, "/audit?action=governance.delete")
	if rec.Code != http.StatusOK {
		t.Fatalf("list with action: %d", rec.Code)
	}
}

func TestAdvancedHandler_Governance(t *testing.T) {
	h, c := newAdvancedHandler(t)

	// Pre-seed a memory to delete (strength must be 1-10).
	mem := &store.MemoryRow{
		ID:       "mem_gov1",
		Type:     "fact",
		Title:    "gov",
		Content:  "x",
		Strength: 5,
		IsLatest: 1,
		Version:  1,
	}
	if err := c.Memories.Create(mem); err != nil {
		t.Fatalf("seed memory: %v", err)
	}

	rec := postJSON(h.HandleGovernanceDeleteMemory, "/governance/memories", map[string]any{"id": "mem_gov1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d: %s", rec.Code, rec.Body.String())
	}
	rec = postJSON(h.HandleGovernanceDeleteMemory, "/governance/memories", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("delete missing: %d", rec.Code)
	}
	rec = postRaw(h.HandleGovernanceDeleteMemory, "/governance/memories", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("delete bad JSON: %d", rec.Code)
	}

	// Bulk: seed two more.
	for _, id := range []string{"mem_gov2", "mem_gov3"} {
		_ = c.Memories.Create(&store.MemoryRow{ID: id, Type: "fact", Title: "t", Content: "c", Strength: 5, IsLatest: 1, Version: 1})
	}
	rec = postJSON(h.HandleGovernanceBulkDelete, "/governance/bulk-delete", map[string]any{
		"ids": []string{"mem_gov2", "mem_gov3"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("bulk: %d: %s", rec.Code, rec.Body.String())
	}
	rec = postJSON(h.HandleGovernanceBulkDelete, "/governance/bulk-delete", map[string]any{"ids": []string{}})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bulk empty: %d", rec.Code)
	}
	rec = postRaw(h.HandleGovernanceBulkDelete, "/governance/bulk-delete", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bulk bad JSON: %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// GraphHandler
// ---------------------------------------------------------------------------

func TestGraphHandler_ExtractAndBadJSON(t *testing.T) {
	h, _ := newGraphHandler(t)
	// Happy-path — HandleExtract does not call the extractor, it only ACKs.
	rec := postJSON(h.HandleExtract, "/imprint/graph/extract", map[string]string{
		"observationId": "obs_1",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("extract: %d: %s", rec.Code, rec.Body.String())
	}
	rec = postRaw(h.HandleExtract, "/imprint/graph/extract", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("extract bad JSON: %d", rec.Code)
	}
}

func TestGraphHandler_QueryAndStatsAndAll(t *testing.T) {
	h, c := newGraphHandler(t)

	// Seed graph.
	if err := c.Graph.CreateNode(&store.GraphNodeRow{ID: "gn_q1", Type: "file", Name: "a.go"}); err != nil {
		t.Fatalf("seed node: %v", err)
	}
	if err := c.Graph.CreateNode(&store.GraphNodeRow{ID: "gn_q2", Type: "file", Name: "b.go"}); err != nil {
		t.Fatalf("seed node: %v", err)
	}
	if err := c.Graph.CreateEdge(&store.GraphEdgeRow{
		ID: "ge_q1", Type: "imports", SourceNodeID: "gn_q1", TargetNodeID: "gn_q2",
		Weight: 0.8, IsLatest: 1, Version: 1,
	}); err != nil {
		t.Fatalf("seed edge: %v", err)
	}

	// Query
	rec := postJSON(h.HandleQuery, "/imprint/graph/query", map[string]any{
		"startNodeId": "gn_q1",
		"maxDepth":    2,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("query: %d: %s", rec.Code, rec.Body.String())
	}

	// Query missing startNodeId
	rec = postJSON(h.HandleQuery, "/imprint/graph/query", map[string]any{"startNodeId": ""})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("query missing: %d", rec.Code)
	}
	// Query with invalid startNodeId
	rec = postJSON(h.HandleQuery, "/imprint/graph/query", map[string]any{"startNodeId": "nope"})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("query invalid: %d", rec.Code)
	}
	// Query bad JSON
	rec = postRaw(h.HandleQuery, "/imprint/graph/query", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("query bad JSON: %d", rec.Code)
	}

	// Stats
	rec = getJSON(h.HandleStats, "/imprint/graph/stats")
	if rec.Code != http.StatusOK {
		t.Fatalf("stats: %d: %s", rec.Code, rec.Body.String())
	}

	// All
	rec = getJSON(h.HandleAll, "/imprint/graph/all")
	if rec.Code != http.StatusOK {
		t.Fatalf("all: %d: %s", rec.Code, rec.Body.String())
	}

	// Relations
	rec = postJSON(h.HandleRelations, "/imprint/relations", map[string]any{
		"sourceNodeId": "gn_q1",
		"targetNodeId": "gn_q2",
		"type":         "related_to",
		"weight":       0.5,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("relations: %d: %s", rec.Code, rec.Body.String())
	}
	rec = postJSON(h.HandleRelations, "/imprint/relations", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("relations missing: %d", rec.Code)
	}
	rec = postRaw(h.HandleRelations, "/imprint/relations", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("relations bad JSON: %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// PipelineHandler — only test validation paths (nil service would panic otherwise).
// ---------------------------------------------------------------------------

func TestPipelineHandler_Validation(t *testing.T) {
	h, _ := newPipelineHandler(t)

	// Each endpoint: bad JSON + missing sessionId.
	for _, hnd := range []http.HandlerFunc{
		h.HandleSummarize,
		h.HandleConsolidatePipeline,
		h.HandleFullPipeline,
		h.HandleFinalize,
	} {
		rec := postRaw(hnd, "/", []byte("x"))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("bad JSON: expected 400, got %d", rec.Code)
		}
		rec = postJSON(hnd, "/", map[string]any{"sessionId": ""})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("missing sessionId: expected 400, got %d", rec.Code)
		}
	}
}

// ---------------------------------------------------------------------------
// SettingsHandler
// ---------------------------------------------------------------------------

func TestSettingsHandler_GetAndUpdate(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		DataDir:          dir,
		AnthropicAPIKey:  "abc12345XYZ98765",
		AnthropicModel:   "claude-haiku-4-5-20251001",
		LLMProviderOrder: []string{"anthropic"},
	}
	h := NewSettingsHandler(cfg)

	// Get
	rec := getJSON(h.HandleGetSettings, "/settings")
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	llm := body["llm"].(map[string]any)
	key := llm["anthropicApiKey"].(string)
	if strings.Contains(key, "12345XYZ9") {
		t.Errorf("api key should be masked, got %s", key)
	}

	// Update
	rec = postJSON(h.HandleUpdateSettings, "/settings", map[string]any{
		"anthropicModel": "claude-sonnet-4-5",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d: %s", rec.Code, rec.Body.String())
	}
	body = decodeBody(t, rec)
	if body["status"] != "saved" {
		t.Errorf("expected status saved, got %v", body["status"])
	}
	if cfg.AnthropicModel != "claude-sonnet-4-5" {
		t.Errorf("cfg.AnthropicModel not applied: %s", cfg.AnthropicModel)
	}

	// Bad JSON
	rec = postRaw(h.HandleUpdateSettings, "/settings", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad JSON: %d", rec.Code)
	}
}

func TestSettingsHandler_UpdateSaveFailure(t *testing.T) {
	// Use a non-existent directory so SaveUserSettings fails.
	cfg := &config.Config{DataDir: "\x00/invalid/path"}
	h := NewSettingsHandler(cfg)

	rec := postJSON(h.HandleUpdateSettings, "/settings", map[string]any{"anthropicModel": "x"})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on save failure, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// Extra session/memory/observation edge cases to drive coverage.
// ---------------------------------------------------------------------------

func TestSessionHandler_BadJSONAndList(t *testing.T) {
	sh, _, _ := setupTestHandlers(t)

	rec := postRaw(sh.HandleStart, "/imprint/session/start", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("start bad JSON: %d", rec.Code)
	}

	rec = postRaw(sh.HandleEnd, "/imprint/session/end", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("end bad JSON: %d", rec.Code)
	}

	rec = postJSON(sh.HandleEnd, "/imprint/session/end", map[string]string{"sessionId": "missing"})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("end missing session: expected 500, got %d", rec.Code)
	}

	// List with pagination params.
	rec = getJSON(sh.HandleList, "/imprint/sessions?limit=5&offset=0")
	if rec.Code != http.StatusOK {
		t.Errorf("list with pagination: %d", rec.Code)
	}
	rec = getJSON(sh.HandleList, "/imprint/sessions?limit=abc&offset=xyz")
	if rec.Code != http.StatusOK {
		t.Errorf("list bad pagination: %d", rec.Code)
	}
}

func TestMemoryHandler_BadJSONAndList(t *testing.T) {
	_, _, mh := setupTestHandlers(t)

	rec := postRaw(mh.HandleRemember, "/imprint/remember", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("remember bad JSON: %d", rec.Code)
	}
	rec = postRaw(mh.HandleForget, "/imprint/forget", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("forget bad JSON: %d", rec.Code)
	}
	rec = postRaw(mh.HandleEvolve, "/imprint/evolve", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("evolve bad JSON: %d", rec.Code)
	}

	// Forget unknown id — service returns nil (idempotent soft-delete), so 200 is fine.
	rec = postJSON(mh.HandleForget, "/imprint/forget", map[string]string{"id": "nope"})
	if rec.Code != http.StatusOK && rec.Code != http.StatusInternalServerError {
		t.Errorf("forget unknown: unexpected %d", rec.Code)
	}

	// Evolve missing id — row not found -> error.
	rec = postJSON(mh.HandleEvolve, "/imprint/evolve", map[string]any{"id": "nope", "content": "x", "strength": 5})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("evolve unknown: expected 500, got %d", rec.Code)
	}

	// List with type filter + pagination.
	rec = getJSON(mh.HandleList, "/imprint/memories?type=pattern&limit=5&offset=0")
	if rec.Code != http.StatusOK {
		t.Errorf("list filter: %d", rec.Code)
	}
	rec = getJSON(mh.HandleList, "/imprint/memories?limit=abc&offset=xyz")
	if rec.Code != http.StatusOK {
		t.Errorf("list bad pagination: %d", rec.Code)
	}
}

func TestObservationHandler_BadJSONAndPagination(t *testing.T) {
	sh, oh, _ := setupTestHandlers(t)

	rec := postRaw(oh.HandleObserve, "/imprint/observe", []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("observe bad JSON: %d", rec.Code)
	}

	postJSON(sh.HandleStart, "/imprint/session/start", map[string]string{
		"sessionId": "ses_page",
		"project":   "proj",
		"cwd":       "/tmp",
	})

	// Observe with bad payload (no sessionId) -> service returns error.
	rec = postJSON(oh.HandleObserve, "/imprint/observe", types.HookPayload{
		HookType: types.HookPostToolUse,
	})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("observe invalid: expected 500, got %d", rec.Code)
	}

	// List with bad pagination params falls back to defaults.
	rec = getJSON(oh.HandleList, "/imprint/observations?sessionId=ses_page&limit=abc&offset=xyz")
	if rec.Code != http.StatusOK {
		t.Errorf("list pagination: %d", rec.Code)
	}
}

// TestObservationHandler_ListCompressed tests the compressed-list branch using
// a fresh container that lets us seed compressed rows directly.
func TestObservationHandler_ListCompressed(t *testing.T) {
	c := newTestContainer(t)

	// Seed session.
	if err := c.Sessions.Create(&store.SessionRow{
		ID: "ses_comp", Project: "proj", Cwd: "/tmp", Status: types.SessionActive,
	}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	// Seed compressed observation.
	narrative := "narr"
	if err := c.Observations.CreateCompressed(&store.CompressedObservationRow{
		ID:         "obs_comp1",
		SessionID:  "ses_comp",
		Type:       "decision",
		Title:      "Some title",
		Narrative:  &narrative,
		Concepts:   []string{"a"},
		Files:      []string{"x.go"},
		Importance: 5,
		Confidence: 0.8,
	}); err != nil {
		t.Fatalf("create compressed: %v", err)
	}

	svc := service.NewObserveService(c, 500, 8000)
	oh := NewObservationHandler(svc)

	rec := getJSON(oh.HandleList, "/imprint/observations?sessionId=ses_comp")
	if rec.Code != http.StatusOK {
		t.Fatalf("list compressed: %d: %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["type"] != "compressed" {
		t.Errorf("expected type compressed, got %v", body["type"])
	}
}

// Round-trip writeJSON/writeError already covered at 100%, but smoke-check
// headers.
func TestWriteJSON_Headers(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusTeapot, map[string]string{"k": "v"})
	if rec.Code != http.StatusTeapot {
		t.Errorf("status: got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type: got %s", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["k"] != "v" {
		t.Errorf("body mismatch: %v", body)
	}
}
