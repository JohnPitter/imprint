package main

import (
	"context"
	"log"
	"os"

	"imprint/internal/mcp"
)

func main() {
	// Log to stderr only (MCP uses stdin/stdout for JSON-RPC)
	log.SetOutput(os.Stderr)

	baseURL := os.Getenv("IMPRINT_URL")
	if baseURL == "" {
		baseURL = "http://localhost:3111"
	}
	secret := os.Getenv("IMPRINT_SECRET")
	toolMode := os.Getenv("IMPRINT_TOOLS")

	server := mcp.NewServer(baseURL, secret, toolMode)

	if err := server.Run(context.Background()); err != nil {
		log.Fatalf("MCP server error: %v", err)
	}
}
