package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func makeRequest(method string, id any, params any) JSONRPCRequest {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
	}
	if params != nil {
		raw, _ := json.Marshal(params)
		req.Params = raw
	}
	return req
}

func TestMCPServer_Initialize(t *testing.T) {
	s := NewServer("http://localhost:3111", "", "core")
	ctx := context.Background()

	resp := s.dispatch(ctx, makeRequest("initialize", 1, nil))

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if resp.ID != 1 {
		t.Errorf("expected ID 1, got %v", resp.ID)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	// With no client-supplied protocolVersion the server falls back to the
	// newest version it supports. Just assert it's one of the supported set.
	got := result["protocolVersion"]
	known := map[string]bool{}
	for _, v := range supportedProtocolVersions {
		known[v] = true
	}
	if s, ok := got.(string); !ok || !known[s] {
		t.Errorf("expected protocolVersion in %v, got %v", supportedProtocolVersions, got)
	}

	caps, ok := result["capabilities"].(map[string]any)
	if !ok {
		t.Fatal("capabilities is not a map")
	}
	for _, key := range []string{"tools", "resources", "prompts"} {
		if caps[key] == nil {
			t.Errorf("expected capability %q", key)
		}
	}

	info, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatal("serverInfo is not a map")
	}
	if info["name"] != "imprint" {
		t.Errorf("expected server name imprint, got %v", info["name"])
	}
}

func TestMCPServer_InitializeNegotiatesClientVersion(t *testing.T) {
	s := NewServer("http://localhost:3111", "", "core")
	ctx := context.Background()

	for _, want := range supportedProtocolVersions {
		params, _ := json.Marshal(map[string]any{"protocolVersion": want})
		req := JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "initialize", Params: params}
		resp := s.dispatch(ctx, req)
		if resp.Error != nil {
			t.Fatalf("%s: unexpected error: %v", want, resp.Error)
		}
		result := resp.Result.(map[string]any)
		if got := result["protocolVersion"]; got != want {
			t.Errorf("client asked %s, server returned %v", want, got)
		}
	}
}

func TestMCPServer_ToolsList(t *testing.T) {
	s := NewServer("http://localhost:3111", "", "core")
	ctx := context.Background()

	resp := s.dispatch(ctx, makeRequest("tools/list", 2, nil))

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}

	tools, ok := result["tools"].([]MCPTool)
	if !ok {
		t.Fatal("tools is not []MCPTool")
	}
	if len(tools) != 8 {
		t.Errorf("expected 8 core tools, got %d", len(tools))
	}

	expectedNames := map[string]bool{
		"memory_recall":      false,
		"memory_save":        false,
		"memory_search":      false,
		"memory_forget":      false,
		"memory_context":     false,
		"memory_profiles":    false,
		"memory_patterns":    false,
		"memory_graph_query": false,
	}
	for _, tool := range tools {
		if _, exists := expectedNames[tool.Name]; !exists {
			t.Errorf("unexpected tool: %s", tool.Name)
		}
		expectedNames[tool.Name] = true
	}
	for name, found := range expectedNames {
		if !found {
			t.Errorf("missing tool: %s", name)
		}
	}
}

func TestMCPServer_ToolsCall_Search(t *testing.T) {
	var receivedPath string
	var receivedBody map[string]any

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&receivedBody)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"memories": []map[string]any{
				{"id": "mem-1", "title": "Test memory", "score": 0.95},
			},
		})
	}))
	defer ts.Close()

	s := NewServer(ts.URL, "test-secret", "core")
	ctx := context.Background()

	resp := s.dispatch(ctx, makeRequest("tools/call", 3, map[string]any{
		"name":      "memory_search",
		"arguments": map[string]any{"query": "test query", "limit": 5},
	}))

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if receivedPath != "/imprint/search" {
		t.Errorf("expected path /imprint/search, got %s", receivedPath)
	}
	if receivedBody["query"] != "test query" {
		t.Errorf("expected query 'test query', got %v", receivedBody["query"])
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	content, ok := result["content"].([]map[string]any)
	if !ok {
		t.Fatal("content is not []map[string]any")
	}
	if len(content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(content))
	}
	if content[0]["type"] != "text" {
		t.Errorf("expected content type text, got %v", content[0]["type"])
	}
	// Verify the response text contains the memory data
	text, ok := content[0]["text"].(string)
	if !ok || text == "" {
		t.Error("expected non-empty text content")
	}
}

func TestMCPServer_ToolsCall_Remember(t *testing.T) {
	var receivedPath string
	var receivedBody map[string]any

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&receivedBody)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":     "mem-new",
			"status": "saved",
		})
	}))
	defer ts.Close()

	s := NewServer(ts.URL, "", "core")
	ctx := context.Background()

	resp := s.dispatch(ctx, makeRequest("tools/call", 4, map[string]any{
		"name": "memory_save",
		"arguments": map[string]any{
			"title":   "Important pattern",
			"content": "Always use parameterized queries",
			"type":    "pattern",
		},
	}))

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if receivedPath != "/imprint/remember" {
		t.Errorf("expected path /imprint/remember, got %s", receivedPath)
	}
	if receivedBody["title"] != "Important pattern" {
		t.Errorf("expected title 'Important pattern', got %v", receivedBody["title"])
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	if result["isError"] != nil {
		t.Errorf("expected no isError flag, got %v", result["isError"])
	}
}

func TestMCPServer_ToolsCall_UnknownTool(t *testing.T) {
	s := NewServer("http://localhost:3111", "", "core")
	ctx := context.Background()

	resp := s.dispatch(ctx, makeRequest("tools/call", 5, map[string]any{
		"name":      "nonexistent_tool",
		"arguments": map[string]any{},
	}))

	if resp.Error != nil {
		t.Fatal("expected no JSON-RPC error (tool errors are returned as content)")
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	isError, ok := result["isError"].(bool)
	if !ok || !isError {
		t.Error("expected isError=true for unknown tool")
	}
}

func TestMCPServer_ResourcesList(t *testing.T) {
	s := NewServer("http://localhost:3111", "", "core")
	ctx := context.Background()

	resp := s.dispatch(ctx, makeRequest("resources/list", 6, nil))

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}

	resources, ok := result["resources"].([]map[string]any)
	if !ok {
		t.Fatal("resources is not []map[string]any")
	}
	if len(resources) != 2 {
		t.Errorf("expected 2 resources, got %d", len(resources))
	}

	foundStatus := false
	foundGraph := false
	for _, r := range resources {
		switch r["uri"] {
		case "imprint://status":
			foundStatus = true
		case "imprint://graph/stats":
			foundGraph = true
		}
	}
	if !foundStatus {
		t.Error("missing imprint://status resource")
	}
	if !foundGraph {
		t.Error("missing imprint://graph/stats resource")
	}
}

func TestMCPServer_PromptsList(t *testing.T) {
	s := NewServer("http://localhost:3111", "", "core")
	ctx := context.Background()

	resp := s.dispatch(ctx, makeRequest("prompts/list", 7, nil))

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}

	prompts, ok := result["prompts"].([]map[string]any)
	if !ok {
		t.Fatal("prompts is not []map[string]any")
	}
	if len(prompts) != 3 {
		t.Errorf("expected 3 prompts, got %d", len(prompts))
	}

	expectedPrompts := map[string]bool{
		"recall_context":  false,
		"session_handoff": false,
		"detect_patterns": false,
	}
	for _, p := range prompts {
		name, _ := p["name"].(string)
		if _, exists := expectedPrompts[name]; exists {
			expectedPrompts[name] = true
		}
	}
	for name, found := range expectedPrompts {
		if !found {
			t.Errorf("missing prompt: %s", name)
		}
	}
}

func TestMCPServer_UnknownMethod(t *testing.T) {
	s := NewServer("http://localhost:3111", "", "core")
	ctx := context.Background()

	resp := s.dispatch(ctx, makeRequest("bogus/method", 8, nil))

	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("expected error code -32601, got %d", resp.Error.Code)
	}
	if resp.Error.Message == "" {
		t.Error("expected non-empty error message")
	}
}
