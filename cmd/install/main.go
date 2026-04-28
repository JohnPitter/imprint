// install.go builds Imprint binaries and installs as a Claude Code plugin.
//
// Usage:
//
//	go run ./cmd/install          # Build + install
//	go run ./cmd/install --uninstall   # Remove from settings
//	go run ./cmd/install --build-only  # Build binaries only (no settings change)
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

var hookNames = []string{
	"session-start", "session-end", "prompt-submit",
	"post-tool-use", "post-tool-failure", "pre-tool-use",
	"pre-compact", "subagent-start", "subagent-stop",
	"notification", "stop",
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--uninstall" {
		doUninstall()
		return
	}
	buildOnly := len(os.Args) > 1 && os.Args[1] == "--build-only"

	projectRoot := findProjectRoot()
	pluginDir := filepath.Join(projectRoot, "plugin")
	binDir := filepath.Join(pluginDir, "bin")
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}

	fmt.Println("[build] Project root:", projectRoot)

	// 1. Build frontend
	fmt.Println("[build] Building frontend...")
	runCmd(projectRoot, "npm", "install", "--prefix", "frontend")
	runCmd(projectRoot, "npm", "run", "build", "--prefix", "frontend")

	// 2. Build server binary → plugin/bin/imprint[.exe]
	fmt.Println("[build] Building Imprint server...")
	os.MkdirAll(binDir, 0o755)
	buildGo(projectRoot, filepath.Join(binDir, "imprint"+ext), ".")

	// 3. Build hooks → plugin/bin/hooks/*[.exe]
	fmt.Println("[build] Building hooks...")
	os.MkdirAll(filepath.Join(binDir, "hooks"), 0o755)
	for _, hook := range hookNames {
		buildGo(projectRoot, filepath.Join(binDir, "hooks", hook+ext), "./cmd/hooks/"+hook)
		fmt.Printf("  %s\n", hook)
	}

	// 4. Build MCP server → plugin/bin/mcp-server[.exe]
	fmt.Println("[build] Building MCP server...")
	buildGo(projectRoot, filepath.Join(binDir, "mcp-server"+ext), "./cmd/mcp-server")

	// 5. Make scripts executable
	scriptsDir := filepath.Join(pluginDir, "scripts")
	if entries, err := os.ReadDir(scriptsDir); err == nil {
		for _, e := range entries {
			os.Chmod(filepath.Join(scriptsDir, e.Name()), 0o755)
		}
	}

	// 6. Create data directory
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".imprint")
	os.MkdirAll(dataDir, 0o755)

	if buildOnly {
		fmt.Println("\n[build] Done! Binaries built in plugin/bin/")
		fmt.Println("  Test with: claude --plugin-dir", filepath.ToSlash(pluginDir))
		return
	}

	// 7. Register in Claude Code settings.json
	fmt.Println("[install] Registering plugin in Claude Code settings...")
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	registerPlugin(settingsPath, pluginDir)

	fmt.Println()
	fmt.Println("[install] Done! Imprint installed as Claude Code plugin.")
	fmt.Println()
	fmt.Println("  Plugin dir:", filepath.ToSlash(pluginDir))
	fmt.Println("  Data dir:  ", filepath.ToSlash(dataDir))
	fmt.Println()
	fmt.Println("  The server auto-starts when you open Claude Code.")
	fmt.Println("  Web UI:     http://localhost:3111")
	fmt.Println()
	fmt.Println("  To test without installing:")
	fmt.Printf("    claude --plugin-dir %s\n", filepath.ToSlash(pluginDir))
	fmt.Println()
	fmt.Println("  To uninstall:")
	fmt.Println("    go run ./cmd/install --uninstall")
}

func registerPlugin(settingsPath, pluginDir string) {
	settings := loadSettings(settingsPath)

	// Remove any old direct hooks containing "imprint"
	if hooksMap, ok := settings["hooks"].(map[string]any); ok {
		for event, entries := range hooksMap {
			if arr, ok := entries.([]any); ok {
				var filtered []any
				for _, e := range arr {
					if m, ok := e.(map[string]any); ok {
						if hooks, ok := m["hooks"].([]any); ok {
							hasImprint := false
							for _, h := range hooks {
								if hm, ok := h.(map[string]any); ok {
									if cmd, ok := hm["command"].(string); ok && strings.Contains(cmd, "imprint") {
										hasImprint = true
									}
								}
							}
							if !hasImprint {
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

	// Remove old MCP server entry
	if mcpServers, ok := settings["mcpServers"].(map[string]any); ok {
		delete(mcpServers, "imprint")
		settings["mcpServers"] = mcpServers
	}

	// Add plugin-dir based hooks (the proper plugin way)
	// Claude Code reads hooks from the plugin's hooks/hooks.json when loaded via --plugin-dir
	// But for persistent installation, we register hooks directly pointing to plugin/bin/
	hooksMap := getOrCreateMap(settings, "hooks")
	pluginBinHooks := filepath.ToSlash(filepath.Join(pluginDir, "bin", "hooks"))
	ensureScript := filepath.ToSlash(filepath.Join(pluginDir, "scripts", "ensure-server.sh"))
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}

	type hookDef struct {
		event   string
		matcher string
	}
	hookDefs := map[string]hookDef{
		"session-start":     {"SessionStart", ""},
		"session-end":       {"SessionEnd", ""},
		"prompt-submit":     {"UserPromptSubmit", ""},
		"post-tool-use":     {"PostToolUse", ""},
		"post-tool-failure": {"PostToolUseFailure", ""},
		"pre-tool-use":      {"PreToolUse", "Edit|Write|Read|Glob|Grep"},
		"pre-compact":       {"PreCompact", ""},
		"subagent-start":    {"SubagentStart", ""},
		"subagent-stop":     {"SubagentStop", ""},
		"notification":      {"Notification", ""},
		"task-completed":    {"TaskCompleted", ""},
		"stop":              {"Stop", ""},
	}

	for _, hook := range hookNames {
		def := hookDefs[hook]
		hookBin := filepath.ToSlash(filepath.Join(pluginBinHooks, hook+ext))

		// SessionStart gets the ensure-server prefix
		cmd := hookBin
		if hook == "session-start" {
			cmd = ensureScript + " && " + hookBin
		}

		entry := map[string]any{
			"matcher": def.matcher,
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": cmd,
				},
			},
		}

		if existing, ok := hooksMap[def.event]; ok {
			if arr, ok := existing.([]any); ok {
				hooksMap[def.event] = append(arr, entry)
			} else {
				hooksMap[def.event] = []any{entry}
			}
		} else {
			hooksMap[def.event] = []any{entry}
		}
	}
	settings["hooks"] = hooksMap

	// Register MCP server
	mcpServers := getOrCreateMap(settings, "mcpServers")
	mcpServers["imprint"] = map[string]any{
		"command": filepath.ToSlash(filepath.Join(pluginDir, "bin", "mcp-server"+ext)),
		"args":    []string{},
	}
	settings["mcpServers"] = mcpServers

	saveSettings(settingsPath, settings)
}

func doUninstall() {
	home, _ := os.UserHomeDir()
	settingsPath := filepath.Join(home, ".claude", "settings.json")

	fmt.Println("[uninstall] Removing Imprint from Claude Code settings...")
	settings := loadSettings(settingsPath)

	// Remove hooks
	if hooksMap, ok := settings["hooks"].(map[string]any); ok {
		for event, entries := range hooksMap {
			if arr, ok := entries.([]any); ok {
				var filtered []any
				for _, e := range arr {
					if m, ok := e.(map[string]any); ok {
						if hooks, ok := m["hooks"].([]any); ok {
							has := false
							for _, h := range hooks {
								if hm, ok := h.(map[string]any); ok {
									if cmd, ok := hm["command"].(string); ok && strings.Contains(cmd, "imprint") {
										has = true
									}
								}
							}
							if !has {
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

	// Remove MCP
	if mcpServers, ok := settings["mcpServers"].(map[string]any); ok {
		delete(mcpServers, "imprint")
		settings["mcpServers"] = mcpServers
	}

	saveSettings(settingsPath, settings)
	fmt.Println("[uninstall] Done. Binaries in plugin/bin/ are not removed.")
}

func findProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			fmt.Fprintln(os.Stderr, "[error] cannot find go.mod")
			os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "[error] build %s failed: %v\n", pkg, err)
		os.Exit(1)
	}
}

func runCmd(dir, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func loadSettings(path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]any{}
	}
	var s map[string]any
	json.Unmarshal(data, &s)
	return s
}

func saveSettings(path string, s map[string]any) {
	data, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(path, data, 0o644)
}

func getOrCreateMap(m map[string]any, key string) map[string]any {
	if v, ok := m[key]; ok {
		if vm, ok := v.(map[string]any); ok {
			return vm
		}
	}
	r := map[string]any{}
	m[key] = r
	return r
}
