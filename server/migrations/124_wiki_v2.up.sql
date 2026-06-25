-- Add wiki V2 columns: agent binding, template, validation, ingestion tracking.
ALTER TABLE wiki_space ADD COLUMN IF NOT EXISTS default_agent_id UUID REFERENCES agent(id) ON DELETE SET NULL;
ALTER TABLE wiki_space ADD COLUMN IF NOT EXISTS template TEXT NOT NULL DEFAULT 'general';
ALTER TABLE wiki_page ADD COLUMN IF NOT EXISTS validation_warnings JSONB NOT NULL DEFAULT '[]';
ALTER TABLE wiki_source ADD COLUMN IF NOT EXISTS ingested_to_path TEXT;
