-- Global LLM Model catalog. Each model belongs to a provider and is
-- identified by its model_id (e.g. "deepseek-v4-pro[1m]", "gpt-5.5").
-- The server-side catalog is always available regardless of daemon status,
-- so the frontend model picker works even when no runtime is online.
CREATE TABLE llm_model (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id     UUID NOT NULL REFERENCES llm_provider(id) ON DELETE CASCADE,
    model_id        TEXT NOT NULL,
    display_name    TEXT NOT NULL DEFAULT '',
    capabilities    TEXT[] NOT NULL DEFAULT '{}',
    context_window  INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(provider_id, model_id)
);

COMMENT ON COLUMN llm_model.provider_id IS
    'FK to llm_provider. CASCADE delete: removing a provider removes all its models.';

COMMENT ON COLUMN llm_model.model_id IS
    'API-facing model identifier passed to the LLM CLI via --model flag '
    '(e.g. deepseek-v4-pro[1m], gpt-5.5, qwen-vl-max).';

COMMENT ON COLUMN llm_model.display_name IS
    'Human-readable label shown in the agent model picker dropdown.';

COMMENT ON COLUMN llm_model.capabilities IS
    'Array of capability tags: text, vision, code, tool_use. Used by the UI '
    'to filter models by agent role (e.g. vision for image-generation agents).';

COMMENT ON COLUMN llm_model.context_window IS
    'Maximum context window size in tokens. Informational — not enforced at '
    'the Multica layer; the LLM provider is responsible for its own limits.';

COMMENT ON COLUMN llm_model.updated_at IS
    'Set automatically by the application on UPDATE. Kept as a column default '
    'only for initial INSERT to match the created_at convention.';
