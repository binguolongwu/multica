-- 145 down: no-op.
-- Migration 145 only set Chinese COMMENTs on tables and columns. Comments are
-- add-only metadata with no behavioral side effect; clearing them on rollback
-- would not restore any prior behavior and would only strip discoverability.
-- The previous (English) comments, where any existed, are preserved in git
-- history if a restore is ever needed.
SELECT 1;
