// Package main implements a CLI command that imports historical Claude Code
// JSONL transcripts into Imprint's memory store via the /imprint/observe endpoint.
//
// Usage:
//
//	go run ./cmd/mine <path-to-transcript.jsonl>
//	go run ./cmd/mine <directory>  (scans for all .jsonl files)
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"imprint/internal/hooks"
)

// transcriptEntry represents a single line in a Claude Code JSONL transcript.
type transcriptEntry struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`

	// For assistant/human messages
	Message *struct {
		Content json.RawMessage `json:"content"`
	} `json:"message,omitempty"`

	// For tool_use result entries
	ToolName     string          `json:"tool_name,omitempty"`
	ToolInput    json.RawMessage `json:"tool_input,omitempty"`
	ToolResponse json.RawMessage `json:"tool_response,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: go run ./cmd/mine <path-to-transcript.jsonl | directory>\n")
		fmt.Fprintf(os.Stderr, "\nImports Claude Code JSONL transcripts into Imprint.\n")
		fmt.Fprintf(os.Stderr, "Typical location: ~/.claude/projects/*/transcript.jsonl\n")
		os.Exit(1)
	}

	target := os.Args[1]
	cfg := hooks.LoadConfig()

	info, err := os.Stat(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var files []string
	if info.IsDir() {
		files, err = findJSONLFiles(target)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning directory: %v\n", err)
			os.Exit(1)
		}
	} else {
		files = []string{target}
	}

	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "No .jsonl files found in %s\n", target)
		os.Exit(1)
	}

	fmt.Printf("Found %d transcript file(s)\n", len(files))

	totalObservations := 0
	for _, f := range files {
		count, err := processTranscript(cfg, f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error processing %s: %v\n", f, err)
			continue
		}
		totalObservations += count
		fmt.Printf("  %s: %d observations posted\n", filepath.Base(f), count)
	}

	fmt.Printf("\nDone. Total observations posted: %d\n", totalObservations)
}

// findJSONLFiles recursively finds all .jsonl files in a directory.
func findJSONLFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			// Skip hidden directories except .claude
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") && base != ".claude" && path != dir {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(info.Name()), ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// processTranscript reads a JSONL transcript and POSTs tool_use observations to Imprint.
func processTranscript(cfg hooks.Config, filePath string) (int, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	sessionID := "mine_" + sanitizeSessionID(filepath.Base(filePath))
	count := 0

	scanner := bufio.NewScanner(f)
	// Allow large lines (transcripts can have big tool outputs)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry transcriptEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue // skip malformed lines
		}

		// Only process tool_use result entries
		if entry.Type != "result" || entry.Subtype != "tool_use" {
			continue
		}
		if entry.ToolName == "" {
			continue
		}

		// Extract tool output as string
		toolOutput := extractToolOutput(entry.ToolResponse)
		if len(toolOutput) > 8000 {
			toolOutput = toolOutput[:8000]
		}

		// POST to /imprint/observe
		payload := map[string]any{
			"session_id":  sessionID,
			"hook_type":   "post_tool_use",
			"tool_name":   entry.ToolName,
			"tool_input":  entry.ToolInput,
			"tool_output": toolOutput,
		}

		if _, err := hooks.Post(cfg, "/imprint/observe", payload); err != nil {
			// Silently skip failures — the server might reject duplicates
			continue
		}
		count++
	}

	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("read file: %w", err)
	}

	return count, nil
}

// extractToolOutput converts the tool_response JSON to a string.
func extractToolOutput(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try as string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Fall back to raw JSON
	return string(raw)
}

// sanitizeSessionID creates a safe session ID from a filename.
func sanitizeSessionID(name string) string {
	// Remove extension
	name = strings.TrimSuffix(name, filepath.Ext(name))
	// Replace unsafe chars
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, name)
	if len(safe) > 32 {
		safe = safe[:32]
	}
	return safe
}
