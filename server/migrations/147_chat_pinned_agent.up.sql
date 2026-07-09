CREATE TABLE chat_pinned_agent (
    user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    sort_order SMALLINT NOT NULL DEFAULT 0 CHECK (sort_order >= 0 AND sort_order <= 4),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, agent_id, workspace_id)
);
