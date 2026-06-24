-- name: CreateOSSProviderConfig :one
INSERT INTO oss_provider_config (
    workspace_id, name, provider, bucket, region, endpoint,
    access_key, secret_key_encrypted, custom_domain, folder_prefix, is_default
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetOSSProviderConfig :one
SELECT * FROM oss_provider_config WHERE id = $1 AND workspace_id = $2;

-- name: ListOSSProviderConfigs :many
SELECT * FROM oss_provider_config WHERE workspace_id = $1 ORDER BY created_at ASC;

-- name: UpdateOSSProviderConfig :one
UPDATE oss_provider_config SET
    name = $2,
    provider = $3,
    bucket = $4,
    region = $5,
    endpoint = $6,
    access_key = $7,
    secret_key_encrypted = COALESCE(sqlc.narg('secret_key_encrypted'), secret_key_encrypted),
    custom_domain = $8,
    folder_prefix = $9,
    is_default = $10,
    updated_at = now()
WHERE id = $1 AND workspace_id = $11
RETURNING *;

-- name: DeleteOSSProviderConfig :exec
DELETE FROM oss_provider_config WHERE id = $1 AND workspace_id = $2;

-- name: CreateOSSObject :one
INSERT INTO oss_object (config_id, key, filename, size_bytes, content_type, uploaded_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListOSSObjects :many
SELECT * FROM oss_object WHERE config_id = $1 ORDER BY created_at DESC;

-- name: GetOSSObject :one
SELECT * FROM oss_object WHERE id = $1 AND config_id = $2;

-- name: DeleteOSSObject :exec
DELETE FROM oss_object WHERE id = $1 AND config_id = $2;

-- name: DeleteOSSObjectByKey :exec
DELETE FROM oss_object WHERE config_id = $1 AND key = $2;

-- name: ListOSSObjectsByPrefix :many
SELECT * FROM oss_object WHERE config_id = $1 AND key LIKE $2 ORDER BY created_at DESC;
