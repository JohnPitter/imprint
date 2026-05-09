-- 002_eval_candidates.sql
--
-- Records `(query, returned ids)` for every search/recall call when the
-- IMPRINT_EVAL_CAPTURE feature flag is on. Used later to replay queries
-- against new code paths and detect retrieval regressions before release.
--
-- Off by default. When enabled, the capture path scrubs PII first via the
-- existing privacy.ScrubAll chain, so this table never holds raw tool
-- output even on opt-in installs. The schema is intentionally narrow —
-- richer eval metadata (timing, latency, score breakdown) can be layered
-- on later columns without breaking the simple capture flow.

CREATE TABLE IF NOT EXISTS eval_candidates (
    id              TEXT PRIMARY KEY,
    captured_at     TEXT NOT NULL DEFAULT (datetime('now')),
    source          TEXT NOT NULL,                 -- "mcp" | "http" | "cli"
    operation       TEXT NOT NULL,                 -- "search" | "recall" | "graph_query"
    query_text      TEXT NOT NULL,                 -- already scrubbed
    returned_ids    TEXT NOT NULL,                 -- JSON array of memory ids in returned order
    result_count    INTEGER NOT NULL DEFAULT 0,
    session_id      TEXT
);

CREATE INDEX IF NOT EXISTS idx_eval_candidates_captured_at ON eval_candidates(captured_at);
CREATE INDEX IF NOT EXISTS idx_eval_candidates_operation   ON eval_candidates(operation);
