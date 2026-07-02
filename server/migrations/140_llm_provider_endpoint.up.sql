-- 140_llm_provider_endpoint.up.sql

-- 1. Create runtime_protocol_map (global, replaces hardcoded llmRuntimeEnvVars)
CREATE TABLE runtime_protocol_map (
    protocol_map_id  UUID PRIMARY KEY DEFAULT uuidv7(),
    protocol_family  TEXT NOT NULL UNIQUE,
    api_type         TEXT NOT NULL,
    env_var_api_key  TEXT NOT NULL DEFAULT '',
    env_var_base_url TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seed from current llmRuntimeEnvVars hardcoded map
INSERT INTO runtime_protocol_map (protocol_map_id, protocol_family, api_type, env_var_api_key, env_var_base_url) VALUES
(uuidv7(), 'claude',      'anthropic',     'ANTHROPIC_API_KEY', 'ANTHROPIC_BASE_URL'),
(uuidv7(), 'codex',       'openai_chat',   'OPENAI_API_KEY',    'OPENAI_BASE_URL'),
(uuidv7(), 'hermes',      'anthropic',     'GLM_API_KEY',      ''),
(uuidv7(), 'copilot',     '',              '',                  ''),
(uuidv7(), 'opencode',    '',              '',                  ''),
(uuidv7(), 'openclaw',    'anthropic',     'ANTHROPIC_API_KEY', 'ANTHROPIC_BASE_URL'),
(uuidv7(), 'cursor',      'anthropic',     'ANTHROPIC_API_KEY', 'ANTHROPIC_BASE_URL'),
(uuidv7(), 'kimi',        'openai_chat',   'OPENAI_API_KEY',    'OPENAI_BASE_URL'),
(uuidv7(), 'kiro',        'anthropic',     'ANTHROPIC_API_KEY', 'ANTHROPIC_BASE_URL'),
(uuidv7(), 'codebuddy',   'anthropic',     'ANTHROPIC_API_KEY', 'ANTHROPIC_BASE_URL'),
(uuidv7(), 'antigravity', 'anthropic',     'ANTHROPIC_API_KEY', 'ANTHROPIC_BASE_URL'),
(uuidv7(), 'zeroclaw',    'anthropic',     'ANTHROPIC_API_KEY', 'ANTHROPIC_BASE_URL');

-- 2. Create llm_provider_endpoint
CREATE TABLE llm_provider_endpoint (
    endpoint_id   UUID PRIMARY KEY DEFAULT uuidv7(),
    provider_id   UUID NOT NULL REFERENCES llm_provider(id) ON DELETE CASCADE,
    workspace_id  UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    api_type      TEXT NOT NULL,
    api_base_url  TEXT NOT NULL DEFAULT '',
    status        SMALLINT NOT NULL DEFAULT 1,
    sort          INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id, provider_id, api_type)
);

CREATE INDEX idx_llm_provider_endpoint_provider ON llm_provider_endpoint(provider_id);
CREATE INDEX idx_llm_provider_endpoint_workspace ON llm_provider_endpoint(workspace_id);

-- 3. Migrate existing provider connections to endpoints
-- Each provider gets one endpoint from its old (api_type, api_base_url).
-- If api_base_url is empty, skip (no endpoint to create).
INSERT INTO llm_provider_endpoint (endpoint_id, provider_id, workspace_id, api_type, api_base_url, status, sort)
SELECT uuidv7(), id, workspace_id,
    CASE
        WHEN api_type = 'anthropic' THEN 'anthropic'
        WHEN api_type = 'openai' THEN 'openai_chat'
        ELSE api_type
    END,
    api_base_url, 1, 0
FROM llm_provider
WHERE api_base_url != '';
