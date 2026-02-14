-- name: GetAllApplications :many
SELECT id, name, description, platforms, config, created_at, updated_at
FROM applications
ORDER BY name;

-- name: GetApplicationByID :one
SELECT id, name, description, platforms, config, created_at, updated_at
FROM applications
WHERE id = $1;

-- name: CreateApplication :exec
INSERT INTO applications (id, name, description, platforms, config, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: UpdateApplication :exec
UPDATE applications
SET name = $2, description = $3, platforms = $4, config = $5, updated_at = $6
WHERE id = $1;

-- name: DeleteApplication :exec
DELETE FROM applications
WHERE id = $1;