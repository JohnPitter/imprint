# Imprint — Release Notes

## Unreleased (proposed v1.6.0) — Token economy, three-tier memory & Codex support

The biggest release since the initial plugin: Imprint becomes a token-economy +
high-value-memory plugin, and gains first-class **Codex** support alongside Claude Code.

### Token economy meter (Phase 1)
- Append-only `token_ledger` + `injection_log` (migration 006), idempotent saving credit.
- Every background LLM spend point instrumented (compress, consolidate, graph, reflect,
  summarize, root) → ledger + budget.
- **Budget ceiling** (per session/day) that pauses background work and trims injection
  before overspending, without breaking the main path.
- `GET /imprint/economy` (plan-aware: currency for API, "fôlego" for subscription) +
  **Economy** UI tab. "Memory used" via cheap file/concept co-occurrence.

### Three memory layers + intuition (Phase 2)
- Explicit layers: base (compressed observations) → refined (memories) → **rooted
  (intuitions)** (migration 007).
- Intuitions are born only by convergence of many insights, injected resident at max
  priority, auto-weaken on contradiction, with a hard residency cap.
- **Inspection screen** (always visible, manually demotable/deletable) — invariant 11.
- **Lazy on-demand injection** (`/imprint/inject/lazy`) and precedence handling
  (specific refined overrides generic intuition).
- **Memory governance (A5):** export, purge-by-repo, full reset — drops docs from
  search indexes too.

### Spend Haiku only on what becomes insight (Phase 3)
- Pre-compression importance gate: trivial observations are captured deterministically
  (regex pre-pass, no LLM) into the base layer.

### Code graph as a relevance signal (Phase 4)
- `BlastRadius` over the existing graph boosts lazy injection — editing a file surfaces
  memories about structurally related files. Pure-Go (no tree-sitter / CGO).

### Codex support
- Codex plugin (`.codex-plugin/plugin.json`, hooks, marketplace, `codex-hook` /
  `codex-watch` binaries) with the official Codex hook format (`command` +
  `commandWindows`) and MCP config (`mcp_servers`).
- **Full feature parity** with Claude Code — the only difference is the background model.

### Cheap GPT-5 background (Codex's "Haiku")
- New OpenAI provider tuned for GPT-5 reasoning models (`max_completion_tokens`, no
  `temperature`, `reasoning_effort`). Default `gpt-5-mini`; `gpt-5-nano` for the cheapest tier.
- **Codex ChatGPT-OAuth**: reuses an existing `codex login` (no API key) — reads
  `~/.codex/auth.json`, refreshes via `auth.openai.com`, calls the ChatGPT backend
  Responses API. Or use `OPENAI_API_KEY`. Auto-detected; zero-config.
- Provider chain: `anthropic → codex-oauth → openai → openrouter → llamacpp`.

### Compatibility
- Claude Code / Anthropic flow is unchanged and takes priority when an Anthropic key is
  present; the new providers only activate when their credentials exist. Both work in the
  same install. All migrations idempotent; per-repo isolation preserved.

> Note: the Codex ChatGPT-OAuth path is validated against mocks (SSE/headers/refresh);
> confirm against a live `codex login` on first use.
