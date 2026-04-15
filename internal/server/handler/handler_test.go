package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"imprint/internal/service"
	"imprint/internal/store"
	"imprint/internal/types"
)

func setupTestHandlers(t *testing.T) (*SessionHandler, *ObservationHandler, *MemoryHandler) {
	t.Helper()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	c := service.NewContainer(db)
	sessionSvc := service.NewSessionService(c)
	observeSvc := service.NewObserveService(c, 500, 8000)
	rememberSvc := service.NewRememberService(c)
	return NewSessionHandler(sessionSvc), NewObservationHandler(observeSvc), NewMemoryHandler(rememberSvc)
}

func postJSON(handler http.HandlerFunc, path string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func getJSON(handler http.HandlerFunc, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to decode response body: %v\nbody: %s", err, rec.Body.String())
	}
	return result
}

// ---------------------------------------------------------------------------
// SessionHandler
// ---------------------------------------------------------------------------

func TestSessionHandler_StartAndEnd(t *testing.T) {
	sh, _, _ := setupTestHandlers(t)

	// Start session.
	rec := postJSON(sh.HandleStart, "/imprint/session/start", map[string]string{
		"sessionId": "ses_http1",
		"project":   "proj",
		"cwd":       "/tmp/proj",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("HandleStart: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := decodeBody(t, rec)
	session, ok := body["session"].(map[string]any)
	if !ok {
		t.Fatal("response missing 'session' object")
	}
	if session["ID"] != "ses_http1" {
		t.Errorf("expected session ID ses_http1, got %v", session["ID"])
	}

	// End session.
	rec = postJSON(sh.HandleEnd, "/imprint/session/end", map[string]string{
		"sessionId": "ses_http1",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("HandleEnd: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	endBody := decodeBody(t, rec)
	if endBody["status"] != "ok" {
		t.Errorf("expected status ok, got %v", endBody["status"])
	}
}

func TestSessionHandler_StartBadRequest(t *testing.T) {
	sh, _, _ := setupTestHandlers(t)

	// Missing project should fail.
	rec := postJSON(sh.HandleStart, "/imprint/session/start", map[string]string{
		"sessionId": "ses_bad",
		"cwd":       "/tmp",
	})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for missing project, got %d", rec.Code)
	}
}

func TestSessionHandler_List(t *testing.T) {
	sh, _, _ := setupTestHandlers(t)

	// Start 2 sessions.
	for i := 0; i < 2; i++ {
		rec := postJSON(sh.HandleStart, "/imprint/session/start", map[string]string{
			"sessionId": fmt.Sprintf("ses_list_%d", i),
			"project":   "proj-list",
			"cwd":       "/tmp",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("Start session %d: %d", i, rec.Code)
		}
	}

	rec := getJSON(sh.HandleList, "/imprint/sessions?project=proj-list")
	if rec.Code != http.StatusOK {
		t.Fatalf("HandleList: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := decodeBody(t, rec)
	sessions, ok := body["sessions"].([]any)
	if !ok {
		t.Fatal("response missing 'sessions' array")
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

// ---------------------------------------------------------------------------
// ObservationHandler
// ---------------------------------------------------------------------------

func TestObservationHandler_Observe(t *testing.T) {
	sh, oh, _ := setupTestHandlers(t)

	// Create a session first.
	postJSON(sh.HandleStart, "/imprint/session/start", map[string]string{
		"sessionId": "ses_obshttp",
		"project":   "proj",
		"cwd":       "/tmp",
	})

	payload := types.HookPayload{
		SessionID: "ses_obshttp",
		HookType:  types.HookPostToolUse,
		ToolName:  strPtr("Write"),
		ToolInput: json.RawMessage(`{"path":"/tmp/test.go"}`),
	}
	rec := postJSON(oh.HandleObserve, "/imprint/observe", payload)
	if rec.Code != http.StatusCreated {
		t.Fatalf("HandleObserve: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	body := decodeBody(t, rec)
	obs, ok := body["observation"].(map[string]any)
	if !ok {
		t.Fatal("response missing 'observation' object")
	}
	if obs["SessionID"] != "ses_obshttp" {
		t.Errorf("expected session_id ses_obshttp, got %v", obs["SessionID"])
	}
}

func TestObservationHandler_ObserveDedup(t *testing.T) {
	sh, oh, _ := setupTestHandlers(t)

	postJSON(sh.HandleStart, "/imprint/session/start", map[string]string{
		"sessionId": "ses_dedup_http",
		"project":   "proj",
		"cwd":       "/tmp",
	})

	payload := types.HookPayload{
		SessionID: "ses_dedup_http",
		HookType:  types.HookPostToolUse,
		ToolName:  strPtr("Read"),
		ToolInput: json.RawMessage(`{"path":"/tmp/same.go"}`),
	}

	// First call — should succeed.
	rec := postJSON(oh.HandleObserve, "/imprint/observe", payload)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first observe: expected 201, got %d", rec.Code)
	}

	// Second call — duplicate, should be "skipped".
	rec = postJSON(oh.HandleObserve, "/imprint/observe", payload)
	if rec.Code != http.StatusOK {
		t.Fatalf("second observe: expected 200, got %d", rec.Code)
	}
	body := decodeBody(t, rec)
	if body["status"] != "skipped" {
		t.Errorf("expected status 'skipped', got %v", body["status"])
	}
}

func TestObservationHandler_List(t *testing.T) {
	sh, oh, _ := setupTestHandlers(t)

	postJSON(sh.HandleStart, "/imprint/session/start", map[string]string{
		"sessionId": "ses_obslist",
		"project":   "proj",
		"cwd":       "/tmp",
	})

	// Create 2 observations with different tool inputs to avoid dedup.
	for i := 0; i < 2; i++ {
		payload := types.HookPayload{
			SessionID: "ses_obslist",
			HookType:  types.HookPostToolUse,
			ToolName:  strPtr("Bash"),
			ToolInput: json.RawMessage(fmt.Sprintf(`{"cmd":"echo %d"}`, i)),
		}
		rec := postJSON(oh.HandleObserve, "/imprint/observe", payload)
		if rec.Code != http.StatusCreated {
			t.Fatalf("observe %d: expected 201, got %d: %s", i, rec.Code, rec.Body.String())
		}
	}

	rec := getJSON(oh.HandleList, "/imprint/observations?sessionId=ses_obslist")
	if rec.Code != http.StatusOK {
		t.Fatalf("HandleList: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := decodeBody(t, rec)
	observations, ok := body["observations"].([]any)
	if !ok {
		t.Fatal("response missing 'observations' array")
	}
	if len(observations) != 2 {
		t.Errorf("expected 2 observations, got %d", len(observations))
	}
	// Should be raw type since no compressed observations exist.
	if body["type"] != "raw" {
		t.Errorf("expected type 'raw', got %v", body["type"])
	}
}

func TestObservationHandler_ListRequiresSessionId(t *testing.T) {
	_, oh, _ := setupTestHandlers(t)

	rec := getJSON(oh.HandleList, "/imprint/observations")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when sessionId missing, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// MemoryHandler
// ---------------------------------------------------------------------------

func TestMemoryHandler_RememberAndList(t *testing.T) {
	_, _, mh := setupTestHandlers(t)

	rec := postJSON(mh.HandleRemember, "/imprint/remember", map[string]any{
		"type":     "pattern",
		"title":    "Error handling",
		"content":  "Always wrap errors with context",
		"concepts": []string{"go", "errors"},
		"files":    []string{"main.go"},
		"strength": 8,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("HandleRemember: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	body := decodeBody(t, rec)
	mem, ok := body["memory"].(map[string]any)
	if !ok {
		t.Fatal("response missing 'memory' object")
	}
	if mem["title"] != "Error handling" {
		t.Errorf("expected title 'Error handling', got %v", mem["title"])
	}

	// List memories.
	rec = getJSON(mh.HandleList, "/imprint/memories")
	if rec.Code != http.StatusOK {
		t.Fatalf("HandleList: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body = decodeBody(t, rec)
	memories, ok := body["memories"].([]any)
	if !ok {
		t.Fatal("response missing 'memories' array")
	}
	if len(memories) != 1 {
		t.Errorf("expected 1 memory, got %d", len(memories))
	}
}

func TestMemoryHandler_Forget(t *testing.T) {
	_, _, mh := setupTestHandlers(t)

	// Remember.
	rec := postJSON(mh.HandleRemember, "/imprint/remember", map[string]any{
		"type":     "fact",
		"title":    "Temp fact",
		"content":  "This will be forgotten",
		"strength": 5,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("HandleRemember: expected 201, got %d", rec.Code)
	}

	body := decodeBody(t, rec)
	mem := body["memory"].(map[string]any)
	memID := mem["id"].(string)

	// Forget.
	rec = postJSON(mh.HandleForget, "/imprint/forget", map[string]string{
		"id": memID,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("HandleForget: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	forgetBody := decodeBody(t, rec)
	if forgetBody["status"] != "ok" {
		t.Errorf("expected status ok, got %v", forgetBody["status"])
	}

	// List should return empty.
	rec = getJSON(mh.HandleList, "/imprint/memories")
	if rec.Code != http.StatusOK {
		t.Fatalf("HandleList: expected 200, got %d", rec.Code)
	}

	body = decodeBody(t, rec)
	memories := body["memories"]
	if memories != nil {
		if memList, ok := memories.([]any); ok && len(memList) > 0 {
			t.Errorf("expected no memories after forget, got %d", len(memList))
		}
	}
}

func TestMemoryHandler_Evolve(t *testing.T) {
	_, _, mh := setupTestHandlers(t)

	// Remember.
	rec := postJSON(mh.HandleRemember, "/imprint/remember", map[string]any{
		"type":     "preference",
		"title":    "Indentation",
		"content":  "Use tabs",
		"strength": 5,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("HandleRemember: expected 201, got %d", rec.Code)
	}

	body := decodeBody(t, rec)
	mem := body["memory"].(map[string]any)
	memID := mem["id"].(string)

	// Evolve.
	rec = postJSON(mh.HandleEvolve, "/imprint/evolve", map[string]any{
		"id":       memID,
		"content":  "Use 2 spaces instead of tabs",
		"strength": 9,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("HandleEvolve: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	body = decodeBody(t, rec)
	evolved := body["memory"].(map[string]any)
	if evolved["content"] != "Use 2 spaces instead of tabs" {
		t.Errorf("evolved content mismatch: %v", evolved["content"])
	}
	if evolved["id"] == memID {
		t.Error("evolved memory should have a new ID")
	}
	// Version should be 2.
	if v, ok := evolved["version"].(float64); !ok || int(v) != 2 {
		t.Errorf("expected version 2, got %v", evolved["version"])
	}

	// List should return only the evolved version.
	rec = getJSON(mh.HandleList, "/imprint/memories")
	if rec.Code != http.StatusOK {
		t.Fatalf("HandleList: expected 200, got %d", rec.Code)
	}

	body = decodeBody(t, rec)
	memories := body["memories"].([]any)
	if len(memories) != 1 {
		t.Fatalf("expected 1 latest memory, got %d", len(memories))
	}
	latest := memories[0].(map[string]any)
	if latest["content"] != "Use 2 spaces instead of tabs" {
		t.Errorf("latest memory should be evolved version, got %v", latest["content"])
	}
}

// strPtr returns a pointer to the given string.
func strPtr(s string) *string {
	return &s
}
