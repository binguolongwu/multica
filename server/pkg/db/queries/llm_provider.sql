-- name: CreateLLMProvider :one
INSERT INTO llm_provider (name, api_base_url, api_key, env_var_api_key, env_var_base_url)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: GetLLMProvider :one
SELECT * FROM llm_provider WHERE id = $1;

-- name: ListLLMProviders :many
SELECT * FROM llm_provider ORDER BY name;

-- name: UpdateLLMProvider :one
UPDATE llm_provider SET
    name = COALESCE(sqlc.narg('name'), name),
    api_base_url = COALESCE(sqlc.narg('api_base_url'), api_base_url),
    api_key = COALESCE(sqlc.narg('api_key'), api_key),
    env_var_api_key = COALESCE(sqlc.narg('env_var_api_key'), env_var_api_key),
    env_var_base_url = COALESCE(sqlc.narg('env_var_base_url'), env_var_base_url),
    updated_at = now()
WHERE id = $1 RETURNING *;

-- name: DeleteLLMProvider :exec
DELETE FROM llm_provider WHERE id = $1;

-- name: GetLLMProviderByModelID :one
SELECT p.* FROM llm_provider p
JOIN llm_model m ON m.provider_id = p.id
WHERE m.model_id = $1;
