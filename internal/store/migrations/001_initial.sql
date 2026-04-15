-- 001_initial.sql
-- Comprehensive schema for agentmemory
-- All timestamps are TEXT in ISO 8601 format
-- All JSON fields are TEXT

PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

-- ============================================================================
-- 1. sessions
-- ============================================================================
CREATE TABLE IF NOT EXISTS sessions (
    id          TEXT PRIMARY KEY,
    project     TEXT NOT NULL,
    cwd         TEXT NOT NULL,
    started_at  TEXT NOT NULL DEFAULT (datetime('now')),
    ended_at    TEXT,
    status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'completed', 'abandoned')),
    observation_count INTEGER NOT NULL DEFAULT 0,
    model       TEXT,
    tags        TEXT NOT NULL DEFAULT '[]'
);

CREATE INDEX idx_sessions_project ON sessions(project);
CREATE INDEX idx_sessions_status ON sessions(status);
CREATE INDEX idx_sessions_started_at ON sessions(started_at);

-- ============================================================================
-- 2. raw_observations
-- ============================================================================
CREATE TABLE IF NOT EXISTS raw_observations (
    id              TEXT PRIMARY KEY,
    session_id      TEXT NOT NULL REFERENCES sessions(id),
    timestamp       TEXT NOT NULL DEFAULT (datetime('now')),
    hook_type       TEXT NOT NULL,
    tool_name       TEXT,
    tool_input      TEXT,
    tool_output     TEXT,
    user_prompt     TEXT,
    raw             TEXT
);

CREATE INDEX idx_raw_observations_session_id ON raw_observations(session_id);
CREATE INDEX idx_raw_observations_timestamp ON raw_observations(timestamp);
CREATE INDEX idx_raw_observations_hook_type ON raw_observations(hook_type);
CREATE INDEX idx_raw_observations_tool_name ON raw_observations(tool_name);

-- ============================================================================
-- 3. compressed_observations
-- ============================================================================
CREATE TABLE IF NOT EXISTS compressed_observations (
    id                      TEXT PRIMARY KEY,
    session_id              TEXT NOT NULL REFERENCES sessions(id),
    timestamp               TEXT NOT NULL DEFAULT (datetime('now')),
    type                    TEXT NOT NULL,
    title                   TEXT NOT NULL,
    subtitle                TEXT,
    facts                   TEXT,
    narrative               TEXT,
    concepts                TEXT,
    files                   TEXT,
    importance              INTEGER NOT NULL CHECK (importance BETWEEN 1 AND 10),
    confidence              REAL NOT NULL DEFAULT 0.0,
    source_observation_id   TEXT
);

CREATE INDEX idx_compressed_observations_session_id ON compressed_observations(session_id);
CREATE INDEX idx_compressed_observations_timestamp ON compressed_observations(timestamp);
CREATE INDEX idx_compressed_observations_type ON compressed_observations(type);
CREATE INDEX idx_compressed_observations_importance ON compressed_observations(importance);
CREATE INDEX idx_compressed_observations_source_observation_id ON compressed_observations(source_observation_id);

-- ============================================================================
-- 4. memories
-- ============================================================================
CREATE TABLE IF NOT EXISTS memories (
    id                      TEXT PRIMARY KEY,
    created_at              TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at              TEXT NOT NULL DEFAULT (datetime('now')),
    type                    TEXT NOT NULL CHECK (type IN ('pattern', 'preference', 'architecture', 'bug', 'workflow', 'fact')),
    title                   TEXT NOT NULL,
    content                 TEXT NOT NULL,
    concepts                TEXT,
    files                   TEXT,
    session_ids             TEXT,
    strength                INTEGER NOT NULL CHECK (strength BETWEEN 1 AND 10),
    version                 INTEGER NOT NULL DEFAULT 1,
    parent_id               TEXT REFERENCES memories(id),
    supersedes              TEXT,
    source_observation_ids  TEXT,
    is_latest               INTEGER NOT NULL DEFAULT 1,
    forget_after            TEXT,
    ttl_days                INTEGER
);

CREATE INDEX idx_memories_type ON memories(type);
CREATE INDEX idx_memories_created_at ON memories(created_at);
CREATE INDEX idx_memories_updated_at ON memories(updated_at);
CREATE INDEX idx_memories_strength ON memories(strength);
CREATE INDEX idx_memories_is_latest ON memories(is_latest);
CREATE INDEX idx_memories_parent_id ON memories(parent_id);
CREATE INDEX idx_memories_forget_after ON memories(forget_after);

-- ============================================================================
-- 5. semantic_memories
-- ============================================================================
CREATE TABLE IF NOT EXISTS semantic_memories (
    id              TEXT PRIMARY KEY,
    project         TEXT NOT NULL,
    content         TEXT NOT NULL,
    confidence      REAL NOT NULL,
    access_count    INTEGER NOT NULL DEFAULT 0,
    last_accessed_at TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_semantic_memories_project ON semantic_memories(project);
CREATE INDEX idx_semantic_memories_confidence ON semantic_memories(confidence);
CREATE INDEX idx_semantic_memories_last_accessed_at ON semantic_memories(last_accessed_at);

-- ============================================================================
-- 6. procedural_memories
-- ============================================================================
CREATE TABLE IF NOT EXISTS procedural_memories (
    id          TEXT PRIMARY KEY,
    project     TEXT NOT NULL,
    name        TEXT NOT NULL,
    steps       TEXT,
    triggers    TEXT,
    frequency   INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_procedural_memories_project ON procedural_memories(project);
CREATE INDEX idx_procedural_memories_name ON procedural_memories(name);
CREATE INDEX idx_procedural_memories_frequency ON procedural_memories(frequency);

-- ============================================================================
-- 7. session_summaries
-- ============================================================================
CREATE TABLE IF NOT EXISTS session_summaries (
    session_id          TEXT PRIMARY KEY REFERENCES sessions(id),
    project             TEXT NOT NULL,
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    title               TEXT NOT NULL,
    narrative           TEXT NOT NULL,
    key_decisions       TEXT,
    files_modified      TEXT,
    concepts            TEXT,
    observation_count   INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_session_summaries_project ON session_summaries(project);
CREATE INDEX idx_session_summaries_created_at ON session_summaries(created_at);

-- ============================================================================
-- 8. graph_nodes
-- ============================================================================
CREATE TABLE IF NOT EXISTS graph_nodes (
    id                      TEXT PRIMARY KEY,
    type                    TEXT NOT NULL,
    name                    TEXT NOT NULL,
    properties              TEXT NOT NULL DEFAULT '{}',
    aliases                 TEXT NOT NULL DEFAULT '[]',
    source_observation_ids  TEXT NOT NULL DEFAULT '[]',
    created_at              TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_graph_nodes_type ON graph_nodes(type);
CREATE INDEX idx_graph_nodes_name ON graph_nodes(name);
CREATE INDEX idx_graph_nodes_type_name ON graph_nodes(type, name);

-- ============================================================================
-- 9. graph_edges
-- ============================================================================
CREATE TABLE IF NOT EXISTS graph_edges (
    id                      TEXT PRIMARY KEY,
    type                    TEXT NOT NULL,
    source_node_id          TEXT NOT NULL REFERENCES graph_nodes(id),
    target_node_id          TEXT NOT NULL REFERENCES graph_nodes(id),
    weight                  REAL NOT NULL DEFAULT 0.5 CHECK (weight BETWEEN 0 AND 1),
    source_observation_ids  TEXT NOT NULL DEFAULT '[]',
    created_at              TEXT NOT NULL DEFAULT (datetime('now')),
    valid_from              TEXT,
    valid_to                TEXT,
    is_latest               INTEGER NOT NULL DEFAULT 1,
    version                 INTEGER NOT NULL DEFAULT 1,
    context                 TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_graph_edges_type ON graph_edges(type);
CREATE INDEX idx_graph_edges_source_node_id ON graph_edges(source_node_id);
CREATE INDEX idx_graph_edges_target_node_id ON graph_edges(target_node_id);
CREATE INDEX idx_graph_edges_is_latest ON graph_edges(is_latest);
CREATE INDEX idx_graph_edges_source_target ON graph_edges(source_node_id, target_node_id);

-- ============================================================================
-- 10. actions
-- ============================================================================
CREATE TABLE IF NOT EXISTS actions (
    id              TEXT PRIMARY KEY,
    title           TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'blocked', 'in_progress', 'done', 'cancelled')),
    priority        INTEGER NOT NULL DEFAULT 5 CHECK (priority BETWEEN 1 AND 10),
    assignee        TEXT,
    project         TEXT,
    tags            TEXT NOT NULL DEFAULT '[]',
    parent_id       TEXT REFERENCES actions(id),
    sketch_id       TEXT,
    crystallized    INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_actions_status ON actions(status);
CREATE INDEX idx_actions_priority ON actions(priority);
CREATE INDEX idx_actions_assignee ON actions(assignee);
CREATE INDEX idx_actions_project ON actions(project);
CREATE INDEX idx_actions_parent_id ON actions(parent_id);
CREATE INDEX idx_actions_sketch_id ON actions(sketch_id);
CREATE INDEX idx_actions_created_at ON actions(created_at);

-- ============================================================================
-- 11. action_edges
-- ============================================================================
CREATE TABLE IF NOT EXISTS action_edges (
    id          TEXT PRIMARY KEY,
    source_id   TEXT NOT NULL REFERENCES actions(id),
    target_id   TEXT NOT NULL REFERENCES actions(id),
    type        TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_action_edges_source_id ON action_edges(source_id);
CREATE INDEX idx_action_edges_target_id ON action_edges(target_id);
CREATE INDEX idx_action_edges_type ON action_edges(type);

-- ============================================================================
-- 12. leases
-- ============================================================================
CREATE TABLE IF NOT EXISTS leases (
    id          TEXT PRIMARY KEY,
    action_id   TEXT NOT NULL REFERENCES actions(id),
    agent_id    TEXT NOT NULL,
    acquired_at TEXT NOT NULL DEFAULT (datetime('now')),
    expires_at  TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'active',
    result      TEXT
);

CREATE INDEX idx_leases_action_id ON leases(action_id);
CREATE INDEX idx_leases_agent_id ON leases(agent_id);
CREATE INDEX idx_leases_status ON leases(status);
CREATE INDEX idx_leases_expires_at ON leases(expires_at);

-- ============================================================================
-- 13. routines
-- ============================================================================
CREATE TABLE IF NOT EXISTS routines (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    steps       TEXT NOT NULL DEFAULT '[]',
    tags        TEXT NOT NULL DEFAULT '[]',
    frozen      INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_routines_name ON routines(name);
CREATE INDEX idx_routines_frozen ON routines(frozen);

-- ============================================================================
-- 14. signals
-- ============================================================================
CREATE TABLE IF NOT EXISTS signals (
    id          TEXT PRIMARY KEY,
    from_agent  TEXT NOT NULL,
    to_agent    TEXT NOT NULL,
    content     TEXT NOT NULL,
    type        TEXT NOT NULL DEFAULT 'info',
    thread_id   TEXT,
    parent_id   TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    expires_at  TEXT,
    read_by     TEXT NOT NULL DEFAULT '[]'
);

CREATE INDEX idx_signals_from_agent ON signals(from_agent);
CREATE INDEX idx_signals_to_agent ON signals(to_agent);
CREATE INDEX idx_signals_type ON signals(type);
CREATE INDEX idx_signals_thread_id ON signals(thread_id);
CREATE INDEX idx_signals_parent_id ON signals(parent_id);
CREATE INDEX idx_signals_created_at ON signals(created_at);
CREATE INDEX idx_signals_expires_at ON signals(expires_at);

-- ============================================================================
-- 15. checkpoints
-- ============================================================================
CREATE TABLE IF NOT EXISTS checkpoints (
    id                  TEXT PRIMARY KEY,
    name                TEXT NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    status              TEXT NOT NULL DEFAULT 'pending',
    type                TEXT NOT NULL DEFAULT 'approval',
    action_id           TEXT,
    config              TEXT NOT NULL DEFAULT '{}',
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    resolved_at         TEXT,
    resolved_by         TEXT,
    result              TEXT,
    expires_at          TEXT,
    linked_action_ids   TEXT NOT NULL DEFAULT '[]'
);

CREATE INDEX idx_checkpoints_status ON checkpoints(status);
CREATE INDEX idx_checkpoints_type ON checkpoints(type);
CREATE INDEX idx_checkpoints_action_id ON checkpoints(action_id);
CREATE INDEX idx_checkpoints_created_at ON checkpoints(created_at);
CREATE INDEX idx_checkpoints_expires_at ON checkpoints(expires_at);

-- ============================================================================
-- 16. sentinels
-- ============================================================================
CREATE TABLE IF NOT EXISTS sentinels (
    id                  TEXT PRIMARY KEY,
    name                TEXT NOT NULL,
    type                TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'watching',
    config              TEXT NOT NULL DEFAULT '{}',
    result              TEXT,
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    triggered_at        TEXT,
    expires_at          TEXT,
    linked_action_ids   TEXT NOT NULL DEFAULT '[]',
    escalated_at        TEXT
);

CREATE INDEX idx_sentinels_type ON sentinels(type);
CREATE INDEX idx_sentinels_status ON sentinels(status);
CREATE INDEX idx_sentinels_created_at ON sentinels(created_at);
CREATE INDEX idx_sentinels_expires_at ON sentinels(expires_at);

-- ============================================================================
-- 17. sketches
-- ============================================================================
CREATE TABLE IF NOT EXISTS sketches (
    id              TEXT PRIMARY KEY,
    title           TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'active',
    action_ids      TEXT NOT NULL DEFAULT '[]',
    project         TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    expires_at      TEXT,
    promoted_at     TEXT,
    discarded_at    TEXT
);

CREATE INDEX idx_sketches_status ON sketches(status);
CREATE INDEX idx_sketches_project ON sketches(project);
CREATE INDEX idx_sketches_created_at ON sketches(created_at);

-- ============================================================================
-- 18. crystals
-- ============================================================================
CREATE TABLE IF NOT EXISTS crystals (
    id                  TEXT PRIMARY KEY,
    narrative           TEXT NOT NULL,
    key_outcomes        TEXT NOT NULL DEFAULT '[]',
    files_affected      TEXT NOT NULL DEFAULT '[]',
    lessons             TEXT NOT NULL DEFAULT '[]',
    source_action_ids   TEXT NOT NULL DEFAULT '[]',
    session_id          TEXT,
    project             TEXT,
    created_at          TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_crystals_session_id ON crystals(session_id);
CREATE INDEX idx_crystals_project ON crystals(project);
CREATE INDEX idx_crystals_created_at ON crystals(created_at);

-- ============================================================================
-- 19. lessons
-- ============================================================================
CREATE TABLE IF NOT EXISTS lessons (
    id                  TEXT PRIMARY KEY,
    content             TEXT NOT NULL,
    context             TEXT NOT NULL DEFAULT '',
    confidence          REAL NOT NULL DEFAULT 0.5,
    reinforcements      INTEGER NOT NULL DEFAULT 0,
    source              TEXT NOT NULL DEFAULT 'manual',
    source_ids          TEXT NOT NULL DEFAULT '[]',
    project             TEXT,
    tags                TEXT NOT NULL DEFAULT '[]',
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now')),
    last_reinforced_at  TEXT,
    last_decayed_at     TEXT,
    decay_rate          REAL NOT NULL DEFAULT 0.01,
    deleted             INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_lessons_project ON lessons(project);
CREATE INDEX idx_lessons_confidence ON lessons(confidence);
CREATE INDEX idx_lessons_source ON lessons(source);
CREATE INDEX idx_lessons_created_at ON lessons(created_at);
CREATE INDEX idx_lessons_deleted ON lessons(deleted);

-- ============================================================================
-- 20. insights
-- ============================================================================
CREATE TABLE IF NOT EXISTS insights (
    id                      TEXT PRIMARY KEY,
    title                   TEXT NOT NULL,
    content                 TEXT NOT NULL,
    confidence              REAL NOT NULL DEFAULT 0.5,
    reinforcements          INTEGER NOT NULL DEFAULT 0,
    source_concept_cluster  TEXT NOT NULL DEFAULT '[]',
    source_memory_ids       TEXT NOT NULL DEFAULT '[]',
    source_lesson_ids       TEXT NOT NULL DEFAULT '[]',
    source_crystal_ids      TEXT NOT NULL DEFAULT '[]',
    project                 TEXT,
    tags                    TEXT NOT NULL DEFAULT '[]',
    created_at              TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at              TEXT NOT NULL DEFAULT (datetime('now')),
    last_reinforced_at      TEXT,
    last_decayed_at         TEXT,
    decay_rate              REAL NOT NULL DEFAULT 0.01,
    deleted                 INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_insights_project ON insights(project);
CREATE INDEX idx_insights_confidence ON insights(confidence);
CREATE INDEX idx_insights_created_at ON insights(created_at);
CREATE INDEX idx_insights_deleted ON insights(deleted);

-- ============================================================================
-- 21. facets
-- ============================================================================
CREATE TABLE IF NOT EXISTS facets (
    id          TEXT PRIMARY KEY,
    target_id   TEXT NOT NULL,
    target_type TEXT NOT NULL,
    dimension   TEXT NOT NULL,
    value       TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_facets_target_id ON facets(target_id);
CREATE INDEX idx_facets_target_type ON facets(target_type);
CREATE INDEX idx_facets_dimension ON facets(dimension);
CREATE INDEX idx_facets_target_id_target_type ON facets(target_id, target_type);

-- ============================================================================
-- 22. audit_log
-- ============================================================================
CREATE TABLE IF NOT EXISTS audit_log (
    id          TEXT PRIMARY KEY,
    action      TEXT NOT NULL,
    entity_id   TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    agent_id    TEXT,
    meta        TEXT NOT NULL DEFAULT '{}',
    timestamp   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_audit_log_action ON audit_log(action);
CREATE INDEX idx_audit_log_entity_id ON audit_log(entity_id);
CREATE INDEX idx_audit_log_entity_type ON audit_log(entity_type);
CREATE INDEX idx_audit_log_agent_id ON audit_log(agent_id);
CREATE INDEX idx_audit_log_timestamp ON audit_log(timestamp);

-- ============================================================================
-- 23. project_profiles
-- ============================================================================
CREATE TABLE IF NOT EXISTS project_profiles (
    project             TEXT PRIMARY KEY,
    updated_at          TEXT NOT NULL DEFAULT (datetime('now')),
    top_concepts        TEXT NOT NULL DEFAULT '[]',
    top_files           TEXT NOT NULL DEFAULT '[]',
    conventions         TEXT NOT NULL DEFAULT '[]',
    common_errors       TEXT NOT NULL DEFAULT '[]',
    recent_activity     TEXT NOT NULL DEFAULT '[]',
    session_count       INTEGER NOT NULL DEFAULT 0,
    total_observations  INTEGER NOT NULL DEFAULT 0,
    summary             TEXT
);

CREATE INDEX idx_project_profiles_updated_at ON project_profiles(updated_at);

-- ============================================================================
-- 24. mesh_peers
-- ============================================================================
CREATE TABLE IF NOT EXISTS mesh_peers (
    id              TEXT PRIMARY KEY,
    url             TEXT NOT NULL,
    name            TEXT NOT NULL,
    last_sync_at    TEXT,
    status          TEXT NOT NULL DEFAULT 'disconnected',
    shared_scopes   TEXT NOT NULL DEFAULT '[]',
    sync_filter     TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_mesh_peers_status ON mesh_peers(status);
CREATE INDEX idx_mesh_peers_name ON mesh_peers(name);

-- ============================================================================
-- 25. embeddings
-- ============================================================================
CREATE TABLE IF NOT EXISTS embeddings (
    obs_id      TEXT PRIMARY KEY,
    session_id  TEXT NOT NULL,
    embedding   BLOB NOT NULL,
    dimensions  INTEGER NOT NULL
);

CREATE INDEX idx_embeddings_session_id ON embeddings(session_id);

-- ============================================================================
-- 26. access_log
-- ============================================================================
CREATE TABLE IF NOT EXISTS access_log (
    memory_id   TEXT NOT NULL,
    accessed_at TEXT NOT NULL,
    PRIMARY KEY (memory_id, accessed_at)
);

CREATE INDEX idx_access_log_memory_id ON access_log(memory_id);
CREATE INDEX idx_access_log_accessed_at ON access_log(accessed_at);

-- ============================================================================
-- 27. snapshots
-- ============================================================================
CREATE TABLE IF NOT EXISTS snapshots (
    id          TEXT PRIMARY KEY,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    file_path   TEXT NOT NULL,
    size_bytes  INTEGER NOT NULL DEFAULT 0,
    description TEXT
);

CREATE INDEX idx_snapshots_created_at ON snapshots(created_at);

-- ============================================================================
-- 28. dedup_cache
-- ============================================================================
CREATE TABLE IF NOT EXISTS dedup_cache (
    hash        TEXT PRIMARY KEY,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_dedup_cache_created_at ON dedup_cache(created_at);
