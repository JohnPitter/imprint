#!/bin/bash
# Imprint development bootstrap. Builds binaries from source for use with
# `claude --plugin-dir`. End users should install via the marketplace
# (`/plugin install imprint@imprint-tools`) — that path downloads prebuilt
# binaries automatically and does not need Go.
echo "[Imprint] Building plugin from source..."
echo

if ! command -v go &> /dev/null; then
    echo "[ERROR] Go is not installed. Please install Go 1.25+ from https://go.dev"
    exit 1
fi

if ! command -v node &> /dev/null; then
    echo "[ERROR] Node.js is not installed. Please install Node.js 18+ from https://nodejs.org"
    exit 1
fi

go run ./cmd/install
