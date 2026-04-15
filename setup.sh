#!/bin/bash
echo "[Imprint] Building and installing plugin..."
echo

# Check Go
if ! command -v go &> /dev/null; then
    echo "[ERROR] Go is not installed. Please install Go 1.25+ from https://go.dev"
    exit 1
fi

# Check Node
if ! command -v node &> /dev/null; then
    echo "[ERROR] Node.js is not installed. Please install Node.js 18+ from https://nodejs.org"
    exit 1
fi

# Run installer
go run ./cmd/install
