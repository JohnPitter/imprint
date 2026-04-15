package pipeline

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"imprint/internal/llm"
	"imprint/internal/store"
	"imprint/internal/types"
)

// mockLLMProvider is a configurable fake LLM provider for pipeline tests.
type mockLLMProvider struct {
	response string
	err      error
	calls    int
}

func (m *mockLLMProvider) Name() string      { return "mock" }
func (m *mockLLMProvider) Available() bool    { return true }
func (m *mockLLMProvider) Complete(_ context.Context, _ llm.CompletionRequest) (string, error) {
	m.calls++
	return m.response, m.err
}

// ---------------------------------------------------------------------------
// XML parsing tests
// ---------------------------------------------------------------------------

func TestGetXMLTag(t *testing.T) {
	input := `<observation><type>file_operation</type><title>Edit app.go</title></observation>`

	got := getXMLTag(input, "type")
	if got != "file_operation" {
		t.Fatalf("expected 'file_operation', got %q", got)
	}

	got = getXMLTag(input, "title")
	if got != "Edit app.go" {
		t.Fatalf("expected 'Edit app.go', got %q", got)
	}
}

func TestGetXMLTag_NotFound(t *testing.T) {
	input := `<observation><title>Hello</title></observation>`

	got := getXMLTag(input, "missing")
	if got != "" {
		t.Fatalf("expected empty string for missing tag, got %q", got)
	}
}

func TestGetXMLTag_Multiline(t *testing.T) {
	input := `<narrative>
Line one.
Line two.
</narrative>`

	got := getXMLTag(input, "narrative")
	if got != "Line one.\nLine two." {
		t.Fatalf("expected multiline content, got %q", got)
	}
}

func TestGetXMLChildren(t *testing.T) {
	input := `<facts><fact>Added db.Open call</fact><fact>Changed error handling</fact></facts>`

	got := getXMLChildren(input, "facts", "fact")
	if len(got) != 2 {
		t.Fatalf("expected 2 facts, got %d", len(got))
	}
	if got[0] != "Added db.Open call" {
		t.Fatalf("expected first fact 'Added db.Open call', got %q", got[0])
	}
	if got[1] != "Changed error handling" {
		t.Fatalf("expected second fact 'Changed error handling', got %q", got[1])
	}
}

func TestGetXMLChildren_Empty(t *testing.T) {
	input := `<facts></facts>`

	got := getXMLChildren(input, "facts", "fact")
	if got != nil {
		t.Fatalf("expected nil for empty parent, got %v", got)
	}
}

func TestGetXMLInt(t *testing.T) {
	input := `<importance>7</importance>`

	got := getXMLInt(input, "importance")
	if got != 7 {
		t.Fatalf("expected 7, got %d", got)
	}
}

func TestGetXMLInt_Invalid(t *testing.T) {
	input := `<importance>high</importance>`

	got := getXMLInt(input, "importance")
	if got != 0 {
		t.Fatalf("expected 0 for non-numeric, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// Compressor tests
// ---------------------------------------------------------------------------

const mockCompressResponse = `<observation>
  <type>file_operation</type>
  <title>Edit app.go</title>
  <subtitle>Updated startup logic</subtitle>
  <narrative>Modified the application startup to include database initialization.</narrative>
  <facts><fact>Added db.Open call</fact><fact>Changed error handling</fact></facts>
  <concepts><concept>go</concept><concept>startup</concept></concepts>
  <files><file>app.go</file></files>
  <importance>7</importance>
</observation>`

func TestCompressor_Compress(t *testing.T) {
	mock := &mockLLMProvider{response: mockCompressResponse}
	comp := NewCompressor(mock)

	toolName := "Edit"
	raw := &store.RawObservationRow{
		ID:         "raw-001",
		SessionID:  "sess-001",
		Timestamp:  time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC),
		HookType:   "tool_use",
		ToolName:   &toolName,
		ToolInput:  json.RawMessage(`{"path":"app.go"}`),
		ToolOutput: json.RawMessage(`{"content":"package main"}`),
	}

	compressed, err := comp.Compress(context.Background(), raw)
	if err != nil {
		t.Fatalf("Compress() error: %v", err)
	}

	if compressed.Type != "file_operation" {
		t.Fatalf("expected type 'file_operation', got %q", compressed.Type)
	}
	if compressed.Title != "Edit app.go" {
		t.Fatalf("expected title 'Edit app.go', got %q", compressed.Title)
	}
	if compressed.Subtitle == nil || *compressed.Subtitle != "Updated startup logic" {
		t.Fatalf("expected subtitle 'Updated startup logic', got %v", compressed.Subtitle)
	}
	if compressed.Narrative == nil || *compressed.Narrative != "Modified the application startup to include database initialization." {
		t.Fatal("unexpected narrative value")
	}
	if len(compressed.Facts) != 2 {
		t.Fatalf("expected 2 facts, got %d", len(compressed.Facts))
	}
	if len(compressed.Concepts) != 2 {
		t.Fatalf("expected 2 concepts, got %d", len(compressed.Concepts))
	}
	if len(compressed.Files) != 1 || compressed.Files[0] != "app.go" {
		t.Fatalf("expected files [app.go], got %v", compressed.Files)
	}
	if compressed.Importance != 7 {
		t.Fatalf("expected importance 7, got %d", compressed.Importance)
	}
	if compressed.Confidence != 0.8 {
		t.Fatalf("expected confidence 0.8, got %f", compressed.Confidence)
	}
	if compressed.SessionID != "sess-001" {
		t.Fatalf("expected session ID 'sess-001', got %q", compressed.SessionID)
	}
	if compressed.SourceObservationID == nil || *compressed.SourceObservationID != "raw-001" {
		t.Fatal("expected source observation ID to be 'raw-001'")
	}
	if mock.calls != 1 {
		t.Fatalf("expected 1 LLM call, got %d", mock.calls)
	}
}

// ---------------------------------------------------------------------------
// Summarizer tests
// ---------------------------------------------------------------------------

const mockSummarizeResponse = `<summary>
  <title>Database initialization refactor</title>
  <narrative>Refactored the app startup to properly initialize the database with WAL mode and run migrations on start.</narrative>
  <key_decisions><decision>Used WAL mode for concurrency</decision></key_decisions>
  <files_modified><file>app.go</file><file>store/db.go</file></files_modified>
  <concepts><concept>sqlite</concept><concept>migrations</concept></concepts>
</summary>`

func TestSummarizer_Summarize(t *testing.T) {
	mock := &mockLLMProvider{response: mockSummarizeResponse}
	sum := NewSummarizer(mock)

	narrative := "Modified the application startup."
	observations := []store.CompressedObservationRow{
		{
			ID:        "cobs-001",
			SessionID: "sess-001",
			Type:      "file_operation",
			Title:     "Edit app.go",
			Narrative: &narrative,
		},
	}

	summary, err := sum.Summarize(context.Background(), "sess-001", "myproject", observations)
	if err != nil {
		t.Fatalf("Summarize() error: %v", err)
	}

	if summary.Title != "Database initialization refactor" {
		t.Fatalf("expected title 'Database initialization refactor', got %q", summary.Title)
	}
	if summary.SessionID != "sess-001" {
		t.Fatalf("expected session ID 'sess-001', got %q", summary.SessionID)
	}
	if summary.Project != "myproject" {
		t.Fatalf("expected project 'myproject', got %q", summary.Project)
	}
	if summary.ObservationCount != 1 {
		t.Fatalf("expected observation count 1, got %d", summary.ObservationCount)
	}
	if summary.Narrative == "" {
		t.Fatal("expected non-empty narrative")
	}

	// Verify JSON arrays are properly marshaled.
	var decisions []string
	if err := json.Unmarshal([]byte(summary.KeyDecisions), &decisions); err != nil {
		t.Fatalf("failed to unmarshal key decisions: %v", err)
	}
	if len(decisions) != 1 || decisions[0] != "Used WAL mode for concurrency" {
		t.Fatalf("unexpected key decisions: %v", decisions)
	}

	var files []string
	if err := json.Unmarshal([]byte(summary.FilesModified), &files); err != nil {
		t.Fatalf("failed to unmarshal files modified: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files modified, got %d", len(files))
	}

	var concepts []string
	if err := json.Unmarshal([]byte(summary.Concepts), &concepts); err != nil {
		t.Fatalf("failed to unmarshal concepts: %v", err)
	}
	if len(concepts) != 2 {
		t.Fatalf("expected 2 concepts, got %d", len(concepts))
	}

	if mock.calls != 1 {
		t.Fatalf("expected 1 LLM call, got %d", mock.calls)
	}
}

// ---------------------------------------------------------------------------
// Worker tests
// ---------------------------------------------------------------------------

// setupWorkerTestDB creates a real SQLite DB for worker integration tests.
func setupWorkerTestDB(t *testing.T) (*store.DB, *store.ObservationStore) {
	t.Helper()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Create a session for FK constraints.
	sessStore := store.NewSessionStore(db)
	sess := &store.SessionRow{
		ID:        "sess-worker",
		Project:   "test-project",
		Cwd:       "/workspace/test",
		StartedAt: time.Now().UTC(),
		Status:    types.SessionActive,
		Tags:      []string{},
	}
	if err := sessStore.Create(sess); err != nil {
		t.Fatalf("failed to create test session: %v", err)
	}

	obsStore := store.NewObservationStore(db)
	return db, obsStore
}

func TestWorker_ProcessesJobs(t *testing.T) {
	_, obsStore := setupWorkerTestDB(t)

	mock := &mockLLMProvider{response: mockCompressResponse}
	comp := NewCompressor(mock)
	worker := NewWorker(comp, obsStore, 2)

	// Submit raw observations.
	toolName := "Edit"
	for i := 0; i < 3; i++ {
		raw := &store.RawObservationRow{
			ID:         "raw-w-" + string(rune('a'+i)),
			SessionID:  "sess-worker",
			Timestamp:  time.Now().UTC(),
			HookType:   "tool_use",
			ToolName:   &toolName,
			ToolInput:  json.RawMessage(`{"path":"app.go"}`),
			ToolOutput: json.RawMessage(`{"content":"ok"}`),
		}
		// Insert raw observation into DB first (for referential integrity if needed).
		if err := obsStore.CreateRaw(raw); err != nil {
			t.Fatalf("failed to create raw observation: %v", err)
		}
		worker.Submit(raw)
	}

	// Give workers time to process.
	time.Sleep(200 * time.Millisecond)
	worker.Stop()

	// Verify compressed observations were created.
	compressed, err := obsStore.ListCompressed("sess-worker", 100, 0)
	if err != nil {
		t.Fatalf("ListCompressed error: %v", err)
	}
	if len(compressed) != 3 {
		t.Fatalf("expected 3 compressed observations, got %d", len(compressed))
	}
	if mock.calls != 3 {
		t.Fatalf("expected 3 LLM calls, got %d", mock.calls)
	}
}

func TestWorker_Stop(t *testing.T) {
	_, obsStore := setupWorkerTestDB(t)

	mock := &mockLLMProvider{response: mockCompressResponse}
	comp := NewCompressor(mock)
	worker := NewWorker(comp, obsStore, 2)

	// Stop immediately without submitting any jobs.
	done := make(chan struct{})
	go func() {
		worker.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Graceful shutdown succeeded.
	case <-time.After(5 * time.Second):
		t.Fatal("worker.Stop() did not complete within 5 seconds")
	}
}
