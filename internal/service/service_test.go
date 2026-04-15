package service

import (
	"encoding/json"
	"testing"
	"time"

	"imprint/internal/store"
	"imprint/internal/types"
)

func setupTestContainer(t *testing.T) *Container {
	t.Helper()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewContainer(db)
}

// ---------------------------------------------------------------------------
// SessionService
// ---------------------------------------------------------------------------

func TestSessionService_StartAndEnd(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewSessionService(c)

	session, blocks, err := svc.Start("ses_test1", "myproject", "/tmp/proj")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if session == nil {
		t.Fatal("Start returned nil session")
	}
	if session.ID != "ses_test1" {
		t.Errorf("expected session ID ses_test1, got %s", session.ID)
	}
	if session.Project != "myproject" {
		t.Errorf("expected project myproject, got %s", session.Project)
	}
	if session.Status != types.SessionActive {
		t.Errorf("expected status active, got %s", session.Status)
	}
	// Context blocks may be empty for a fresh database — that is valid.
	_ = blocks

	// End the session.
	if err := svc.End("ses_test1"); err != nil {
		t.Fatalf("End: %v", err)
	}

	// Verify the session is now completed.
	row, err := c.Sessions.GetByID("ses_test1")
	if err != nil {
		t.Fatalf("GetByID after End: %v", err)
	}
	if row.Status != types.SessionCompleted {
		t.Errorf("expected status completed, got %s", row.Status)
	}
	if row.EndedAt == nil {
		t.Error("expected EndedAt to be set after End")
	}
}

func TestSessionService_StartGeneratesID(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewSessionService(c)

	session, _, err := svc.Start("", "proj", "/tmp")
	if err != nil {
		t.Fatalf("Start with empty ID: %v", err)
	}
	if session.ID == "" {
		t.Fatal("expected generated session ID, got empty string")
	}
	if len(session.ID) < 4 {
		t.Errorf("generated ID too short: %s", session.ID)
	}
	// Should have the ses_ prefix.
	if session.ID[:4] != "ses_" {
		t.Errorf("expected ses_ prefix, got %s", session.ID)
	}
}

func TestSessionService_StartRequiresProject(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewSessionService(c)

	_, _, err := svc.Start("ses_x", "", "/tmp")
	if err == nil {
		t.Fatal("expected error when project is empty")
	}
}

func TestSessionService_List(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewSessionService(c)

	// Create 3 sessions.
	for i := 0; i < 3; i++ {
		_, _, err := svc.Start("", "proj-list", "/tmp")
		if err != nil {
			t.Fatalf("Start session %d: %v", i, err)
		}
	}

	// List all for this project.
	sessions, err := svc.List("proj-list", 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}

	// Pagination: limit 2.
	sessions, err = svc.List("proj-list", 2, 0)
	if err != nil {
		t.Fatalf("List with limit: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions with limit, got %d", len(sessions))
	}

	// Pagination: offset 2 should return 1.
	sessions, err = svc.List("proj-list", 10, 2)
	if err != nil {
		t.Fatalf("List with offset: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("expected 1 session with offset 2, got %d", len(sessions))
	}
}

// ---------------------------------------------------------------------------
// ObserveService
// ---------------------------------------------------------------------------

func makePayload(sessionID string, hookType types.HookType, toolName *string) *types.HookPayload {
	return &types.HookPayload{
		SessionID: sessionID,
		HookType:  hookType,
		ToolName:  toolName,
		Timestamp: time.Now(),
	}
}

func strPtr(s string) *string { return &s }

func TestObserveService_Observe(t *testing.T) {
	c := setupTestContainer(t)
	// Create a session first so IncrementObservationCount works.
	sessionSvc := NewSessionService(c)
	sessionSvc.Start("ses_obs1", "proj", "/tmp")

	svc := NewObserveService(c, 500, 8000)

	payload := makePayload("ses_obs1", types.HookPostToolUse, strPtr("Write"))
	payload.ToolInput = json.RawMessage(`{"path":"/tmp/test.go"}`)
	payload.ToolOutput = json.RawMessage(`{"ok":true}`)

	obs, err := svc.Observe(payload)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if obs == nil {
		t.Fatal("Observe returned nil observation")
	}
	if obs.SessionID != "ses_obs1" {
		t.Errorf("expected session_id ses_obs1, got %s", obs.SessionID)
	}
	if obs.HookType != string(types.HookPostToolUse) {
		t.Errorf("expected hook_type post_tool_use, got %s", obs.HookType)
	}

	// Verify it was stored.
	raw, err := svc.ListRaw("ses_obs1", 10, 0)
	if err != nil {
		t.Fatalf("ListRaw: %v", err)
	}
	if len(raw) != 1 {
		t.Errorf("expected 1 raw observation, got %d", len(raw))
	}
}

func TestObserveService_ObserveStripsSecrets(t *testing.T) {
	c := setupTestContainer(t)
	sessionSvc := NewSessionService(c)
	sessionSvc.Start("ses_secret", "proj", "/tmp")

	svc := NewObserveService(c, 500, 8000)

	payload := makePayload("ses_secret", types.HookPostToolUse, strPtr("Bash"))
	// Include a fake API key that should be stripped.
	payload.ToolOutput = json.RawMessage(`{"result":"key is sk-proj-ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890abcd"}`)

	obs, err := svc.Observe(payload)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if obs == nil {
		t.Fatal("Observe returned nil observation")
	}

	// The raw field should have the secret redacted.
	rawStr := string(obs.Raw)
	if contains(rawStr, "sk-proj-ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890abcd") {
		t.Error("raw field still contains unredacted API key")
	}
	if !contains(rawStr, "[REDACTED]") {
		t.Error("raw field does not contain [REDACTED] marker")
	}

	// ToolOutput should also be scrubbed.
	outputStr := string(obs.ToolOutput)
	if contains(outputStr, "sk-proj-ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890abcd") {
		t.Error("toolOutput still contains unredacted API key")
	}
}

func TestObserveService_ObserveDedup(t *testing.T) {
	c := setupTestContainer(t)
	sessionSvc := NewSessionService(c)
	sessionSvc.Start("ses_dedup", "proj", "/tmp")

	svc := NewObserveService(c, 500, 8000)

	payload := makePayload("ses_dedup", types.HookPostToolUse, strPtr("Read"))
	payload.ToolInput = json.RawMessage(`{"path":"/tmp/same.go"}`)

	obs1, err := svc.Observe(payload)
	if err != nil {
		t.Fatalf("Observe first: %v", err)
	}
	if obs1 == nil {
		t.Fatal("first observe should not be nil")
	}

	// Same payload again — should be deduped.
	obs2, err := svc.Observe(payload)
	if err != nil {
		t.Fatalf("Observe second: %v", err)
	}
	if obs2 != nil {
		t.Error("second observe should return nil (duplicate)")
	}
}

func TestObserveService_ObserveRateLimit(t *testing.T) {
	c := setupTestContainer(t)
	sessionSvc := NewSessionService(c)
	sessionSvc.Start("ses_rate", "proj", "/tmp")

	// Allow only 2 observations per session.
	svc := NewObserveService(c, 2, 8000)

	for i := 0; i < 2; i++ {
		p := makePayload("ses_rate", types.HookPostToolUse, strPtr("Bash"))
		p.ToolInput = json.RawMessage(`{"cmd":"echo ` + string(rune('a'+i)) + `"}`)

		obs, err := svc.Observe(p)
		if err != nil {
			t.Fatalf("Observe #%d: %v", i, err)
		}
		if obs == nil {
			t.Fatalf("Observe #%d should not be nil", i)
		}
	}

	// 3rd observation should be silently skipped (rate limit).
	p := makePayload("ses_rate", types.HookPostToolUse, strPtr("Bash"))
	p.ToolInput = json.RawMessage(`{"cmd":"echo c"}`)
	obs, err := svc.Observe(p)
	if err != nil {
		t.Fatalf("Observe #3: %v", err)
	}
	if obs != nil {
		t.Error("3rd observe should return nil (rate limited)")
	}
}

// ---------------------------------------------------------------------------
// RememberService
// ---------------------------------------------------------------------------

func TestRememberService_RememberAndList(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewRememberService(c)

	mem, err := svc.Remember(types.MemPattern, "Use context managers", "Always use context managers for resources", []string{"python", "resources"}, []string{"main.py"}, 7)
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}
	if mem == nil {
		t.Fatal("Remember returned nil")
	}
	if mem.Title != "Use context managers" {
		t.Errorf("expected title 'Use context managers', got %s", mem.Title)
	}
	if mem.Strength != 7 {
		t.Errorf("expected strength 7, got %d", mem.Strength)
	}
	if mem.Version != 1 {
		t.Errorf("expected version 1, got %d", mem.Version)
	}

	// List should return it.
	memories, err := svc.List("", 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(memories) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(memories))
	}
	if memories[0].ID != mem.ID {
		t.Errorf("listed memory ID mismatch: %s vs %s", memories[0].ID, mem.ID)
	}
}

func TestRememberService_Forget(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewRememberService(c)

	mem, err := svc.Remember(types.MemFact, "Go is compiled", "Go compiles to native code", nil, nil, 5)
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}

	if err := svc.Forget(mem.ID); err != nil {
		t.Fatalf("Forget: %v", err)
	}

	// List should return empty (is_latest=0 after forget).
	memories, err := svc.List("", 10, 0)
	if err != nil {
		t.Fatalf("List after forget: %v", err)
	}
	if len(memories) != 0 {
		t.Errorf("expected 0 memories after forget, got %d", len(memories))
	}
}

func TestRememberService_Evolve(t *testing.T) {
	c := setupTestContainer(t)
	svc := NewRememberService(c)

	original, err := svc.Remember(types.MemPreference, "Tab size", "Use 4 spaces", nil, nil, 6)
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}

	evolved, err := svc.Evolve(original.ID, "Use 2 spaces (updated preference)", 8)
	if err != nil {
		t.Fatalf("Evolve: %v", err)
	}
	if evolved == nil {
		t.Fatal("Evolve returned nil")
	}
	if evolved.Content != "Use 2 spaces (updated preference)" {
		t.Errorf("evolved content mismatch: %s", evolved.Content)
	}
	if evolved.Version != 2 {
		t.Errorf("expected version 2, got %d", evolved.Version)
	}
	if evolved.Strength != 8 {
		t.Errorf("expected strength 8, got %d", evolved.Strength)
	}
	if evolved.Title != original.Title {
		t.Errorf("evolved title should match original: got %s, want %s", evolved.Title, original.Title)
	}

	// Only the new version should appear in list (is_latest).
	memories, err := svc.List("", 50, 0)
	if err != nil {
		t.Fatalf("List after evolve: %v", err)
	}
	if len(memories) != 1 {
		t.Fatalf("expected 1 latest memory, got %d", len(memories))
	}
	if memories[0].ID != evolved.ID {
		t.Errorf("latest memory should be evolved: got %s, want %s", memories[0].ID, evolved.ID)
	}
}

// contains is a small helper to check substring presence.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexSubstring(s, substr) >= 0)
}

func indexSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
