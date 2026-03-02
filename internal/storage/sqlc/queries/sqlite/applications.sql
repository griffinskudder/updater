-- name: GetAllApplications :many
SELECT id, name, description, platforms, config, created_at, updated_at
FROM applications
ORDER BY name;

-- name: GetApplicationByID :one
SELECT id, name, description, platforms, config, created_at, updated_at
FROM applications
WHERE id = ?;

-- name: UpsertApplication :exec
INSERT INTO applications (id, name, description, platforms, config, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    name = excluded.name,
    description = excluded.description,
    platforms = excluded.platforms,
    config = excluded.config,
    updated_at = excluded.updated_at;

-- name: DeleteApplication :exec
DELETE FROM applications
WHERE id = ?;