-- Add workspace_id to llm_provider and llm_model for workspace isolation.
-- Existing global data is dropped (tables were empty in production, seed data on dev).
ALTER TABLE llm_model DROP CONSTRAINT IF EXISTS llm_model_provider_id_fkey;
DROP TABLE IF EXISTS llm_model CASCADE;
DROP TABLE IF EXISTS llm_provider CASCADE;

CREATE TABLE llm_provider (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    code            TEXT NOT NULL,
    api_type        TEXT NOT NULL DEFAULT 'openai',
    api_base_url    TEXT NOT NULL DEFAULT '',
    api_key         TEXT NOT NULL DEFAULT '',
    env_var_api_key TEXT NOT NULL DEFAULT 'ANTHROPIC_API_KEY',
    env_var_base_url TEXT NOT NULL DEFAULT 'ANTHROPIC_BASE_URL',
    status          SMALLINT NOT NULL DEFAULT 1,
    sort            INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id, code)
);
COMMENT ON COLUMN llm_provider.workspace_id IS 'FK to workspace. Provider is scoped to a single workspace.';

CREATE TABLE llm_model (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    provider_id     UUID NOT NULL REFERENCES llm_provider(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    model_code      TEXT NOT NULL,
    type            SMALLINT NOT NULL DEFAULT 1,
    temperature     DOUBLE PRECISION NOT NULL DEFAULT 0.7,
    max_tokens      INT NOT NULL DEFAULT 4096,
    context_window  INT NOT NULL DEFAULT 0,
    capabilities    TEXT[] NOT NULL DEFAULT '{}',
    status          SMALLINT NOT NULL DEFAULT 1,
    sort            INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id, provider_id, model_code)
);
COMMENT ON COLUMN llm_model.workspace_id IS 'FK to workspace for direct workspace-scoped queries.';
