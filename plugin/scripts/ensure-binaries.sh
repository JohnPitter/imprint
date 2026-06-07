#!/usr/bin/env bash
# ensure-binaries.sh
#
# Downloads the prebuilt Imprint binaries for the current OS+arch from the
# GitHub release matching the plugin version, if and only if they are not
# already present in $CLAUDE_PLUGIN_ROOT/bin/.
#
# Idempotent: runs every SessionStart but exits in milliseconds when the
# binaries already exist (the common case).
#
# Best-effort: prints a one-line warning to stderr on failure but never
# blocks session start. The hooks that follow will simply no-op when the
# binaries are missing.

set -u

PLUGIN_ROOT="${CLAUDE_PLUGIN_ROOT:-}"
if [ -z "$PLUGIN_ROOT" ]; then
  PLUGIN_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fi

BIN_DIR="$PLUGIN_ROOT/bin"
SENTINEL="$BIN_DIR/.version"
PLUGIN_JSON="$PLUGIN_ROOT/.claude-plugin/plugin.json"
REPO="JohnPitter/imprint"

# Read the plugin version so we know which release tag to fetch.
# Uses awk to avoid taking on a Python or jq dependency.
read_version() {
  if [ ! -f "$PLUGIN_JSON" ]; then
    echo ""
    return
  fi
  awk -F\" '/"version"[[:space:]]*:/ { print $4; exit }' "$PLUGIN_JSON"
}

VERSION="$(read_version)"
if [ -z "$VERSION" ]; then
  echo "imprint: cannot read plugin version from $PLUGIN_JSON" >&2
  exit 0
fi
TAG="v$VERSION"

# Fast path: binaries match the expected version, nothing to do.
if [ -f "$SENTINEL" ] && [ "$(cat "$SENTINEL" 2>/dev/null)" = "$TAG" ] &&
   { [ -x "$BIN_DIR/imprint" ] || [ -x "$BIN_DIR/imprint.exe" ]; } &&
   { [ -x "$BIN_DIR/codex-watch" ] || [ -x "$BIN_DIR/codex-watch.exe" ]; } &&
   { [ -x "$BIN_DIR/codex-hook" ] || [ -x "$BIN_DIR/codex-hook.exe" ]; }; then
  exit 0
fi

# Detect OS+arch in the release naming convention.
detect_target() {
  local os arch
  case "$(uname -s)" in
    Darwin) os=darwin ;;
    Linux) os=linux ;;
    CYGWIN*|MINGW*|MSYS*) os=windows ;;
    *) echo ""; return ;;
  esac
  case "$(uname -m)" in
    arm64|aarch64) arch=arm64 ;;
    x86_64|amd64) arch=amd64 ;;
    *) echo ""; return ;;
  esac
  echo "$os-$arch"
}

TARGET="$(detect_target)"
if [ -z "$TARGET" ]; then
  echo "imprint: unsupported OS/arch ($(uname -s)/$(uname -m))" >&2
  exit 0
fi

# Pick the archive extension. Windows release ships .zip; everyone else .tar.gz.
case "$TARGET" in
  windows-*) ARCHIVE="imprint-${TARGET}.zip" ;;
  *)         ARCHIVE="imprint-${TARGET}.tar.gz" ;;
esac

URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}"

if ! command -v curl >/dev/null 2>&1; then
  echo "imprint: curl not available, cannot download binaries" >&2
  exit 0
fi

TMP="$(mktemp -d 2>/dev/null || mktemp -d -t imprint)"
trap 'rm -rf "$TMP"' EXIT

echo "imprint: fetching binaries for $TARGET ($TAG)..." >&2

if ! curl -fsSL --max-time 60 -o "$TMP/$ARCHIVE" "$URL"; then
  echo "imprint: download failed from $URL" >&2
  echo "imprint: falling back to local build (run: cd \$(go env GOPATH)/src/imprint && go run ./cmd/install --build-only)" >&2
  exit 0
fi

# Extract into a temp tree, then atomically swap into place.
case "$ARCHIVE" in
  *.tar.gz)
    if ! tar -xzf "$TMP/$ARCHIVE" -C "$TMP"; then
      echo "imprint: extraction failed" >&2
      exit 0
    fi
    ;;
  *.zip)
    if command -v unzip >/dev/null 2>&1; then
      unzip -q "$TMP/$ARCHIVE" -d "$TMP" || { echo "imprint: unzip failed" >&2; exit 0; }
    else
      # Windows runners ship powershell; Git Bash on Windows can call it.
      powershell.exe -NoProfile -Command "Expand-Archive -Path '$TMP/$ARCHIVE' -DestinationPath '$TMP'" \
        || { echo "imprint: Expand-Archive failed" >&2; exit 0; }
    fi
    ;;
esac

EXTRACTED="$TMP/imprint-${TARGET}/bin"
if [ ! -d "$EXTRACTED" ]; then
  echo "imprint: extracted layout unexpected ($EXTRACTED missing)" >&2
  exit 0
fi

mkdir -p "$BIN_DIR"
# Copy with cp -R to preserve exec bits, then write the version sentinel last.
cp -R "$EXTRACTED/." "$BIN_DIR/"

# On Windows the release tarball contains *.exe binaries, but .mcp.json
# references the bare name (so a single config works across OSes).
# Side-by-side copies let either lookup succeed.
if [ "${TARGET%-*}" = "windows" ]; then
  for exe in "$BIN_DIR"/*.exe "$BIN_DIR"/hooks/*.exe; do
    [ -f "$exe" ] || continue
    base="${exe%.exe}"
    [ -f "$base" ] || cp "$exe" "$base"
  done
fi

echo "$TAG" > "$SENTINEL"

echo "imprint: binaries installed to $BIN_DIR" >&2
