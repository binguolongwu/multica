-- Skill CRUD

-- name: ListSkillsByWorkspace :many
-- Includes platform and built-in skills (workspace_id IS NULL) plus
-- workspace-bound skills for the given workspace.
SELECT * FROM skill
WHERE workspace_id IS NULL OR workspace_id = $1
ORDER BY skill_type, name ASC;

-- name: ListSkillSummariesByWorkspace :many
-- Same as ListSkillsByWorkspace but omits the SKILL.md `content` column. Used
-- by list endpoints (CLI table, web list page) where the body is never read;
-- shipping it everywhere blew up payload size on workspaces with many skills
-- and caused 15s CLI timeouts from high-latency regions (GH multica-ai/multica#2174).
SELECT id, workspace_id, name, description, config, skill_type, is_builtin, source_skill_id, created_by, created_at, updated_at
FROM skill
WHERE workspace_id IS NULL OR workspace_id = $1
ORDER BY skill_type, name ASC;

-- name: GetSkill :one
SELECT * FROM skill
WHERE id = $1;

-- name: GetSkillInWorkspace :one
SELECT * FROM skill
WHERE id = $1 AND workspace_id = $2;

-- name: GetSkillByWorkspaceAndName :one
-- Used by agent-template materialization to implement find-or-create: when a
-- template references a skill by name that already exists in the workspace,
-- reuse the existing skill_id rather than INSERT (which would fail the
-- UNIQUE(workspace_id, name) constraint from migration 008).
SELECT * FROM skill
WHERE workspace_id = $1 AND name = $2;

-- name: CreateSkill :one
INSERT INTO skill (workspace_id, name, description, content, config, skill_type, is_builtin, source_skill_id, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateSkill :one
UPDATE skill SET
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    content = COALESCE(sqlc.narg('content'), content),
    config = COALESCE(sqlc.narg('config'), config),
    skill_type = COALESCE(sqlc.narg('skill_type'), skill_type),
    workspace_id = COALESCE(sqlc.narg('workspace_id'), workspace_id),
    source_skill_id = COALESCE(sqlc.narg('source_skill_id'), source_skill_id),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteSkill :exec
-- Defense-in-depth: workspace_id is a SQL-layer tenant guard. See DeleteIssue.
DELETE FROM skill WHERE id = $1 AND workspace_id = $2;

-- Skill File CRUD

-- name: ListSkillFiles :many
SELECT * FROM skill_file
WHERE skill_id = $1
ORDER BY path ASC;

-- name: GetSkillFile :one
SELECT * FROM skill_file
WHERE id = $1;

-- name: UpsertSkillFile :one
INSERT INTO skill_file (skill_id, path, content)
VALUES ($1, $2, $3)
ON CONFLICT (skill_id, path) DO UPDATE SET
    content = EXCLUDED.content,
    updated_at = now()
RETURNING *;

-- name: DeleteSkillFile :exec
DELETE FROM skill_file WHERE id = $1;

-- name: DeleteSkillFilesBySkill :exec
DELETE FROM skill_file WHERE skill_id = $1;

-- Agent-Skill junction

-- name: ListAgentSkills :many
SELECT s.* FROM skill s
JOIN agent_skill ask ON ask.skill_id = s.id
WHERE ask.agent_id = $1
ORDER BY s.name ASC;

-- name: ListAgentSkillSummaries :many
-- Summary variant for the agent skills list endpoint — omits `content` for
-- the same reason as ListSkillSummariesByWorkspace.
SELECT s.id, s.workspace_id, s.name, s.description, s.config, s.skill_type, s.is_builtin, s.source_skill_id, s.created_by, s.created_at, s.updated_at
FROM skill s
JOIN agent_skill ask ON ask.skill_id = s.id
WHERE ask.agent_id = $1
ORDER BY s.name ASC;

-- name: ListAgentSkillNamesByAgentIDs :many
SELECT ask.agent_id, s.name
FROM agent_skill ask
JOIN skill s ON s.id = ask.skill_id
WHERE ask.agent_id = ANY(sqlc.arg('agent_ids')::uuid[])
ORDER BY ask.agent_id, s.name ASC;

-- name: AddAgentSkill :exec
INSERT INTO agent_skill (agent_id, skill_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveAgentSkill :exec
DELETE FROM agent_skill
WHERE agent_id = $1 AND skill_id = $2;

-- name: RemoveAllAgentSkills :exec
DELETE FROM agent_skill WHERE agent_id = $1;

-- name: ListAgentSkillsByWorkspace :many
SELECT ask.agent_id, s.id, s.name, s.description, s.skill_type
FROM agent_skill ask
JOIN skill s ON s.id = ask.skill_id
WHERE s.workspace_id IS NULL OR s.workspace_id = $1
ORDER BY s.name ASC;

-- Skill type queries (platform / builtin skill discovery)

-- name: ListSkillsByType :many
SELECT * FROM skill
WHERE skill_type = $1 AND ($2::boolean IS NULL OR is_builtin = $2)
ORDER BY name ASC;

-- name: ListPlatformSkills :many
-- Returns platform skills (both is_builtin true and false) — the set available
-- for agent template skill_ids references and tenant installation.
SELECT id, workspace_id, name, description, config, skill_type, is_builtin, source_skill_id, created_by, created_at, updated_at
FROM skill
WHERE skill_type = 'platform'
ORDER BY is_builtin DESC, name ASC;

-- name: GetSkillBySourceAndWorkspace :one
-- Find workspace skill that was installed from a given platform skill.
SELECT * FROM skill
WHERE workspace_id = $1 AND source_skill_id = $2
LIMIT 1;

-- name: ListSkillsBySource :many
-- List all workspace copies of a given platform skill (for usage tracking).
SELECT * FROM skill
WHERE source_skill_id = $1
ORDER BY workspace_id, name ASC;

-- name: SyncUpstreamSkill :one
-- Overwrite workspace skill content from platform source.
UPDATE skill SET
    name = $2,
    description = $3,
    content = $4,
    config = $5,
    updated_at = now()
WHERE id = $1
RETURNING *;
