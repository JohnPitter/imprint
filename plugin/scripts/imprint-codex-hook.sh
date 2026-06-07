#!/usr/bin/env bash
# Unix Codex hook entrypoint (the `command` in hooks/codex-hooks.json; Windows
# uses the `commandWindows` .cmd override). Ensures the binaries and the local
# Imprint server exist, then pipes the Codex hook JSON (stdin) to codex-hook.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN="$PLUGIN_ROOT/bin"

if [ ! -x "$BIN/ensure-server" ] || [ ! -x "$BIN/codex-hook" ]; then
  "$PLUGIN_ROOT/scripts/ensure-binaries.sh" >&2 || true
fi

# Best-effort: if the hook binary is still missing, emit a no-op continue so the
# Codex session is never blocked, then exit cleanly.
if [ ! -x "$BIN/codex-hook" ]; then
  echo "imprint: codex-hook binary missing; run 'go run ./cmd/install' from the repo root" >&2
  echo '{"continue": true}'
  exit 0
fi

"$BIN/ensure-server" >/dev/null 2>&1 || true
exec "$BIN/codex-hook"
