-- 138_skill_refactor.up.sql

-- 1. Add new columns
ALTER TABLE skill
  ADD COLUMN IF NOT EXISTS is_builtin BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS source_skill_id UUID REFERENCES skill(id) ON DELETE SET NULL;

-- 2. Migrate data: skill_type='builtin' → skill_type='platform', is_builtin=TRUE
UPDATE skill SET
  is_builtin = CASE WHEN skill_type = 'builtin' THEN TRUE ELSE FALSE END,
  skill_type = CASE WHEN skill_type IN ('builtin', 'platform') THEN 'platform' ELSE 'workspace' END;

-- 3. Drop old constraints (from migration 137)
ALTER TABLE skill DROP CONSTRAINT IF EXISTS ck_skill_type;
ALTER TABLE skill DROP CONSTRAINT IF EXISTS ck_skill_workspace_required;

-- 4. Add new constraints
ALTER TABLE skill
  ADD CONSTRAINT ck_skill_type CHECK (skill_type IN ('platform', 'workspace')),
  ADD CONSTRAINT ck_skill_workspace_required CHECK (
    (skill_type = 'workspace' AND workspace_id IS NOT NULL)
    OR (skill_type = 'platform' AND workspace_id IS NULL)
  ),
  ADD CONSTRAINT ck_skill_builtin CHECK (is_builtin = FALSE OR skill_type = 'platform'),
  ADD CONSTRAINT ck_skill_builtin_ws CHECK (is_builtin = FALSE OR workspace_id IS NULL);
