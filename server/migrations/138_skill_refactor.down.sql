-- 138_skill_refactor.down.sql

-- 1. Revert data: is_builtin=TRUE → skill_type='builtin'
UPDATE skill SET skill_type = CASE
  WHEN is_builtin THEN 'builtin'
  ELSE skill_type
END;

-- 2. Drop new constraints
ALTER TABLE skill
  DROP CONSTRAINT IF EXISTS ck_skill_type,
  DROP CONSTRAINT IF EXISTS ck_skill_workspace_required,
  DROP CONSTRAINT IF EXISTS ck_skill_builtin,
  DROP CONSTRAINT IF EXISTS ck_skill_builtin_ws;

-- 3. Drop new columns
ALTER TABLE skill
  DROP COLUMN IF EXISTS is_builtin,
  DROP COLUMN IF EXISTS source_skill_id;

-- 4. Restore old constraints (from migration 137)
ALTER TABLE skill
  ADD CONSTRAINT ck_skill_type CHECK (skill_type IN ('builtin', 'platform', 'workspace')),
  ADD CONSTRAINT ck_skill_workspace_required CHECK (
    (skill_type = 'workspace' AND workspace_id IS NOT NULL)
    OR (skill_type IN ('builtin', 'platform') AND workspace_id IS NULL)
  );
