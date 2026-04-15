package main

import (
	"fmt"
	"os"

	"imprint/internal/hooks"
)

func main() {
	cfg := hooks.LoadConfig()
	input, err := hooks.ReadStdin()
	if err != nil {
		os.Exit(0)
	}

	// Extract files and terms from tool input for enrichment
	var files, terms []string
	toolName := hooks.GetString(input, "tool_name")
	if toolInput, ok := input["tool_input"].(map[string]any); ok {
		if f, ok := toolInput["file_path"].(string); ok {
			files = append(files, f)
		}
		if f, ok := toolInput["path"].(string); ok {
			files = append(files, f)
		}
		if p, ok := toolInput["pattern"].(string); ok {
			terms = append(terms, p)
		}
		if q, ok := toolInput["query"].(string); ok {
			terms = append(terms, q)
		}
	}

	result, err := hooks.Post(cfg, "/imprint/enrich", map[string]any{
		"sessionId": hooks.GetString(input, "session_id"),
		"files":     files,
		"terms":     terms,
		"toolName":  toolName,
	})
	if err != nil {
		os.Exit(0)
	}

	if ctx, ok := result["context"].(string); ok && ctx != "" {
		fmt.Print(ctx)
	}
}
