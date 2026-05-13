# Imprint on Codex

This directory now includes a Codex plugin manifest alongside the existing
Claude Code plugin files.

## What Works

- Codex can discover the plugin through `plugin/.codex-plugin/plugin.json`.
- The Codex MCP config uses `plugin/.mcp.codex.json`.
- On Windows, the MCP command runs `plugin/scripts/imprint-mcp.cmd`, which:
  - builds `plugin/bin` from source if the binaries are missing;
  - starts the local Imprint HTTP server through `ensure-server.exe`;
  - starts `codex-watch.exe` in the background for automatic capture;
  - launches `mcp-server.exe` over stdio.
- `codex-watch` tails `~/.codex/sessions/**/*.jsonl` and records Codex
  prompts, tool calls, tool outputs, and assistant messages as Imprint
  observations.

The existing Claude files remain unchanged:

- `plugin/.claude-plugin/plugin.json`
- `plugin/.mcp.json`
- `plugin/hooks/hooks.json`

## Local Marketplace Entry

For repo-local Codex plugin discovery, use the marketplace entry at:

```text
.agents/plugins/marketplace.json
```

The entry points to `./plugin`, so it works from this repository layout without
moving files into a separate `plugins/imprint` directory.

## Capture Notes

Codex does not expose the same hook payload contract as Claude Code in this
repository, so automatic capture is transcript-based instead of event-hook
based. The watcher stores offsets in `~/.imprint/codex-watch-state.json` and
uses `~/.imprint/codex-watch.lock` to avoid duplicate watcher processes.
