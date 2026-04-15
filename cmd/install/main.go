// install.go installs imprint as a Claude Code plugin.
//
// What it does:
//   1. Builds all binaries (server, hooks, mcp-server) into ~/.imprint/bin/
//   2. Registers hooks in ~/.claude/settings.json
//   3. Registers MCP server in ~/.claude/settings.json
//
// Usage:
//   go run ./cmd/install
//   go run ./cmd/install --uninstall
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var hooks = []string{
	"session-start", "session-end", "prompt-submit",
	"post-tool-use", "post-tool-failure", "pre-tool-use",
	"pre-compact", "subagent-start", "subagent-stop",
	"notification", "task-completed", "stop",
}

var hookEvents = map[string]struct {
	event   string
	matcher string
}{
	"session-start":    {"SessionStart", ""},
	"session-end":      {"SessionEnd", ""},
	"prompt-submit":    {"UserPromptSubmit", ""},
	"post-tool-use":    {"PostToolUse", ""},
	"post-tool-failure": {"PostToolUseFailure", ""},
	"pre-tool-use":     {"PreToolUse", "Edit|Write|Read|Glob|Grep"},
	"pre-compact":      {"PreCompact", ""},
	"subagent-start":   {"SubagentStart", ""},
	"subagent-stop":    {"SubagentStop", ""},
	"notification":     {"Notification", ""},
	"task-completed":   {"TaskCompleted", ""},
	"stop":             {"Stop", ""},
}

func main() {
	uninstall := len(os.Args) > 1 && os.Args[1] == "--uninstall"

	home, err := os.UserHomeDir()
	if err != nil {
		fatal("cannot find home dir: %v", err)
	}

	binDir := filepath.Join(home, ".imprint", "bin")
	settingsPath := filepath.Join(home, ".claude", "settings.json")

	if uninstall {
		doUninstall(binDir, settingsPath)
		return
	}

	doInstall(binDir, settingsPath)
}

func doInstall(binDir, settingsPath string) {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}

	// 1. Create bin directory
	fmt.Println("[install] Creating", binDir)
	os.MkdirAll(binDir, 0o755)

	// 2. Find project root (where go.mod is)
	projectRoot := findProjectRoot()
	fmt.Println("[install] Project root:", projectRoot)

	// 3. Build frontend
	fmt.Println("[install] Building frontend...")
	runCmd(projectRoot, "npm", "install", "--prefix", "frontend")
	runCmd(projectRoot, "npm", "run", "build", "--prefix", "frontend")

	// 4. Build main server
	fmt.Println("[install] Building imprint server...")
	serverBin := filepath.Join(binDir, "imprint"+ext)
	buildGo(projectRoot, serverBin, ".")

	// 5. Build hooks
	fmt.Println("[install] Building hooks...")
	for _, hook := range hooks {
		hookBin := filepath.Join(binDir, "hooks", hook+ext)
		os.MkdirAll(filepath.Dir(hookBin), 0o755)
		buildGo(projectRoot, hookBin, "./cmd/hooks/"+hook)
		fmt.Printf("  %s\n", hook)
	}

	// 6. Build MCP server
	fmt.Println("[install] Building MCP server...")
	mcpBin := filepath.Join(binDir, "mcp-server"+ext)
	buildGo(projectRoot, mcpBin, "./cmd/mcp-server")

	// 7. Update settings.json
	fmt.Println("[install] Updating Claude Code settings...")
	settings := loadSettings(settingsPath)

	// Add hooks
	hooksMap := getOrCreateMap(settings, "hooks")
	hooksDir := filepath.Join(binDir, "hooks")

	for _, hook := range hooks {
		info := hookEvents[hook]
		hookCmd := filepath.ToSlash(filepath.Join(hooksDir, hook+ext))
		entry := []any{
			map[string]any{
				"matcher": info.matcher,
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": hookCmd,
					},
				},
			},
		}

		// Check if event already has entries
		if existing, ok := hooksMap[info.event]; ok {
			if arr, ok := existing.([]any); ok {
				// Append if not already present
				found := false
				for _, e := range arr {
					if m, ok := e.(map[string]any); ok {
						if hooks, ok := m["hooks"].([]any); ok {
							for _, h := range hooks {
								if hm, ok := h.(map[string]any); ok {
									if cmd, ok := hm["command"].(string); ok && strings.Contains(cmd, "imprint") {
										found = true
									}
								}
							}
						}
					}
				}
				if !found {
					hooksMap[info.event] = append(arr, entry[0])
				}
				continue
			}
		}
		hooksMap[info.event] = entry
	}
	settings["hooks"] = hooksMap

	// Add MCP server
	mcpServers := getOrCreateMap(settings, "mcpServers")
	mcpServers["imprint"] = map[string]any{
		"command": filepath.ToSlash(mcpBin),
		"args":    []string{},
	}
	settings["mcpServers"] = mcpServers

	saveSettings(settingsPath, settings)

	fmt.Println()
	fmt.Println("[install] Done! Imprint installed as Claude Code plugin.")
	fmt.Println()
	fmt.Printf("  Server:   %s\n", filepath.ToSlash(filepath.Join(binDir, "imprint"+ext)))
	fmt.Printf("  Hooks:    %s\n", filepath.ToSlash(filepath.Join(binDir, "hooks")))
	fmt.Printf("  MCP:      %s\n", filepath.ToSlash(mcpBin))
	fmt.Printf("  Settings: %s\n", settingsPath)
	fmt.Println()
	fmt.Println("  Start the server before using Claude Code:")
	fmt.Printf("    %s\n", filepath.ToSlash(filepath.Join(binDir, "imprint"+ext)))
	fmt.Println()
	fmt.Println("  Or run in background:")
	fmt.Printf("    %s &\n", filepath.ToSlash(filepath.Join(binDir, "imprint"+ext)))
}

func doUninstall(binDir, settingsPath string) {
	fmt.Println("[uninstall] Removing Imprint from Claude Code settings...")

	settings := loadSettings(settingsPath)

	// Remove hooks that contain "imprint"
	if hooksMap, ok := settings["hooks"].(map[string]any); ok {
		for event, entries := range hooksMap {
			if arr, ok := entries.([]any); ok {
				var filtered []any
				for _, e := range arr {
					if m, ok := e.(map[string]any); ok {
						if hooks, ok := m["hooks"].([]any); ok {
							hasAgentMemory := false
							for _, h := range hooks {
								if hm, ok := h.(map[string]any); ok {
									if cmd, ok := hm["command"].(string); ok && strings.Contains(cmd, "imprint") {
										hasAgentMemory = true
									}
								}
							}
							if !hasAgentMemory {
								filtered = append(filtered, e)
							}
						} else {
							filtered = append(filtered, e)
						}
					}
				}
				if len(filtered) > 0 {
					hooksMap[event] = filtered
				} else {
					delete(hooksMap, event)
				}
			}
		}
		settings["hooks"] = hooksMap
	}

	// Remove MCP server
	if mcpServers, ok := settings["mcpServers"].(map[string]any); ok {
		delete(mcpServers, "imprint")
		settings["mcpServers"] = mcpServers
	}

	saveSettings(settingsPath, settings)

	fmt.Println("[uninstall] Removing binaries...")
	os.RemoveAll(filepath.Join(binDir, "hooks"))
	os.Remove(filepath.Join(binDir, "imprint.exe"))
	os.Remove(filepath.Join(binDir, "imprint"))
	os.Remove(filepath.Join(binDir, "mcp-server.exe"))
	os.Remove(filepath.Join(binDir, "mcp-server"))

	fmt.Println("[uninstall] Done.")
}

func findProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			fatal("cannot find go.mod in any parent directory")
		}
		dir = parent
	}
}

func buildGo(projectRoot, output, pkg string) {
	cmd := exec.Command("go", "build", "-ldflags=-s -w", "-o", output, pkg)
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal("build %s failed: %v", pkg, err)
	}
}

func runCmd(dir string, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run() // ignore errors (npm install may warn)
}

func loadSettings(path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}
		}
		fatal("read settings: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		fatal("parse settings: %v", err)
	}
	return settings
}

func saveSettings(path string, settings map[string]any) {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		fatal("marshal settings: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		fatal("write settings: %v", err)
	}
}

func getOrCreateMap(m map[string]any, key string) map[string]any {
	if v, ok := m[key]; ok {
		if vm, ok := v.(map[string]any); ok {
			return vm
		}
	}
	result := map[string]any{}
	m[key] = result
	return result
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[error] "+format+"\n", args...)
	os.Exit(1)
}
