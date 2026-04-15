package mcp

// MCPTool defines an MCP tool with its name, description, and input schema.
type MCPTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// CoreTools returns the 8 core MCP tools.
func CoreTools() []MCPTool {
	return []MCPTool{
		{
			Name:        "memory_recall",
			Description: "Recall relevant memories and context for the current task",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "Search query"},
					"limit": map[string]any{"type": "integer", "description": "Max results", "default": 10},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "memory_save",
			Description: "Save a new long-term memory",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"type":     map[string]any{"type": "string", "enum": []string{"pattern", "preference", "architecture", "bug", "workflow", "fact"}},
					"title":    map[string]any{"type": "string"},
					"content":  map[string]any{"type": "string"},
					"concepts": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"strength": map[string]any{"type": "integer", "minimum": 1, "maximum": 10, "default": 5},
				},
				"required": []string{"title", "content"},
			},
		},
		{
			Name:        "memory_search",
			Description: "Search through memories and observations",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
					"limit": map[string]any{"type": "integer", "default": 10},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "memory_forget",
			Description: "Remove a memory by ID",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{"type": "string"},
				},
				"required": []string{"id"},
			},
		},
		{
			Name:        "memory_context",
			Description: "Get relevant context for the current session",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"sessionId": map[string]any{"type": "string"},
					"project":   map[string]any{"type": "string"},
					"budget":    map[string]any{"type": "integer", "default": 2000},
				},
			},
		},
		{
			Name:        "memory_profiles",
			Description: "Get project profiles and statistics",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"project": map[string]any{"type": "string"},
				},
			},
		},
		{
			Name:        "memory_patterns",
			Description: "Detect patterns across sessions",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "memory_graph_query",
			Description: "Query the knowledge graph",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"startNodeId": map[string]any{"type": "string"},
					"maxDepth":    map[string]any{"type": "integer", "default": 2},
				},
				"required": []string{"startNodeId"},
			},
		},
	}
}

// AllTools returns all MCP tools (core + advanced). For now, returns core only.
func AllTools() []MCPTool {
	// Will be expanded in Phase 7 with advanced tools
	return CoreTools()
}
