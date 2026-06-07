# Imprint on Codex

This directory now includes a Codex plugin manifest alongside the existing
Claude Code plugin files. On Codex you get the **same** Imprint feature set as on
Claude Code (capture → compress → inject, the token-economy meter, the three
memory layers incl. intuitions, lazy injection, and the code-graph relevance
signal) — only the background LLM changes.

## Background model (cheap GPT-5, the Codex "Haiku")

The heavy background work (compress, consolidate, summarize, graph/intuition
extraction) runs on a cheap model, just like Haiku on Claude Code. On Codex that
defaults to **`gpt-5-mini`** (≈ $0.25 / $2.00 per 1M tokens). Set
`OPENAI_MODEL=gpt-5-nano` (≈ $0.05 / $0.40) for the cheapest tier.

Zero-config auth, in priority order (provider chain `anthropic → codex-oauth →
openai → openrouter → llamacpp`):

1. **Codex ChatGPT login (OAuth)** — if you ran `codex login` with a ChatGPT
   plan, Imprint reuses those tokens from `~/.codex/auth.json` directly: it
   refreshes the access token via `auth.openai.com` and calls the same ChatGPT
   backend Responses API Codex uses, billing your subscription. No API key.
   Model via `OPENAI_OAUTH_MODEL` (default `gpt-5`); the economy meter shows
   "fôlego" (subscription) instead of currency.
2. **OpenAI API key** — `OPENAI_API_KEY`, or an api-key `~/.codex/auth.json`.
   Defaults to `gpt-5-mini` (`OPENAI_MODEL`); `gpt-5-nano` for the cheapest tier.
   Priced per token, so the meter shows a currency saldo.

Tune via `OPENAI_MODEL` / `OPENAI_OAUTH_MODEL`, `OPENAI_REASONING_EFFORT`
(default `minimal`), `OPENAI_PRICE_IN` / `OPENAI_PRICE_OUT`.

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
