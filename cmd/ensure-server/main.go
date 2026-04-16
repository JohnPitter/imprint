// ensure-server checks if the Imprint server is running and starts it if not.
// Called by the SessionStart hook before session-start.exe.
package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

func main() {
	port := os.Getenv("IMPRINT_PORT")
	if port == "" {
		port = "3111"
	}

	url := fmt.Sprintf("http://localhost:%s/imprint/livez", port)

	// Check if already running
	if isRunning(url) {
		os.Exit(0)
	}

	// Find the server binary
	serverBin := findServerBinary()
	if serverBin == "" {
		hookLog("ensure-server: no server binary found")
		os.Exit(0)
	}

	// Redirect server stdout/stderr to a log file for post-mortem debugging
	logFile := serverLogFile()

	cmd := exec.Command(serverBin)
	if logFile != nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}

	// Detach from parent process
	detachProcess(cmd)

	if err := cmd.Start(); err != nil {
		hookLog(fmt.Sprintf("ensure-server: failed to start %s: %v", serverBin, err))
		if logFile != nil {
			logFile.Close()
		}
		os.Exit(0)
	}

	// Wait up to 10 seconds for server to come up
	for i := range 10 {
		time.Sleep(time.Second)
		if isRunning(url) {
			if logFile != nil {
				logFile.Close()
			}
			os.Exit(0)
		}
		if i == 4 {
			hookLog("ensure-server: server not ready after 5s, still waiting...")
		}
	}

	hookLog("ensure-server: server did not respond within 10s")
	if logFile != nil {
		logFile.Close()
	}
	os.Exit(0)
}

func isRunning(url string) bool {
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// hookLog appends a timestamped line to ~/.imprint/hooks.log for post-mortem debugging.
func hookLog(msg string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	logPath := filepath.Join(home, ".imprint", "hooks.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().UTC().Format(time.RFC3339), msg)
}

// serverLogFile opens ~/.imprint/server.log for appending server output.
func serverLogFile() *os.File {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	logPath := filepath.Join(home, ".imprint", "server.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil
	}
	return f
}

func findServerBinary() string {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}

	// 1. Check next to this binary (plugin/bin/imprint.exe)
	self, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(self)
		candidate := filepath.Join(dir, "imprint"+ext)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		// Maybe we're in plugin/bin/hooks/, check parent
		candidate = filepath.Join(dir, "..", "imprint"+ext)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// 2. Check ~/.imprint/bin/
	home, err := os.UserHomeDir()
	if err == nil {
		candidate := filepath.Join(home, ".imprint", "bin", "imprint"+ext)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return ""
}
