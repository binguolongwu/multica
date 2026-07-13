-- Replace is_pinned BOOLEAN with pinned_at TIMESTAMPTZ (MUL-4295).
--
-- pinned_at doubles as both the boolean flag (NULL = not pinned) and the sort
-- key within the pinned group (most-recently pinned first). The old is_pinned
-- column is migrated and dropped.

-- 1. Add the new pinned_at column alongside the old is_pinned.
ALTER TABLE chat_session ADD COLUMN IF NOT EXISTS pinned_at TIMESTAMPTZ;

-- 2. Migrate existing pinned sessions: use updated_at as the approximate
--    pin timestamp since we don't have a precise pin time.
UPDATE chat_session SET pinned_at = updated_at WHERE is_pinned = TRUE;

-- 3. Drop the old is_pinned column now that data is migrated.
ALTER TABLE chat_session DROP COLUMN IF EXISTS is_pinned;
