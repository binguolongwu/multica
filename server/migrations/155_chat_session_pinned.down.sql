-- Revert pinned_at back to is_pinned.
ALTER TABLE chat_session ADD COLUMN IF NOT EXISTS is_pinned BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE chat_session SET is_pinned = TRUE WHERE pinned_at IS NOT NULL;
ALTER TABLE chat_session DROP COLUMN IF EXISTS pinned_at;
