-- name: GetAllApplications :many
SELECT id, name, description, platforms, config, created_at, updated_at
FROM applications
ORDER BY name;

-- name: GetApplicationByID :one
SELECT id, name, description, platforms, config, created_at, updated_at
FROM applications
WHERE id = ?;

-- name: CreateApplication :exec
INSERT INTO applications (id, name, description, platforms, config, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateApplication :exec
UPDATE applications
SET name = ?, description = ?, platforms = ?, config = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteApplication :exec
DELETE FROM applications
WHERE id = ?;