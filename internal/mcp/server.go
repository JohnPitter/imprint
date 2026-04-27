package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Server is the MCP JSON-RPC server that proxies tool calls to the imprint HTTP API.
type Server struct {
	baseURL  string
	secret   string
	client   *http.Client
	toolMode string // "core" or "all"
}

// NewServer creates a new MCP server.
func NewServer(baseURL, secret, toolMode string) *Server {
	if baseURL == "" {
		baseURL = "http://localhost:3111"
	}
	if toolMode == "" {
		toolMode = "core"
	}
	return &Server{
		baseURL:  baseURL,
		secret:   secret,
		client:   &http.Client{Timeout: 30 * time.Second},
		toolMode: toolMode,
	}
}

// Run starts the MCP server, reading from stdin and writing to stdout.
func (s *Server) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeResponse(errorResponse(nil, -32700, "parse error"))
			continue
		}

		// JSON-RPC notifications have no id and MUST NOT receive a response.
		// Claude Code sends notifications/initialized after the handshake.
		if req.ID == nil {
			s.handleNotification(ctx, req)
			continue
		}

		resp := s.dispatch(ctx, req)
		s.writeResponse(resp)
	}

	return scanner.Err()
}

// handleNotification consumes notifications silently. Currently the only one
// the protocol sends us is notifications/initialized, which we acknowledge by
// doing nothing (per spec, no response is allowed).
func (s *Server) handleNotification(_ context.Context, _ JSONRPCRequest) {
	// no-op
}

func (s *Server) writeResponse(resp JSONRPCResponse) {
	data, _ := json.Marshal(resp)
	fmt.Fprintf(os.Stdout, "%s\n", data)
}

func (s *Server) dispatch(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	case "resources/list":
		return s.handleResourcesList(req)
	case "prompts/list":
		return s.handlePromptsList(req)
	case "ping":
		// Some MCP clients send ping to keep the connection alive; respond empty.
		return successResponse(req.ID, map[string]any{})
	default:
		return errorResponse(req.ID, -32601, "method not found: "+req.Method)
	}
}

// supportedProtocolVersions lists protocol versions this server can speak,
// newest first. The MCP spec says servers should respond with the highest
// version they support that the client also supports — when the client asks
// for a newer version we echo it back, otherwise we fall back to the latest
// version we know.
var supportedProtocolVersions = []string{
	"2025-06-18",
	"2025-03-26",
	"2024-11-05",
}

func (s *Server) handleInitialize(req JSONRPCRequest) JSONRPCResponse {
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(req.Params, &params)

	negotiated := supportedProtocolVersions[0] // default: newest we support
	for _, v := range supportedProtocolVersions {
		if params.ProtocolVersion == v {
			negotiated = v
			break
		}
	}

	return successResponse(req.ID, map[string]any{
		"protocolVersion": negotiated,
		"capabilities": map[string]any{
			"tools":     map[string]any{},
			"resources": map[string]any{},
			"prompts":   map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "imprint",
			"version": "1.0.0",
		},
	})
}

func (s *Server) handleToolsList(req JSONRPCRequest) JSONRPCResponse {
	var tools []MCPTool
	if s.toolMode == "all" {
		tools = AllTools()
	} else {
		tools = CoreTools()
	}
	return successResponse(req.ID, map[string]any{"tools": tools})
}

func (s *Server) handleToolsCall(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, -32602, "invalid params")
	}

	// Map MCP tool names to HTTP endpoints
	result, err := s.callTool(ctx, params.Name, params.Arguments)
	if err != nil {
		return successResponse(req.ID, map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
			},
			"isError": true,
		})
	}

	// Format result as MCP content
	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return successResponse(req.ID, map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": string(resultJSON)},
		},
	})
}

func (s *Server) callTool(ctx context.Context, name string, args map[string]any) (any, error) {
	switch name {
	case "memory_recall", "memory_search":
		return s.post(ctx, "/imprint/search", args)
	case "memory_save":
		return s.post(ctx, "/imprint/remember", args)
	case "memory_forget":
		return s.post(ctx, "/imprint/forget", args)
	case "memory_context":
		return s.post(ctx, "/imprint/context", args)
	case "memory_profiles":
		return s.get(ctx, "/imprint/profile")
	case "memory_patterns":
		return s.post(ctx, "/imprint/patterns", args)
	case "memory_graph_query":
		return s.post(ctx, "/imprint/graph/query", args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (s *Server) post(ctx context.Context, path string, body any) (any, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.secret != "" {
		req.Header.Set("Authorization", "Bearer "+s.secret)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return result, nil
}

func (s *Server) get(ctx context.Context, path string) (any, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", s.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.secret != "" {
		req.Header.Set("Authorization", "Bearer "+s.secret)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return result, nil
}

func (s *Server) handleResourcesList(req JSONRPCRequest) JSONRPCResponse {
	return successResponse(req.ID, map[string]any{
		"resources": []map[string]any{
			{"uri": "imprint://status", "name": "AgentMemory Status", "mimeType": "application/json"},
			{"uri": "imprint://graph/stats", "name": "Knowledge Graph Stats", "mimeType": "application/json"},
		},
	})
}

func (s *Server) handlePromptsList(req JSONRPCRequest) JSONRPCResponse {
	return successResponse(req.ID, map[string]any{
		"prompts": []map[string]any{
			{"name": "recall_context", "description": "Recall relevant context for the current task"},
			{"name": "session_handoff", "description": "Generate a session handoff summary"},
			{"name": "detect_patterns", "description": "Detect patterns in recent sessions"},
		},
	})
}
