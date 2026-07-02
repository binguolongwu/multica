-- llm_provider_endpoint CRUD

-- name: CreateLLMProviderEndpoint :one
INSERT INTO llm_provider_endpoint (provider_id, workspace_id, api_type, api_base_url, status, sort)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetLLMProviderEndpoint :one
SELECT * FROM llm_provider_endpoint
WHERE endpoint_id = $1 AND workspace_id = $2;

-- name: ListLLMProviderEndpoints :many
SELECT * FROM llm_provider_endpoint
WHERE provider_id = $1 AND workspace_id = $2
ORDER BY sort, api_type;

-- name: UpdateLLMProviderEndpoint :one
UPDATE llm_provider_endpoint SET
    api_type = COALESCE(sqlc.narg('api_type'), api_type),
    api_base_url = COALESCE(sqlc.narg('api_base_url'), api_base_url),
    status = COALESCE(sqlc.narg('status'), status),
    sort = COALESCE(sqlc.narg('sort'), sort),
    updated_at = now()
WHERE endpoint_id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteLLMProviderEndpoint :exec
DELETE FROM llm_provider_endpoint
WHERE endpoint_id = $1 AND workspace_id = $2;
