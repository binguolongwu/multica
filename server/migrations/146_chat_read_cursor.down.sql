-- 146 down: revert last_read_at cursor and activity index.
DROP INDEX IF EXISTS idx_chat_session_creator_activity;
ALTER TABLE chat_session DROP COLUMN last_read_at;
