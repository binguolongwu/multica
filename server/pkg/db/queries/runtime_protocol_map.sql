-- runtime_protocol_map CRUD (global table, no workspace_id)

-- name: ListRuntimeProtocolMap :many
SELECT * FROM runtime_protocol_map
ORDER BY protocol_family;

-- name: GetRuntimeProtocolMapByFamily :one
SELECT * FROM runtime_protocol_map
WHERE protocol_family = $1;

-- name: UpsertRuntimeProtocolMap :one
INSERT INTO runtime_protocol_map (protocol_family, api_type, env_var_api_key, env_var_base_url)
VALUES ($1, $2, $3, $4)
ON CONFLICT (protocol_family) DO UPDATE SET
    api_type = EXCLUDED.api_type,
    env_var_api_key = EXCLUDED.env_var_api_key,
    env_var_base_url = EXCLUDED.env_var_base_url,
    updated_at = now()
RETURNING *;

-- name: DeleteRuntimeProtocolMap :exec
DELETE FROM runtime_protocol_map WHERE protocol_family = $1;
