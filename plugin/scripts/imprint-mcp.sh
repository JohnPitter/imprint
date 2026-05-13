#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

if [ ! -x "$PLUGIN_ROOT/bin/mcp-server" ] || [ ! -x "$PLUGIN_ROOT/bin/ensure-server" ] || [ ! -x "$PLUGIN_ROOT/bin/imprint" ] || [ ! -x "$PLUGIN_ROOT/bin/codex-watch" ]; then
  "$PLUGIN_ROOT/scripts/ensure-binaries.sh" >&2 || true
fi

if [ ! -x "$PLUGIN_ROOT/bin/mcp-server" ] || [ ! -x "$PLUGIN_ROOT/bin/ensure-server" ] || [ ! -x "$PLUGIN_ROOT/bin/imprint" ] || [ ! -x "$PLUGIN_ROOT/bin/codex-watch" ]; then
  echo "imprint: missing plugin/bin binaries; run 'go run ./cmd/install' from the repository root" >&2
  exit 1
fi

"$PLUGIN_ROOT/bin/ensure-server" >/dev/null
nohup "$PLUGIN_ROOT/bin/codex-watch" >/dev/null 2>&1 &
exec "$PLUGIN_ROOT/bin/mcp-server"
