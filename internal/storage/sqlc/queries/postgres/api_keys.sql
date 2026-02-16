-- name: CreateAPIKey :exec
INSERT INTO api_keys (id, name, key_hash, prefix, permissions, enabled, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetAPIKeyByHash :one
SELECT id, name, key_hash, prefix, permissions, enabled, created_at, updated_at
FROM api_keys
WHERE key_hash = $1;

-- name: ListAPIKeys :many
SELECT id, name, key_hash, prefix, permissions, enabled, created_at, updated_at
FROM api_keys
ORDER BY created_at;

-- name: UpdateAPIKey :exec
UPDATE api_keys
SET name = $2, permissions = $3, enabled = $4, updated_at = $5
WHERE id = $1;

-- name: DeleteAPIKey :exec
DELETE FROM api_keys
WHERE id = $1;
