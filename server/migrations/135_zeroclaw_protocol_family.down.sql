-- Revert: remove 'zeroclaw' from the protocol_family CHECK constraint.
-- Restores the original set from migration 120.

DO $$
DECLARE
    constraint_name text;
BEGIN
    SELECT con.conname INTO constraint_name
    FROM pg_constraint con
    JOIN pg_class rel ON rel.oid = con.conrelid
    WHERE rel.relname = 'runtime_profile'
      AND con.contype = 'c'
      AND pg_get_constraintdef(con.oid) LIKE '%protocol_family%';

    IF constraint_name IS NOT NULL THEN
        EXECUTE format('ALTER TABLE runtime_profile DROP CONSTRAINT %I', constraint_name);
    END IF;
END $$;

ALTER TABLE runtime_profile ADD CONSTRAINT runtime_profile_protocol_family_check CHECK (protocol_family IN (
    'claude',
    'codebuddy',
    'codex',
    'copilot',
    'opencode',
    'openclaw',
    'hermes',
    'gemini',
    'pi',
    'cursor',
    'kimi',
    'kiro',
    'antigravity'
));
