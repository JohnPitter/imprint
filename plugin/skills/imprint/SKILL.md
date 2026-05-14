---
name: imprint
description: Persistent local memory for Codex sessions. Use when prior project context, decisions, architecture notes, preferences, debugging history, or cross-session recall would help; use the Imprint MCP tools to search, recall, and save durable memories.
---

# Imprint

Imprint is a local memory system for Codex. It runs a local HTTP server backed
by SQLite, exposes MCP tools, captures official Codex hook events, and keeps
`codex-watch` as a transcript backfill path.

## When to use

Use Imprint when:

- The user asks what happened before, what was decided, or what prior work says.
- You need architecture, debugging, workflow, preference, or project history from previous sessions.
- You are about to make a meaningful technical decision and existing project memory could reduce risk.
- The user asks to remember, save, persist, or forget information.

## Tools

Prefer these MCP tools:

- `memory_recall`: retrieve focused context for the current task.
- `memory_search`: search memories and compressed observations.
- `memory_save`: persist a durable memory after a real decision or useful finding.
- `memory_forget`: remove an explicit memory by ID when requested.
- `memory_context`: get session/project context when a session id or project is known.
- `memory_profiles`, `memory_patterns`, `memory_graph_query`: use for broader audits, patterns, or graph exploration.

## Workflow

1. For project-context questions, call `memory_recall` with a concise query that includes the project, files, feature, or error when known.
2. Use returned memory IDs and titles when referencing prior context.
3. Save only durable signal with `memory_save`: decisions, constraints, recurring bugs, architectural facts, user preferences, and workflow lessons.
4. Do not save secrets, credentials, raw private data, or noisy one-off command output.

## Automatic Capture

The Codex plugin loads `plugin/hooks/codex-hooks.json` and records
`SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, and `Stop`
events as Imprint observations. It also starts `codex-watch` with the MCP
wrapper; the watcher tails `~/.codex/sessions/**/*.jsonl`, creates `codex_...`
sessions, and backfills prompts, tool calls, tool outputs, and assistant
messages. This complements explicit MCP recall/save operations.
