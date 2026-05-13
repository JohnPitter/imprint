// codex-watch tails Codex JSONL transcripts and mirrors relevant events into
// Imprint. Codex plugins currently package MCP servers and skills; this watcher
// gives Imprint automatic capture without relying on Claude Code hook payloads.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"imprint/internal/hooks"
)

const (
	defaultPollInterval = 2 * time.Second
	lockStaleAfter      = 30 * time.Second
	recentWindow        = 24 * time.Hour
)

type stateFile struct {
	Files map[string]int64 `json:"files"`
}

type watcher struct {
	cfg        hooks.Config
	sessions   string
	dataDir    string
	statePath  string
	lockPath   string
	state      stateFile
	callNames  map[string]string
	sessionCwd map[string]string
	fileSess   map[string]string
}

func main() {
	w, err := newWatcher()
	if err != nil {
		logLine("codex-watch: " + err.Error())
		return
	}
	if !w.acquireLock() {
		return
	}
	defer w.releaseLock()

	w.loadState()
	w.run()
}

func newWatcher() (*watcher, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	dataDir := os.Getenv("IMPRINT_DATA_DIR")
	if dataDir == "" {
		dataDir = filepath.Join(home, ".imprint")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}

	sessions := os.Getenv("CODEX_SESSIONS_DIR")
	if sessions == "" {
		sessions = filepath.Join(home, ".codex", "sessions")
	}

	return &watcher{
		cfg:        hooks.LoadConfig(),
		sessions:   sessions,
		dataDir:    dataDir,
		statePath:  filepath.Join(dataDir, "codex-watch-state.json"),
		lockPath:   filepath.Join(dataDir, "codex-watch.lock"),
		state:      stateFile{Files: map[string]int64{}},
		callNames:  map[string]string{},
		sessionCwd: map[string]string{},
		fileSess:   map[string]string{},
	}, nil
}

func (w *watcher) run() {
	interval := pollInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		w.touchLock()
		if err := w.scanOnce(); err != nil {
			logLine("codex-watch: scan failed: " + err.Error())
		}
		w.saveState()
		<-ticker.C
	}
}

func pollInterval() time.Duration {
	raw := os.Getenv("IMPRINT_CODEX_WATCH_INTERVAL_SECONDS")
	if raw == "" {
		return defaultPollInterval
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultPollInterval
	}
	return time.Duration(n) * time.Second
}

func (w *watcher) scanOnce() error {
	entries, err := findJSONL(w.sessions)
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-recentWindow)
	for _, path := range entries {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		offset, known := w.state.Files[path]
		if !known && info.ModTime().Before(cutoff) {
			w.state.Files[path] = info.Size()
			continue
		}
		if info.Size() < offset {
			offset = 0
		}
		next, err := w.processFile(path, offset)
		if err != nil {
			logLine(fmt.Sprintf("codex-watch: process %s: %v", path, err))
			continue
		}
		w.state.Files[path] = next
	}
	return nil
}

func findJSONL(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".jsonl") {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

func (w *watcher) processFile(path string, offset int64) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return offset, err
	}
	defer f.Close()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset, err
	}

	reader := bufio.NewReader(f)
	pos := offset
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			pos += int64(len(line))
			w.processLine(path, bytes.TrimSpace(line))
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return pos, err
		}
	}
	return pos, nil
}

func (w *watcher) processLine(path string, line []byte) {
	if len(line) == 0 {
		return
	}

	var entry map[string]any
	if err := json.Unmarshal(line, &entry); err != nil {
		return
	}

	timestamp := parseTimestamp(stringValue(entry["timestamp"]))
	entryType := stringValue(entry["type"])
	payload := mapValue(entry["payload"])
	if payload == nil {
		return
	}

	switch entryType {
	case "session_meta":
		w.handleSessionMeta(path, payload)
	case "response_item":
		w.handleResponseItem(path, payload, timestamp)
	}
}

func (w *watcher) handleSessionMeta(path string, payload map[string]any) {
	rawID := stringValue(payload["id"])
	if rawID == "" {
		return
	}
	sessionID := codexSessionID(rawID)
	w.fileSess[path] = sessionID
	cwd := stringValue(payload["cwd"])
	if cwd != "" {
		w.sessionCwd[sessionID] = cwd
	}
	project := filepath.Base(cwd)
	if project == "." || project == string(filepath.Separator) || project == "" {
		project = "codex"
	}

	_, _ = hooks.Post(w.cfg, "/imprint/session/start", map[string]any{
		"sessionId": sessionID,
		"project":   project,
		"cwd":       cwd,
	})
	_, _ = hooks.Post(w.cfg, "/imprint/session/heartbeat", map[string]any{"sessionId": sessionID})
}

func (w *watcher) handleResponseItem(path string, payload map[string]any, timestamp time.Time) {
	itemType := stringValue(payload["type"])
	switch itemType {
	case "message":
		w.handleMessage(path, payload, timestamp)
	case "function_call":
		w.handleFunctionCall(path, payload, timestamp)
	case "function_call_output":
		w.handleFunctionOutput(path, payload, timestamp)
	}
}

func (w *watcher) handleMessage(path string, payload map[string]any, timestamp time.Time) {
	role := stringValue(payload["role"])
	sessionID := sessionFromPayload(payload)
	if sessionID == "" {
		sessionID = w.fileSess[path]
	}
	if sessionID == "" {
		return
	}

	text := contentText(payload["content"])
	if strings.TrimSpace(text) == "" {
		return
	}

	switch role {
	case "user":
		w.postObservation(map[string]any{
			"session_id":  sessionID,
			"hook_type":   "prompt_submit",
			"project_dir": w.sessionCwd[sessionID],
			"user_prompt": hooks.TruncateString(text, 8000),
			"timestamp":   timestamp,
		})
	case "assistant":
		input, _ := json.Marshal(map[string]any{"kind": "assistant_message", "timestamp": timestamp.Format(time.RFC3339Nano)})
		output, _ := json.Marshal(hooks.TruncateString(text, 8000))
		toolName := "codex_assistant"
		w.postObservation(map[string]any{
			"session_id":  sessionID,
			"hook_type":   "post_tool_use",
			"project_dir": w.sessionCwd[sessionID],
			"tool_name":   toolName,
			"tool_input":  json.RawMessage(input),
			"tool_output": json.RawMessage(output),
			"timestamp":   timestamp,
		})
	}
}

func (w *watcher) handleFunctionCall(path string, payload map[string]any, timestamp time.Time) {
	sessionID := sessionFromPayload(payload)
	if sessionID == "" {
		sessionID = w.fileSess[path]
	}
	if sessionID == "" {
		return
	}

	name := stringValue(payload["name"])
	if name == "" {
		name = "codex_tool"
	}
	callID := stringValue(payload["call_id"])
	if callID != "" {
		w.callNames[callID] = name
	}

	input := normalizeJSON(payload["arguments"])
	w.postObservation(map[string]any{
		"session_id":  sessionID,
		"hook_type":   "pre_tool_use",
		"project_dir": w.sessionCwd[sessionID],
		"tool_name":   name,
		"tool_input":  input,
		"metadata":    mustJSON(map[string]any{"call_id": callID, "source": "codex"}),
		"timestamp":   timestamp,
	})
}

func (w *watcher) handleFunctionOutput(path string, payload map[string]any, timestamp time.Time) {
	sessionID := sessionFromPayload(payload)
	if sessionID == "" {
		sessionID = w.fileSess[path]
	}
	if sessionID == "" {
		return
	}

	callID := stringValue(payload["call_id"])
	name := w.callNames[callID]
	if name == "" {
		name = "codex_tool_output"
	}

	input := mustJSON(map[string]any{"call_id": callID, "kind": "tool_output"})
	output := normalizeJSON(payload["output"])
	w.postObservation(map[string]any{
		"session_id":  sessionID,
		"hook_type":   "post_tool_use",
		"project_dir": w.sessionCwd[sessionID],
		"tool_name":   name,
		"tool_input":  input,
		"tool_output": output,
		"metadata":    mustJSON(map[string]any{"call_id": callID, "source": "codex"}),
		"timestamp":   timestamp,
	})
}

func (w *watcher) postObservation(payload map[string]any) {
	if _, err := hooks.Post(w.cfg, "/imprint/observe", payload); err != nil {
		logLine("codex-watch: observe failed: " + err.Error())
		return
	}
	if sid := stringValue(payload["session_id"]); sid != "" {
		_, _ = hooks.Post(w.cfg, "/imprint/session/heartbeat", map[string]any{"sessionId": sid})
	}
}

func sessionFromPayload(payload map[string]any) string {
	for _, key := range []string{"session_id", "sessionId"} {
		if v := stringValue(payload[key]); v != "" {
			return codexSessionID(v)
		}
	}
	return ""
}

func codexSessionID(id string) string {
	if strings.HasPrefix(id, "codex_") {
		return id
	}
	return "codex_" + id
}

func contentText(v any) string {
	switch c := v.(type) {
	case string:
		return c
	case []any:
		var parts []string
		for _, item := range c {
			m := mapValue(item)
			if m == nil {
				continue
			}
			if text := stringValue(m["text"]); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func normalizeJSON(v any) json.RawMessage {
	switch x := v.(type) {
	case nil:
		return nil
	case string:
		var raw json.RawMessage
		if json.Unmarshal([]byte(x), &raw) == nil && len(raw) > 0 {
			return raw
		}
		out, _ := json.Marshal(x)
		return out
	default:
		out, _ := json.Marshal(x)
		return out
	}
}

func mustJSON(v any) json.RawMessage {
	out, _ := json.Marshal(v)
	return out
}

func mapValue(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

func parseTimestamp(raw string) time.Time {
	if raw == "" {
		return time.Now()
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Now()
	}
	return t
}

func (w *watcher) loadState() {
	data, err := os.ReadFile(w.statePath)
	if err != nil {
		return
	}
	var s stateFile
	if json.Unmarshal(data, &s) == nil && s.Files != nil {
		w.state = s
	}
}

func (w *watcher) saveState() {
	data, _ := json.MarshalIndent(w.state, "", "  ")
	_ = os.WriteFile(w.statePath, data, 0o644)
}

func (w *watcher) acquireLock() bool {
	if info, err := os.Stat(w.lockPath); err == nil {
		if time.Since(info.ModTime()) < lockStaleAfter {
			return false
		}
		_ = os.Remove(w.lockPath)
	}
	f, err := os.OpenFile(w.lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return false
	}
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	_ = f.Close()
	return true
}

func (w *watcher) touchLock() {
	now := time.Now()
	_ = os.Chtimes(w.lockPath, now, now)
}

func (w *watcher) releaseLock() {
	_ = os.Remove(w.lockPath)
}

func logLine(msg string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".imprint")
	_ = os.MkdirAll(dir, 0o755)
	f, err := os.OpenFile(filepath.Join(dir, "codex-watch.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s %s\n", time.Now().UTC().Format(time.RFC3339), msg)
}

func init() {
	hooks.DefaultTimeout = 10 * time.Second
}
