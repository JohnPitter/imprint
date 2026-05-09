-- 003_actions_session_id.sql
--
-- Adds a session_id column to actions so the kanban can show which Claude
-- Code session each task belongs to. Existing rows are left with a NULL
-- session_id; the frontend treats NULL as "unknown session" and skips the
-- badge, which is the correct behavior for actions that predate this
-- migration.
--
-- The column is intentionally nullable (no FK constraint) so the migration
-- runs cleanly against any database state, including pre-bootstrap
-- installs and shared-DB setups where some sessions may have been pruned.

ALTER TABLE actions ADD COLUMN session_id TEXT;
CREATE INDEX IF NOT EXISTS idx_actions_session_id ON actions(session_id);
