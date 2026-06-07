.PHONY: dev build hooks mcp cli codex-watch codex-hook frontend test clean all

# Development mode
dev: frontend
	go run .

# Build everything
all: frontend build hooks mcp cli codex-watch codex-hook

# Build Go binary with embedded frontend
build: frontend
	go build -ldflags="-s -w" -o imprint.exe .

# Build frontend
frontend:
	cd frontend && npm run build

# Build all hook binaries
hooks:
	@echo "Building hooks..."
	@mkdir -p build/hooks
	@for hook in session-start session-end prompt-submit post-tool-use \
	             post-tool-failure pre-tool-use pre-compact subagent-start \
	             subagent-stop notification stop; do \
		go build -ldflags="-s -w" -o build/hooks/$$hook.exe ./cmd/hooks/$$hook/ && \
		echo "  $$hook.exe"; \
	done
	@echo "Done."

# Build MCP server binary
mcp:
	go build -ldflags="-s -w" -o build/mcp-server.exe ./cmd/mcp-server/
	@echo "Built build/mcp-server.exe"

# Build CLI binary
cli:
	go build -ldflags="-s -w" -o build/imprint-cli.exe ./cmd/cli/
	@echo "Built build/imprint-cli.exe"

# Build Codex transcript watcher
codex-watch:
	go build -ldflags="-s -w" -o build/codex-watch.exe ./cmd/codex-watch/
	@echo "Built build/codex-watch.exe"

# Build Codex hook adapter
codex-hook:
	go build -ldflags="-s -w" -o build/codex-hook.exe ./cmd/codex-hook/
	@echo "Built build/codex-hook.exe"

# Run tests
test:
	go test ./... -count=1

# Run tests with verbose output
test-v:
	go test ./... -count=1 -v

# Clean build artifacts
clean:
	rm -rf imprint.exe build/hooks build/mcp-server.exe build/codex-watch.exe build/codex-hook.exe
	rm -rf frontend/dist
	rm -rf internal/search/bleve_index

# Install frontend dependencies
install:
	cd frontend && npm install

# Lint
vet:
	go vet ./...
