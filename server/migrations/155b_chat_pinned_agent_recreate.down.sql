-- Revert to the old chat_pinned_agent schema.
DROP TABLE IF EXISTS chat_pinned_agent;

CREATE TABLE chat_pinned_agent (
    user_id UUID NOT NULL,
    agent_id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    sort_order SMALLINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, agent_id, workspace_id)
);
