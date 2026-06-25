-- v2 LLM Provider registry with template support and api_type classification.
-- Replaces the v1 llm_provider table (128).
DROP TABLE IF EXISTS llm_model CASCADE;
DROP TABLE IF EXISTS llm_provider CASCADE;

CREATE TABLE llm_provider (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL UNIQUE,
    code            TEXT NOT NULL UNIQUE,
    api_type        TEXT NOT NULL DEFAULT 'openai',
    api_base_url    TEXT NOT NULL DEFAULT '',
    api_key         TEXT NOT NULL DEFAULT '',
    env_var_api_key TEXT NOT NULL DEFAULT 'ANTHROPIC_API_KEY',
    env_var_base_url TEXT NOT NULL DEFAULT 'ANTHROPIC_BASE_URL',
    status          SMALLINT NOT NULL DEFAULT 1,
    sort            INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON COLUMN llm_provider.name IS 'Human-readable provider name (e.g. DeepSeek, OpenAI).';
COMMENT ON COLUMN llm_provider.code IS 'Machine-friendly short code, globally unique (e.g. deepseek, openai).';
COMMENT ON COLUMN llm_provider.api_type IS 'API protocol family: openai, anthropic. Determines default env var naming.';
COMMENT ON COLUMN llm_provider.api_base_url IS 'API endpoint base URL.';
COMMENT ON COLUMN llm_provider.api_key IS 'Authentication key. Masked in list responses for all users.';
COMMENT ON COLUMN llm_provider.env_var_api_key IS 'Env var name for API key injection into agent runtime.';
COMMENT ON COLUMN llm_provider.env_var_base_url IS 'Env var name for API base URL injection into agent runtime.';
COMMENT ON COLUMN llm_provider.status IS '0=disabled, 1=enabled. Disabled providers are hidden from agent model picker.';
COMMENT ON COLUMN llm_provider.sort IS 'Display sort order (asc).';

-- Pre-configured provider templates (no api_key). Users copy from these to
-- quickly set up common providers without typing all the connection details.
CREATE TABLE llm_provider_template (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL UNIQUE,
    code            TEXT NOT NULL UNIQUE,
    api_type        TEXT NOT NULL DEFAULT 'openai',
    api_base_url    TEXT NOT NULL DEFAULT '',
    env_var_api_key TEXT NOT NULL DEFAULT 'ANTHROPIC_API_KEY',
    env_var_base_url TEXT NOT NULL DEFAULT 'ANTHROPIC_BASE_URL',
    anthropic_api_url TEXT NOT NULL DEFAULT '',
    sort            INT NOT NULL DEFAULT 0,
    status          SMALLINT NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON COLUMN llm_provider_template.name IS 'Template display name.';
COMMENT ON COLUMN llm_provider_template.code IS 'Machine-friendly short code, globally unique.';
COMMENT ON COLUMN llm_provider_template.api_type IS 'API protocol family: openai, anthropic.';
COMMENT ON COLUMN llm_provider_template.api_base_url IS 'Default API endpoint for this provider type.';
COMMENT ON COLUMN llm_provider_template.env_var_api_key IS 'Env var name for API key injection.';
COMMENT ON COLUMN llm_provider_template.env_var_base_url IS 'Env var name for API base URL injection.';
COMMENT ON COLUMN llm_provider_template.anthropic_api_url IS 'Alternative Anthropic-compatible endpoint when api_type=anthropic.';
COMMENT ON COLUMN llm_provider_template.status IS '0=disabled, 1=enabled.';
