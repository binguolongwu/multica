-- name: CreateLLMModel :one
INSERT INTO llm_model (workspace_id, provider_id, name, model_code, type, temperature, max_tokens, context_window, capabilities, sort)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING *;

-- name: GetLLMModel :one
SELECT * FROM llm_model WHERE id = $1 AND workspace_id = $2;

-- name: ListLLMModels :many
SELECT * FROM llm_model WHERE workspace_id = $1 AND status = 1 ORDER BY provider_id, sort, name;

-- name: GetLLMModelByModelCode :one
SELECT * FROM llm_model WHERE model_code = $1 AND workspace_id = $2 AND status = 1;

-- name: UpdateLLMModel :one
UPDATE llm_model SET
    name = COALESCE(sqlc.narg('name'), name),
    model_code = COALESCE(sqlc.narg('model_code'), model_code),
    type = COALESCE(sqlc.narg('type'), type),
    temperature = COALESCE(sqlc.narg('temperature'), temperature),
    max_tokens = COALESCE(sqlc.narg('max_tokens'), max_tokens),
    context_window = COALESCE(sqlc.narg('context_window'), context_window),
    capabilities = COALESCE(sqlc.narg('capabilities'), capabilities),
    status = COALESCE(sqlc.narg('status'), status),
    sort = COALESCE(sqlc.narg('sort'), sort),
    updated_at = now()
WHERE id = $1 AND workspace_id = $2 RETURNING *;

-- name: DeleteLLMModel :exec
DELETE FROM llm_model WHERE id = $1 AND workspace_id = $2;

-- name: ListLLMModelsByProvider :many
SELECT * FROM llm_model WHERE provider_id = $1 AND workspace_id = $2 AND status = 1 ORDER BY sort, name;

-- name: ListLLMModelsForCatalog :many
SELECT m.*, p.name AS provider_name FROM llm_model m
JOIN llm_provider p ON p.id = m.provider_id
WHERE m.status = 1 AND p.status = 1
ORDER BY m.sort, m.name;
