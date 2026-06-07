# Imprint Architecture

This document describes the v2.0 design: the token-economy meter, the three
memory layers (including intuitions), lazy injection, the importance gate, the
code-graph relevance signal, and the multi-provider LLM layer. For a feature
list see the [README](../README.md); for the change history see
[CHANGELOG](../CHANGELOG.md).

## Guiding thesis

"Maximum memory" and "maximum token economy" are in direct tension: every
injected memory occupies context (the token we want to save) and was produced by
spending a background LLM. So the product is **not** maximum memory — it is
**memory with the highest signal/cost ratio**: little memory, but the right
memory, at the right time, in the fewest tokens.

The economy **percentage is an output to be measured, never a target to chase.**
It depends entirely on how much each repo/user reuses; a short, never-revisited
session can legitimately show a negative saldo, and that honesty is the feature.

## Hard invariants

1. Pure Go, no CGO (`modernc.org/sqlite`).
2. No Docker / external DB — everything local under `~/.imprint/`.
3. Privacy: every new datum passes the scrub regexes before persistence; no
   telemetry or external calls beyond the configured LLM.
4. The meter must not cost more than it saves — instrumentation is local
   counters/writes, never a dedicated LLM call.
5. Idempotent migrations; existing DBs migrate without loss.
6. Graceful degradation: if any new module fails, the
   capture → compress → inject path keeps working (everything is nil-safe).
7. Build provenance: `/imprint/health` exposes version + commit.
8. Per-repo isolation: injection and saldo are scoped by `project`; cross-repo is opt-in.
9. Safe schema rollback: new columns optional/tolerant of absence.
10. Concurrency-safe: the ledger is append-only; the saldo is computed by
    aggregation, never a read-modify-write counter.
11. Intuitions are always inspectable (passive visibility) and manually removable.

## Data flow

```
hook → /imprint/observe → raw_observations
     → compress worker → (importance gate) → compressed_observations  [base layer]
     → scheduler: summarize / consolidate → memories                  [refined layer]
                  reflect / graph extract → graph, insights
                  intuition pass → intuitions                         [rooted layer]
session start / lazy → context builder → injected blocks (+ injection_log)
every background LLM call → token_ledger (spend) ; gated by BudgetGate
```

## Storage & migrations

SQLite (WAL) under `~/.imprint/imprint.db`. Migrations are embedded numbered SQL
files applied once, tracked in `_migrations` (`internal/store/db.go`). New tables
since v2.0:

- **006_token_ledger.sql** — `token_ledger` (append-only spend/saving events) and
  `injection_log` (one row per injected item). A partial unique index on
  `token_ledger(ref_id) WHERE kind='saving'` makes "memory used" credit
  idempotent and atomic across sessions.
- **007_intuitions.sql** — `intuitions` (rooted layer; `schema_version` for A6)
  and `intuition_contradictions` (append-only weakening audit trail).

## Token economy (Phase 1)

`internal/store/ledger.go`, `internal/llm/budget.go`,
`internal/server/handler/economy.go`.

- **Spend** — every background call carries `CompletionRequest.SpendPoint`
  (`compress`, `consolidate`, `graph_extract`, `reflect`, `summarize`, `root`).
  The provider layer's `emitSpend` records `(input, output)` tokens to the ledger
  and the budget gate. Confidence `measured` (real provider usage).
- **Injection** — `BuildContext` writes one `injection_log` row per item
  (layer, item id, occupation tokens, files, concepts).
- **Memory used** — cheap **file/concept co-occurrence**: when a later
  observation in the same session touches an injected item's files/concepts,
  `CreditUsage` appends a `saving` event. Confidence `proxy`.
- **Baseline** — `contexto_poupado` is credited only for items actually touched,
  valued at a conservative floor (the item's own occupation tokens). The raw
  source the memory compressed is always larger, so this never overstates.
- **Saldo** = `Σ saved − Σ haiku` per repo/window, aggregated on read. Shown
  honestly (negative on cold start). Plan-aware: currency for API plans (priced
  with the active backend's rates — Haiku or GPT-5), "fôlego" for subscriptions.
- **Budget ceiling** — `BudgetGate` tracks tokens per session and per UTC day;
  `Allow` is checked in `ResilientProvider` *before* the breaker, and
  `ErrBudgetExceeded` is not counted as a provider failure. On exhaustion,
  instrumented calls are skipped (raw obs still stored) and `BuildContext` drops
  to L0/L1 + resident intuitions. Defaults are generous (catch runaways, not
  normal sessions); configurable in Settings.

## Three memory layers (Phase 2)

The layers are a **derived view** over existing tables plus one new table:

| Layer | Source | Meaning |
|---|---|---|
| **base** | `compressed_observations` | raw compressed capture (what happened) |
| **refined** | `memories` | distilled, episode-anchored insights (what I learned) |
| **rooted** | `intuitions` | cross-cutting reasoning premises (how I think) |

Three is the ceiling by design — they map the episodic / semantic / procedural
regimes of memory. New needs enter as a *type/tag within* a layer, never a 4th
layer.

### Intuition lifecycle (`internal/service/intuition.go`, `internal/pipeline/root.go`)

- **Birth — only by convergence.** The detector clusters refined memories by
  shared concept; a cluster must have ≥ `IntuitionMinConvergence` members across
  ≥ `IntuitionMinSessions` distinct sessions, and an LLM (`Rooter`, spend point
  `root`) must confirm they converge into one "how to reason" premise. Never
  created by hand. Existing overlapping intuitions are reinforced instead.
- **Residency.** Active intuitions are injected once in a max-priority block
  (alongside identity), surviving the budget pause and the lazy cut. A hard cap
  (`IntuitionMaxActive`) bounds the always-on context cost; the weakest is
  demoted when the cap is exceeded.
- **Correction — auto-weakening.** A background pass tests active intuitions
  against recent refined memories; an LLM contradiction judgment lowers the
  intuition's strength and, past a floor, demotes it back to refined. Each event
  is logged to `intuition_contradictions`. Already-judged `(intuition, memory)`
  pairs are skipped (no double penalty, no wasted LLM call).
- **Precedence (conflict).** Specific beats generic: a refined memory anchored to
  an episode/file overrides a generic intuition. The injected block states this
  so the model doesn't weigh them equally. Recurrent conflict is itself evidence
  that weakens the intuition.
- **Inspection (invariant 11).** The Intuitions tab and `/imprint/intuitions*`
  endpoints always show active intuitions, their evidence (source memory ids),
  current force, and last contradiction; the user can demote or delete any one.

## Injection

`internal/service/context.go`.

- **Eager (session start)** — L0 identity + resident intuitions + L1 essential
  story + L2 session context, each under a token budget, all logged to
  `injection_log`.
- **Lazy (on demand)** — `POST /imprint/inject/lazy {files, concepts}` returns
  only the refined memories matching what the turn is actually touching. This is
  the largest economy lever: memory the turn doesn't need is memory not spent.
- **Blast radius (Phase 4)** — before matching, the touched files are expanded
  with their graph blast radius (`GraphService.BlastRadius`, BFS over the
  existing LLM-extracted graph), so editing one file surfaces memories about
  structurally related files. Pure-Go; no tree-sitter binding exists without
  CGO, so the existing graph is reused instead.

## Importance gate (Phase 3)

`internal/pipeline/importance.go`. Before compression, `ScoreImportance` rates an
observation (errors, edits, decisions score high; read-only navigation scores
low). Below `COMPRESS_MIN_IMPORTANCE`, the observation is captured
deterministically from the regex pre-pass into the base layer with **zero** LLM
spend; only what can plausibly become a refined memory reaches the model.

## LLM provider layer

`internal/llm/`. A fallback chain (`LLM_PROVIDER_ORDER`, default
`anthropic → codex-oauth → openai → openrouter → llamacpp`); each entry
self-activates only when its credential exists, wrapped with a failure circuit
breaker, and gated by the global token budget.

- **Anthropic** (`anthropic.go`) — API key or auto-detected Claude Code OAuth
  token; default model Haiku.
- **OpenAI GPT-5** (`openai.go`) — Chat Completions tuned for reasoning models:
  `max_completion_tokens` (not `max_tokens`), `temperature` omitted (only the
  default is accepted), optional `reasoning_effort`. Default `gpt-5-mini`.
- **Codex ChatGPT-OAuth** (`codex_oauth.go`) — reuses `codex login` tokens from
  `~/.codex/auth.json` (no API key). Derives the account id from
  `tokens.account_id` or the id_token JWT (`https://api.openai.com/auth →
  chatgpt_account_id`), refreshes the access token via
  `auth.openai.com/oauth/token` (client_id `app_EMoamEEZ…`,
  `grant_type=refresh_token`) and persists rotated tokens back without clobbering
  Codex's other auth fields, then calls the ChatGPT backend Responses API
  (`chatgpt.com/backend-api/codex/responses`, SSE) with the Codex headers.
  Constants mirrored from the official Codex CLI (openai/codex).
- **OpenRouter / llama.cpp** (`openrouter.go`, `llamacpp.go`) — OpenAI-compatible.

Auth is auto-detected at boot (`internal/config`): Claude OAuth, then OpenAI key
(`OPENAI_API_KEY` or api-key `~/.codex/auth.json`), then Codex ChatGPT-OAuth.

## Memory governance (A5)

`internal/store/admin.go`, `internal/service/admin.go`. The user owns their
memory: `GET /imprint/memory/export` (versioned snapshot), `POST .../purge`
(delete one repo across every layer + ledger + search indexes), `POST .../reset`
(cold start). "Apagar é apagar" — deletes are hard and also drop docs from the
BM25/vector indexes.

## Compatibility & coexistence

The Anthropic/Claude Code flow is unchanged and takes priority when an Anthropic
key is present (`anthropic` is first in the chain; the new providers are
fallbacks that only activate with their own credentials). The economy meter
prices the saldo with whichever backend is active. Both agents work in a single
install; migrations are idempotent and tolerant of downgrade (A3/A9).
