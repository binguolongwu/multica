-- 146: Add last_read_at cursor for per-creator unread tracking.
--
-- Semantics: last_read_at is the last time the creator viewed the session.
-- NULL means the session has never been read (treated as unread).
-- Unread count / status is computed as:
--   has_unread = (session.last_read_at IS NULL
--                 OR session.updated_at > session.last_read_at)
--
-- The idx_chat_session_creator_activity index supports the inbox-style
-- "sessions by creator, most recently active first" listing that powers
-- the chat sidebar.
--
-- Idempotent: column may already exist from manual patching; if so, alter
-- it to match the intended nullable semantics and migrate existing data.

DO $$
BEGIN
  -- Add column if it doesn't exist
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'chat_session' AND column_name = 'last_read_at'
  ) THEN
    ALTER TABLE chat_session ADD COLUMN last_read_at TIMESTAMPTZ;
    -- Migrate existing data: sessions with no unread tracked → mark as read now.
    UPDATE chat_session SET last_read_at = NOW() WHERE unread_since IS NULL;
    -- Sessions with unread_since set → leave last_read_at NULL (unread).
  ELSE
    -- Column already exists (e.g. manually patched). Ensure nullable.
    ALTER TABLE chat_session ALTER COLUMN last_read_at DROP NOT NULL;
    ALTER TABLE chat_session ALTER COLUMN last_read_at DROP DEFAULT;
    -- Re-apply data migration for safety.
    UPDATE chat_session SET last_read_at = NOW() WHERE unread_since IS NULL;
  END IF;
END $$;

-- Index for efficient listing sorted by activity per creator.
CREATE INDEX IF NOT EXISTS idx_chat_session_creator_activity
  ON chat_session(creator_id, updated_at DESC);
