-- 005_memory_pinned.sql
--
-- "Pin" uma memória pra protegê-la do decay sweep. Usado pra preservar
-- conhecimento crítico (decisões arquiteturais, identidade do projeto)
-- mesmo quando ele é raramente reforçado.
--
-- Default 0 (não-pinned) preserva comportamento atual. O scheduler
-- pula linhas com pinned = 1 quando arquiva memórias antigas/fracas.

ALTER TABLE memories ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_memories_pinned ON memories(pinned) WHERE pinned = 1;
