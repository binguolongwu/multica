-- Remove deprecated api_type and api_base_url columns from llm_provider.
-- These fields have been moved to llm_provider_endpoint table.
ALTER TABLE llm_provider DROP COLUMN IF EXISTS api_type;
ALTER TABLE llm_provider DROP COLUMN IF EXISTS api_base_url;
