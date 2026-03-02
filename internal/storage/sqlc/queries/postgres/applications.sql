-- name: GetAllApplications :many
SELECT id, name, description, platforms, config, created_at, updated_at
FROM applications
ORDER BY name;

-- name: GetApplicationByID :one
SELECT id, name, description, platforms, config, created_at, updated_at
FROM applications
WHERE id = $1;

-- name: UpsertApplication :exec
INSERT INTO applications (id, name, description, platforms, config, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    platforms = EXCLUDED.platforms,
    config = EXCLUDED.config,
    updated_at = EXCLUDED.updated_at;

-- name: DeleteApplication :exec
DELETE FROM applications
WHERE id = $1;