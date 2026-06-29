-- Agent template queries (platform-level, DB-backed template library)
-- Replaces the file-based agenttmpl package.

-- name: ListAgentTemplates :many
SELECT * FROM agent_template
WHERE (sqlc.narg('category')::text IS NULL OR category = sqlc.narg('category'))
ORDER BY category, name;

-- name: GetAgentTemplate :one
SELECT * FROM agent_template
WHERE id = $1;

-- name: CreateAgentTemplate :one
INSERT INTO agent_template (
    name, description, category, icon, accent, tags,
    instructions, avatar_url, model, thinking_level, visibility,
    max_concurrent_tasks, custom_args, mcp_config, skill_ids, created_by
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11,
    $12, $13, $14, $15, $16
)
RETURNING *;

-- name: UpdateAgentTemplate :one
UPDATE agent_template SET
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    category = COALESCE(sqlc.narg('category'), category),
    icon = COALESCE(sqlc.narg('icon'), icon),
    accent = COALESCE(sqlc.narg('accent'), accent),
    tags = COALESCE(sqlc.narg('tags'), tags),
    instructions = COALESCE(sqlc.narg('instructions'), instructions),
    avatar_url = COALESCE(sqlc.narg('avatar_url'), avatar_url),
    model = COALESCE(sqlc.narg('model'), model),
    thinking_level = COALESCE(sqlc.narg('thinking_level'), thinking_level),
    visibility = COALESCE(sqlc.narg('visibility'), visibility),
    max_concurrent_tasks = COALESCE(sqlc.narg('max_concurrent_tasks'), max_concurrent_tasks),
    custom_args = COALESCE(sqlc.narg('custom_args'), custom_args),
    mcp_config = COALESCE(sqlc.narg('mcp_config'), mcp_config),
    skill_ids = COALESCE(sqlc.narg('skill_ids'), skill_ids),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteAgentTemplate :exec
DELETE FROM agent_template WHERE id = $1;

-- name: GetUserPlatformAdmin :one
SELECT platform_admin FROM "user" WHERE id = $1;
