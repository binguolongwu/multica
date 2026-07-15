-- Restore api_type and api_base_url columns to llm_provider.
ALTER TABLE llm_provider ADD COLUMN IF NOT EXISTS api_type text NOT NULL DEFAULT 'openai';
ALTER TABLE llm_provider ADD COLUMN IF NOT EXISTS api_base_url text NOT NULL DEFAULT '';
