-- name: CreateLLMModel :one
INSERT INTO llm_model (provider_id, model_id, display_name, capabilities, context_window)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: GetLLMModel :one
SELECT * FROM llm_model WHERE id = $1;

-- name: ListLLMModels :many
SELECT * FROM llm_model ORDER BY provider_id, display_name;

-- name: GetLLMModelByModelID :one
SELECT * FROM llm_model WHERE model_id = $1;

-- name: UpdateLLMModel :one
UPDATE llm_model SET
    model_id = COALESCE(sqlc.narg('model_id'), model_id),
    display_name = COALESCE(sqlc.narg('display_name'), display_name),
    capabilities = COALESCE(sqlc.narg('capabilities'), capabilities),
    context_window = COALESCE(sqlc.narg('context_window'), context_window),
    updated_at = now()
WHERE id = $1 RETURNING *;

-- name: DeleteLLMModel :exec
DELETE FROM llm_model WHERE id = $1;

-- name: ListLLMModelsByProvider :many
SELECT * FROM llm_model WHERE provider_id = $1 ORDER BY display_name;
