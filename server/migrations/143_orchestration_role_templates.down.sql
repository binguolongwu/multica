-- 143 down: no-op.
-- These four templates are system-owned seed data; removing them on rollback
-- would orphan agents already created from them. Down is a no-op, matching
-- the policy established by 141/142 for system-owned template backfills.
SELECT 1;
