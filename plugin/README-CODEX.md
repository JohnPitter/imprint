# Imprint on Codex

This directory now includes a Codex plugin manifest alongside the existing
Claude Code plugin files.

## What Works

- Codex can discover the plugin through `plugin/.codex-plugin/plugin.json`.
- The Codex MCP config uses `plugin/.mcp.codex.json`.
- Codex hooks are loaded from `plugin/hooks/codex-hooks.json`, separate from
  the Claude hook file at `plugin/hooks/hooks.json`.
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

The Codex plugin uses official Codex hooks for primary capture:
`SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, and `Stop`.
The hook adapter writes prompt, tool, and final assistant-message observations
to Imprint and can return session context to Codex during startup.

`codex-watch` remains enabled as a transcript backfill path. It stores offsets
in `~/.imprint/codex-watch-state.json` and uses
`~/.imprint/codex-watch.lock` to avoid duplicate watcher processes.
