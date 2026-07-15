-- name: CreateLLMModel :one
INSERT INTO llm_model (workspace_id, provider_id, name, model_code, type, temperature, max_tokens, context_window, capabilities, sort, currency, input_price, output_price)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) RETURNING *;

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
    currency = COALESCE(sqlc.narg('currency'), currency),
    input_price = COALESCE(sqlc.narg('input_price'), input_price),
    output_price = COALESCE(sqlc.narg('output_price'), output_price),
    updated_at = now()
WHERE id = $1 AND workspace_id = $2 RETURNING *;

-- name: DeleteLLMModel :exec
DELETE FROM llm_model WHERE id = $1 AND workspace_id = $2;

-- name: UpsertLLMModel :one
-- Sync a remote model into llm_model. On conflict (same model_code), refresh
-- name, capabilities (caller passes the merged existing∪inferred set), and
-- pricing (only when the caller supplies pricing via narg); preserve the
-- user's manual type/temperature/max_tokens/context_window/sort/status.
INSERT INTO llm_model (
    workspace_id, provider_id, name, model_code, type,
    temperature, max_tokens, context_window, capabilities, sort,
    currency, input_price, output_price
)
VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    COALESCE(sqlc.narg('currency'), 'CNY'),
    COALESCE(sqlc.narg('input_price'), 0::float8),
    COALESCE(sqlc.narg('output_price'), 0::float8)
)
ON CONFLICT (workspace_id, provider_id, model_code)
DO UPDATE SET
    name = EXCLUDED.name,
    capabilities = EXCLUDED.capabilities,
    currency = COALESCE(sqlc.narg('currency'), llm_model.currency),
    input_price = COALESCE(sqlc.narg('input_price'), llm_model.input_price),
    output_price = COALESCE(sqlc.narg('output_price'), llm_model.output_price),
    updated_at = now()
RETURNING *;

-- name: ListLLMModelsByProvider :many
SELECT * FROM llm_model WHERE provider_id = $1 AND workspace_id = $2 ORDER BY sort, name;

-- name: ListLLMModelsForCatalog :many
SELECT m.*, p.name AS provider_name FROM llm_model m
JOIN llm_provider p ON p.id = m.provider_id
WHERE m.status = 1 AND p.status = 1
ORDER BY m.sort, m.name;

-- name: ListLLMModelsByWorkspace :many
SELECT model_code, input_price, output_price, currency FROM llm_model
WHERE workspace_id = $1 AND status = 1
ORDER BY model_code;
