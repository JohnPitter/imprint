#!/bin/bash
# Ensure the Imprint server is running. Start it if not.
# Called by SessionStart hook before session-start.exe

PORT="${IMPRINT_PORT:-3111}"
SERVER="${CLAUDE_PLUGIN_ROOT}/bin/imprint.exe"

# Check if server is already running
if curl -s --connect-timeout 1 "http://localhost:${PORT}/imprint/livez" >/dev/null 2>&1; then
  exit 0
fi

# Start server in background
if [ -f "$SERVER" ]; then
  nohup "$SERVER" > "${HOME}/.imprint/server.log" 2>&1 &
  # Wait up to 3 seconds for server to start
  for i in 1 2 3; do
    sleep 1
    if curl -s --connect-timeout 1 "http://localhost:${PORT}/imprint/livez" >/dev/null 2>&1; then
      exit 0
    fi
  done
fi

exit 0
