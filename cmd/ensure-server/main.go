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
		os.Exit(0) // silently fail
	}

	// Start server in background
	cmd := exec.Command(serverBin)
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Detach from parent process
	detachProcess(cmd)

	if err := cmd.Start(); err != nil {
		os.Exit(0)
	}

	// Wait up to 5 seconds for server to come up
	for range 5 {
		time.Sleep(time.Second)
		if isRunning(url) {
			os.Exit(0)
		}
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
