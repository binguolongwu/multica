-- name: PinAgent :exec
INSERT INTO chat_pinned_agent (user_id, agent_id, workspace_id, sort_order)
VALUES ($1, $2, $3, $4);

-- name: UnpinAgent :exec
DELETE FROM chat_pinned_agent WHERE user_id = $1 AND agent_id = $2 AND workspace_id = $3;

-- name: ListPinnedAgents :many
SELECT * FROM chat_pinned_agent
WHERE user_id = $1 AND workspace_id = $2
ORDER BY sort_order ASC;

-- name: CountPinnedAgents :one
SELECT COUNT(*) FROM chat_pinned_agent
WHERE user_id = $1 AND workspace_id = $2;
