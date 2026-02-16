-- name: CreateAPIKey :exec
INSERT INTO api_keys (id, name, key_hash, prefix, permissions, enabled, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetAPIKeyByHash :one
SELECT id, name, key_hash, prefix, permissions, enabled, created_at, updated_at
FROM api_keys
WHERE key_hash = ?;

-- name: ListAPIKeys :many
SELECT id, name, key_hash, prefix, permissions, enabled, created_at, updated_at
FROM api_keys
ORDER BY created_at;

-- name: UpdateAPIKey :exec
UPDATE api_keys
SET name = ?, permissions = ?, enabled = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteAPIKey :exec
DELETE FROM api_keys
WHERE id = ?;
