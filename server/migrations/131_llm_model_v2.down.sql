DROP TABLE IF EXISTS llm_model CASCADE;

-- Restore v1 schema (migration 129)
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
