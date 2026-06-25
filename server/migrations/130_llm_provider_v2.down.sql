DROP TABLE IF EXISTS llm_provider_template CASCADE;
DROP TABLE IF EXISTS llm_provider CASCADE;

-- Restore v1 schema (migration 128)
CREATE TABLE llm_provider (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL UNIQUE,
    api_base_url    TEXT NOT NULL DEFAULT '',
    api_key         TEXT NOT NULL DEFAULT '',
    env_var_api_key TEXT NOT NULL DEFAULT 'ANTHROPIC_API_KEY',
    env_var_base_url TEXT NOT NULL DEFAULT 'ANTHROPIC_BASE_URL',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
