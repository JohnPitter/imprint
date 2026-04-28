package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestNewServer_DefaultsApplied(t *testing.T) {
	s := NewServer("", "", "")
	if s.baseURL != "http://localhost:3111" {
		t.Errorf("default baseURL = %q", s.baseURL)
	}
	if s.toolMode != "core" {
		t.Errorf("default toolMode = %q", s.toolMode)
	}
}

func TestServer_Get_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("missing/wrong Authorization header")
		}
		_, _ = w.Write([]byte(`{"ok":true,"count":3}`))
	}))
	defer srv.Close()

	s := NewServer(srv.URL, "secret", "core")
	res, err := s.get(context.Background(), "/imprint/profile")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("result not a map: %T", res)
	}
	if m["ok"] != true {
		t.Errorf("ok = %v", m["ok"])
	}
}

func TestServer_Get_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	s := NewServer(srv.URL, "", "core")
	_, err := s.get(context.Background(), "/x")
	if err == nil {
		t.Fatal("expected error on invalid JSON")
	}
}

func TestServer_Get_RequestFail(t *testing.T) {
	s := NewServer("http://127.0.0.1:1", "", "core") // port 1 unlikely to be listening
	_, err := s.get(context.Background(), "/x")
	if err == nil {
		t.Fatal("expected network error")
	}
}

func TestServer_Post_EchoesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q", ct)
		}
		body, _ := io.ReadAll(r.Body)
		// Echo what was sent
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	s := NewServer(srv.URL, "", "core")
	res, err := s.post(context.Background(), "/imprint/search", map[string]any{"query": "hello"})
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("result not a map: %T", res)
	}
	if m["query"] != "hello" {
		t.Errorf("echo query = %v", m["query"])
	}
}

func TestServer_Post_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>"))
	}))
	defer srv.Close()

	s := NewServer(srv.URL, "", "core")
	_, err := s.post(context.Background(), "/x", map[string]any{"k": "v"})
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestAllTools_Superset(t *testing.T) {
	core := CoreTools()
	all := AllTools()
	if len(all) < len(core) {
		t.Errorf("AllTools(%d) should be >= CoreTools(%d)", len(all), len(core))
	}
	// Every core tool name should appear in all.
	allNames := make(map[string]bool)
	for _, tool := range all {
		allNames[tool.Name] = true
	}
	for _, tool := range core {
		if !allNames[tool.Name] {
			t.Errorf("core tool %q missing from AllTools", tool.Name)
		}
	}
}

func TestHandleToolsList_AllMode(t *testing.T) {
	s := NewServer("http://localhost:3111", "", "all")
	resp := s.dispatch(context.Background(), makeRequest("tools/list", 3, nil))
	if resp.Error != nil {
		t.Fatalf("error: %v", resp.Error)
	}
	result := resp.Result.(map[string]any)
	tools := result["tools"].([]MCPTool)
	if len(tools) != len(AllTools()) {
		t.Errorf("all mode should return %d tools, got %d", len(AllTools()), len(tools))
	}
}

func TestCallTool_UnknownTool(t *testing.T) {
	s := NewServer("http://localhost:3111", "", "core")
	_, err := s.callTool(context.Background(), "memory_bogus", map[string]any{})
	if err == nil {
		t.Fatal("expected unknown tool error")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("error message: %v", err)
	}
}

func TestCallTool_RoutesPaths(t *testing.T) {
	var lastPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	s := NewServer(srv.URL, "", "core")
	cases := map[string]string{
		"memory_recall":      "/imprint/search",
		"memory_search":      "/imprint/search",
		"memory_save":        "/imprint/remember",
		"memory_forget":      "/imprint/forget",
		"memory_context":     "/imprint/context",
		"memory_profiles":    "/imprint/graph/stats",
		"memory_patterns":    "/imprint/insights",
		"memory_graph_query": "/imprint/graph/query",
	}
	for tool, wantPath := range cases {
		if _, err := s.callTool(context.Background(), tool, map[string]any{}); err != nil {
			t.Errorf("callTool(%s): %v", tool, err)
			continue
		}
		if lastPath != wantPath {
			t.Errorf("tool %s routed to %q, want %q", tool, lastPath, wantPath)
		}
	}
}

func TestWriteResponse_EmitsJSONL(t *testing.T) {
	// Redirect stdout to a temp file to capture output.
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	s := NewServer("http://localhost:3111", "", "core")
	s.writeResponse(successResponse(42, map[string]any{"ok": true}))

	_ = w.Close()
	os.Stdout = origStdout

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	if !strings.HasSuffix(string(got), "\n") {
		t.Errorf("output should end with newline, got %q", got)
	}
	var resp JSONRPCResponse
	if err := json.Unmarshal(got[:len(got)-1], &resp); err != nil {
		t.Fatalf("unmarshal emitted JSON: %v", err)
	}
	if resp.ID != float64(42) && resp.ID != 42 {
		t.Errorf("ID round-trip: %v", resp.ID)
	}
}

func TestRun_ParseErrorAndValidRequest(t *testing.T) {
	// Feed stdin with one garbage line + one valid request; Run should respond to both
	// and then return when EOF is reached.
	origStdin := os.Stdin
	origStdout := os.Stdout
	rIn, wIn, _ := os.Pipe()
	rOut, wOut, _ := os.Pipe()
	os.Stdin = rIn
	os.Stdout = wOut

	defer func() {
		os.Stdin = origStdin
		os.Stdout = origStdout
	}()

	s := NewServer("http://localhost:3111", "", "core")

	// Write a garbage line + a valid initialize request, then close stdin.
	go func() {
		_, _ = wIn.Write([]byte("not-json\n"))
		_, _ = wIn.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n"))
		_ = wIn.Close()
	}()

	if err := s.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	_ = wOut.Close()
	out, _ := io.ReadAll(rOut)
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 response lines, got %d: %q", len(lines), out)
	}

	// First must be a parse error.
	var parseErr JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[0]), &parseErr); err != nil {
		t.Fatalf("first line not JSON: %v", err)
	}
	if parseErr.Error == nil || parseErr.Error.Code != -32700 {
		t.Errorf("expected parse error -32700, got %+v", parseErr.Error)
	}

	// Second must be successful initialize.
	var initResp JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[1]), &initResp); err != nil {
		t.Fatalf("second line not JSON: %v", err)
	}
	if initResp.Error != nil {
		t.Errorf("unexpected error on initialize: %+v", initResp.Error)
	}
}
