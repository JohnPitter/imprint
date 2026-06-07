# Changelog

All notable changes to Imprint. Format based on [Keep a Changelog](https://keepachangelog.com/);
this project uses [Semantic Versioning](https://semver.org/).

## [2.0.1] — 2026-06-07

### Fixed
- Bump CI/release Go toolchain `1.25.10 → 1.25.11`, clearing the standard-library
  vulnerabilities `GO-2026-5039` (net/textproto) and `GO-2026-5037` (crypto/x509).
  No functional change; rebuilds the v2.0.0 binaries on the patched toolchain.

## [2.0.0] — 2026-06-07

Imprint becomes a **token-economy + high-value-memory** plugin and gains
first-class **Codex** support alongside Claude Code. See
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the design.

### Added — Token economy (Phase 1)
- Append-only `token_ledger` + `injection_log` (migration 006); the *saldo*
  (context saved − background LLM spent) is computed by aggregation, never a
  read-modify-write counter.
- Every background LLM spend point instrumented (compress, consolidate,
  graph_extract, reflect, summarize, root) via `CompletionRequest.SpendPoint`.
- **Budget ceiling** (`BudgetGate`): per-session/day token caps checked before
  the call; when hit, background work pauses and injection falls to the minimum
  without breaking the main path.
- `GET /imprint/economy` (plan-aware: currency for API, "fôlego" for
  subscriptions) + **Economy** UI tab. "Memory used" via file/concept
  co-occurrence (`CreditUsage`).

### Added — Three memory layers + intuition (Phase 2)
- Explicit layers: base (`compressed_observations`) → refined (`memories`) →
  rooted (`intuitions`, migration 007).
- Intuitions are born only by convergence of many refined insights across
  distinct sessions, injected resident at max priority (survive budget pause +
  lazy), and **auto-weaken on contradiction**; hard residency cap.
- **Inspection screen** (invariant 11): `GET /imprint/intuitions`,
  `/intuitions/contradictions`, `POST /intuitions/{demote,delete,detect}` +
  Intuitions UI tab.
- **Lazy on-demand injection** (`POST /imprint/inject/lazy`) and layer
  precedence (specific refined overrides generic intuition).
- **Memory governance (A5):** `GET /imprint/memory/export`,
  `POST /imprint/memory/{purge,reset}` — also drops docs from the search indexes.

### Added — Spend control (Phase 3)
- Pre-compression **importance gate**: trivial observations are captured
  deterministically (regex pre-pass, no LLM) into the base layer.

### Added — Code graph for selection (Phase 4)
- `GraphService.BlastRadius` over the existing graph boosts lazy injection —
  editing a file surfaces memories about structurally related files. Pure-Go
  (no tree-sitter / CGO).

### Added — Codex support & cheap GPT-5
- Codex plugin (`.codex-plugin/plugin.json`, hooks, marketplace, `codex-hook` /
  `codex-watch` binaries) using the official Codex hook format
  (`command` + `commandWindows`) and MCP config (`mcp_servers`).
- New **OpenAI GPT-5** provider tuned for reasoning models
  (`max_completion_tokens`, no `temperature`, `reasoning_effort`); default
  `gpt-5-mini`, `gpt-5-nano` for the cheapest tier.
- New **Codex ChatGPT-OAuth** provider: reuses `codex login` tokens from
  `~/.codex/auth.json` (no API key) — refreshes via `auth.openai.com` and calls
  the ChatGPT-backend Responses API.
- Auto-detection of OpenAI/Codex credentials, mirroring the Claude OAuth
  auto-detection. Provider chain
  `anthropic → codex-oauth → openai → openrouter → llamacpp`.

### Changed
- Web UI: 12 → 14 tabs (added Economy, Intuitions).
- Provider chain gains a token-budget gate alongside the failure circuit breaker.

### Compatibility
- The Claude Code / Anthropic flow is unchanged and takes priority when an
  Anthropic key is present; new providers only activate when their credential
  exists. Both agents work in one install. Migrations are idempotent; per-repo
  isolation preserved.

### Notes
- The Codex ChatGPT-OAuth path is validated against mocks (SSE/headers/refresh);
  confirm against a live `codex login` on first use.

## [1.5.4]
### Changed
- Heartbeat-based finalize: the `Stop` hook posts `/imprint/session/heartbeat`
  each turn; after 15 min idle the scheduler runs the final pipeline. Sessions
  that receive a later heartbeat are auto-resurrected to `active`.

## [1.5.1]
### Added
- Build provenance: `/imprint/health` exposes `version` + git `commit` (ldflags).

## [1.5.0]
### Added
- Pinned memories, time-travel slider, inline concept editor, why-this-memory
  score breakdown, cluster summarizer, conversation playback, pipeline health
  dashboard, live knowledge graph, configurable decay, color-coded concepts,
  incremental graph extraction, URL-synced UI state, per-result search scores.

## [1.4.0]
### Added
- Live Actions kanban via Server-Sent Events (`GET /imprint/actions/stream`).

## [1.3.0]
### Added
- Prompt-injection defense (output scrubbing), memory decay, backlink-boosted
  ranking, opt-in eval capture (`eval_candidates` + export).

## [1.2.0]
### Added
- Hybrid extraction: deterministic regex pre-pass for files/concepts/URLs/
  errors/git refs before the LLM (`IMPRINT_EXTRACTION_MODE`).

[2.0.1]: https://github.com/JohnPitter/imprint/releases/tag/v2.0.1
[2.0.0]: https://github.com/JohnPitter/imprint/releases/tag/v2.0.0
