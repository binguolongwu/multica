-- Down is a no-op: this migration backfills instruction/skill content.
-- Reverting would discard the rewrite; re-running up is idempotent.
SELECT 1;
