ALTER TABLE wiki_source DROP COLUMN IF EXISTS ingested_to_path;
ALTER TABLE wiki_page DROP COLUMN IF EXISTS validation_warnings;
ALTER TABLE wiki_space DROP COLUMN IF EXISTS template;
ALTER TABLE wiki_space DROP COLUMN IF EXISTS default_agent_id;
