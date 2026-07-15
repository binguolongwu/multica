-- name: CreateLLMProvider :one
INSERT INTO llm_provider (workspace_id, name, code, api_key, env_var_api_key, env_var_base_url, sort)
VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING *;

-- name: GetLLMProvider :one
SELECT * FROM llm_provider WHERE id = $1 AND workspace_id = $2;

-- name: ListLLMProviders :many
SELECT * FROM llm_provider WHERE workspace_id = $1 AND status = 1 ORDER BY sort, name;

-- name: UpdateLLMProvider :one
UPDATE llm_provider SET
    name = COALESCE(sqlc.narg('name'), name),
    code = COALESCE(sqlc.narg('code'), code),
    api_key = COALESCE(sqlc.narg('api_key'), api_key),
    env_var_api_key = COALESCE(sqlc.narg('env_var_api_key'), env_var_api_key),
    env_var_base_url = COALESCE(sqlc.narg('env_var_base_url'), env_var_base_url),
    status = COALESCE(sqlc.narg('status'), status),
    sort = COALESCE(sqlc.narg('sort'), sort),
    updated_at = now()
WHERE id = $1 AND workspace_id = $2 RETURNING *;

-- name: DeleteLLMProvider :exec
DELETE FROM llm_provider WHERE id = $1 AND workspace_id = $2;

-- name: GetLLMProviderByModelCode :one
SELECT p.* FROM llm_provider p
JOIN llm_model m ON m.provider_id = p.id
WHERE m.model_code = $1 AND m.workspace_id = $2 AND p.status = 1;

-- name: ListLLMProviderTemplates :many
SELECT * FROM llm_provider_template WHERE status = 1 ORDER BY sort, name;

-- name: GetLLMEndpointForInjection :one
-- 4-table JOIN: model → provider → endpoint → protocol_map
-- Returns the endpoint matching the runtime's protocol_family.
SELECT
    p.api_key,
    e.api_base_url,
    rpm.env_var_api_key,
    rpm.env_var_base_url
FROM llm_model t
JOIN llm_provider p ON p.id = t.provider_id
JOIN llm_provider_endpoint e ON e.provider_id = p.id AND e.workspace_id = t.workspace_id
JOIN runtime_protocol_map rpm ON rpm.api_type = e.api_type
WHERE t.model_code = $1
  AND t.workspace_id = $2
  AND rpm.protocol_family = $3
  AND e.status = 1
  AND p.status = 1
  AND t.status = 1;
