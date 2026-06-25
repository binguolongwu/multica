-- Global LLM Provider registry. Each provider represents an API-compatible
-- LLM service (DeepSeek, OpenAI, Anthropic, etc.) with its authentication
-- credentials and environment variable names for injection into agent runtimes.
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

COMMENT ON COLUMN llm_provider.name IS
    'Human-readable provider name, globally unique (e.g. DeepSeek, OpenAI, 七牛视觉).';

COMMENT ON COLUMN llm_provider.api_base_url IS
    'API endpoint for LLM requests (e.g. https://api.deepseek.com/anthropic).';

COMMENT ON COLUMN llm_provider.api_key IS
    'Authentication key sent to the API. Plaintext storage with RBAC gating — '
    'only owners/admins can read this value; members see a masked placeholder.';

COMMENT ON COLUMN llm_provider.env_var_api_key IS
    'Environment variable name used to inject the API key into the agent '
    'runtime subprocess (e.g. ANTHROPIC_API_KEY, OPENAI_API_KEY).';

COMMENT ON COLUMN llm_provider.env_var_base_url IS
    'Environment variable name used to inject the API base URL into the agent '
    'runtime subprocess (e.g. ANTHROPIC_BASE_URL, OPENAI_BASE_URL).';
