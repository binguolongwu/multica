-- Reverse 137_skill_type_system

ALTER TABLE agent_template RENAME COLUMN skill_ids TO skill_urls;

DELETE FROM skill_file WHERE skill_id IN (SELECT id FROM skill WHERE skill_type = 'builtin');
DELETE FROM skill WHERE skill_type = 'builtin';

ALTER TABLE skill DROP CONSTRAINT IF EXISTS ck_skill_workspace_required;
ALTER TABLE skill ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE skill DROP CONSTRAINT IF EXISTS ck_skill_type;
ALTER TABLE skill DROP COLUMN IF EXISTS skill_type;

DROP INDEX IF EXISTS idx_skill_unique_name;
ALTER TABLE skill ADD CONSTRAINT skill_workspace_id_name_key UNIQUE (workspace_id, name);
