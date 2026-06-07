-- Phase 2 — third memory layer: rooted memory ("intuition").
--
-- The three layers are a derived view over the existing schema plus this table:
--   base     = compressed_observations (raw compressed capture)
--   refined  = memories (distilled, episode-anchored insights)
--   rooted   = intuitions (this table) — cross-cutting premises about *how to
--              reason* in a context, born only by convergence of many refined
--              insights, injected resident at max priority, auto-weakened by
--              contradiction, and always inspectable (invariant 11).
--
-- Scoped per repo (A1). schema_version stamps the format for cross-version
-- migration tolerance (A6). All columns additive/optional so a downgraded
-- binary that never reads this table keeps working (A3).
CREATE TABLE IF NOT EXISTS intuitions (
    id                   TEXT PRIMARY KEY,
    created_at           TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at           TEXT NOT NULL DEFAULT (datetime('now')),
    project              TEXT NOT NULL DEFAULT '',
    statement            TEXT NOT NULL,            -- the "how to reason" rule
    strength             REAL NOT NULL DEFAULT 6,  -- force; decays on contradiction
    evidence_ids         TEXT NOT NULL DEFAULT '[]', -- source refined-memory ids that converged
    evidence_count       INTEGER NOT NULL DEFAULT 0,
    concepts             TEXT NOT NULL DEFAULT '[]',
    files                TEXT NOT NULL DEFAULT '[]',
    last_contradicted_at TEXT,
    contradiction_count  INTEGER NOT NULL DEFAULT 0,
    status               TEXT NOT NULL DEFAULT 'active', -- 'active' | 'demoted' | 'archived'
    schema_version       INTEGER NOT NULL DEFAULT 1,
    born_session_id      TEXT
);

CREATE INDEX IF NOT EXISTS idx_intuitions_project ON intuitions(project);
CREATE INDEX IF NOT EXISTS idx_intuitions_status ON intuitions(status);

-- Append-only contradiction log — the audit trail behind the auto-weakening,
-- surfaced on the inspection screen (invariant 11). Never updated.
CREATE TABLE IF NOT EXISTS intuition_contradictions (
    id             TEXT PRIMARY KEY,
    intuition_id   TEXT NOT NULL,
    ts             TEXT NOT NULL DEFAULT (datetime('now')),
    memory_id      TEXT NOT NULL DEFAULT '',   -- the refined memory that contradicted
    detail         TEXT NOT NULL DEFAULT '',
    strength_delta REAL NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_intuition_contra_id ON intuition_contradictions(intuition_id);
