-- v2 LLM Model catalog with type classification and model parameters.
-- Replaces the v1 llm_model table (129).
DROP TABLE IF EXISTS llm_model CASCADE;

CREATE TABLE llm_model (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
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
    UNIQUE(provider_id, model_code)
);

COMMENT ON COLUMN llm_model.provider_id IS 'FK to llm_provider. CASCADE delete.';
COMMENT ON COLUMN llm_model.name IS 'Human-readable model name (e.g. DeepSeek-V4-Pro).';
COMMENT ON COLUMN llm_model.model_code IS 'API-facing identifier passed to LLM CLI via --model (e.g. deepseek-v4-pro).';
COMMENT ON COLUMN llm_model.type IS 'Model type: 1=LLM对话, 2=视觉, 3=生图, 4=嵌入, 5=语音.';
COMMENT ON COLUMN llm_model.temperature IS 'Default sampling temperature (0.0-2.0).';
COMMENT ON COLUMN llm_model.max_tokens IS 'Default max output tokens per request.';
COMMENT ON COLUMN llm_model.context_window IS 'Max context window in tokens. Informational only.';
COMMENT ON COLUMN llm_model.capabilities IS 'Array of tags: text, vision, code, tool_use.';
COMMENT ON COLUMN llm_model.status IS '0=disabled, 1=enabled.';
COMMENT ON COLUMN llm_model.sort IS 'Display sort order within provider (asc).';
