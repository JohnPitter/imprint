-- Phase 1 — Token economy meter.
--
-- Two append-only tables that let us compute the token "saldo" (context saved −
-- Haiku spent) per repo/session/window without ever doing a read-modify-write
-- on a shared counter (invariant A4 — concurrency safe across sessions). The
-- balance is always derived by aggregation at read time.
--
-- All columns are additive and the path that reads them tolerates their absence,
-- so a downgraded binary that never SELECTs these tables keeps working (A3).

-- token_ledger: one row per economic event.
--   kind = 'haiku_spend' — background LLM tokens burned (compress/consolidate/...)
--   kind = 'saving'      — estimated context tokens an injected memory avoided
-- Spend rows carry input/output tokens; saving rows carry saved_tokens. project
-- may be empty on spend rows recorded deep in the LLM layer (no project in
-- scope there) — the economy query resolves it via a join on session_id.
CREATE TABLE IF NOT EXISTS token_ledger (
    id            TEXT PRIMARY KEY,
    ts            TEXT NOT NULL DEFAULT (datetime('now')),
    kind          TEXT NOT NULL,            -- 'haiku_spend' | 'saving'
    spend_point   TEXT NOT NULL DEFAULT '', -- 'compress' | 'consolidate' | ...
    provider      TEXT NOT NULL DEFAULT '',
    session_id    TEXT,
    project       TEXT NOT NULL DEFAULT '',
    ref_id        TEXT NOT NULL DEFAULT '', -- injection item id for savings
    input_tokens  INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    saved_tokens  INTEGER NOT NULL DEFAULT 0,
    confidence    TEXT NOT NULL DEFAULT ''  -- 'measured' | 'proxy' | 'estimated'
);

CREATE INDEX IF NOT EXISTS idx_token_ledger_ts ON token_ledger(ts);
CREATE INDEX IF NOT EXISTS idx_token_ledger_project ON token_ledger(project);
CREATE INDEX IF NOT EXISTS idx_token_ledger_session ON token_ledger(session_id);

-- One saving per injected item: INSERT OR IGNORE makes crediting idempotent and
-- atomic even if two sessions detect the same reuse concurrently (A4).
CREATE UNIQUE INDEX IF NOT EXISTS idx_token_ledger_saving_ref
    ON token_ledger(ref_id) WHERE kind = 'saving';

-- injection_log: one row per memory/observation actually injected into a
-- session's context. Append-only — never updated. files/concepts are stored so
-- a later turn's tool use can be matched against them (co-occurrence "used"
-- signal) to credit a saving without re-reading the source.
CREATE TABLE IF NOT EXISTS injection_log (
    id          TEXT PRIMARY KEY,
    ts          TEXT NOT NULL DEFAULT (datetime('now')),
    session_id  TEXT NOT NULL,
    project     TEXT NOT NULL DEFAULT '',
    layer       TEXT NOT NULL DEFAULT '',  -- 'L0' | 'L1' | 'L2'
    item_type   TEXT NOT NULL DEFAULT '',  -- 'memory' | 'observation' | 'summary'
    item_id     TEXT NOT NULL DEFAULT '',
    occ_tokens  INTEGER NOT NULL DEFAULT 0,
    files       TEXT NOT NULL DEFAULT '[]',
    concepts    TEXT NOT NULL DEFAULT '[]'
);

CREATE INDEX IF NOT EXISTS idx_injection_log_session ON injection_log(session_id);
CREATE INDEX IF NOT EXISTS idx_injection_log_project ON injection_log(project);
