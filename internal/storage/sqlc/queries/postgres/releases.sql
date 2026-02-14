-- name: GetReleasesByAppID :many
SELECT id, application_id, version, platform, architecture, download_url,
       checksum, checksum_type, file_size, release_notes, release_date,
       required, minimum_version, metadata, created_at
FROM releases
WHERE application_id = $1
ORDER BY release_date DESC;

-- name: GetReleaseByID :one
SELECT id, application_id, version, platform, architecture, download_url,
       checksum, checksum_type, file_size, release_notes, release_date,
       required, minimum_version, metadata, created_at
FROM releases
WHERE id = $1;

-- name: GetRelease :one
SELECT id, application_id, version, platform, architecture, download_url,
       checksum, checksum_type, file_size, release_notes, release_date,
       required, minimum_version, metadata, created_at
FROM releases
WHERE application_id = $1 AND version = $2 AND platform = $3 AND architecture = $4;

-- name: GetReleasesByPlatformArch :many
SELECT id, application_id, version, platform, architecture, download_url,
       checksum, checksum_type, file_size, release_notes, release_date,
       required, minimum_version, metadata, created_at
FROM releases
WHERE application_id = $1 AND platform = $2 AND architecture = $3
ORDER BY release_date DESC;

-- name: CreateRelease :exec
INSERT INTO releases (
    id, application_id, version, platform, architecture, download_url,
    checksum, checksum_type, file_size, release_notes, release_date,
    required, minimum_version, metadata, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15);

-- name: UpdateRelease :exec
UPDATE releases
SET download_url = $2, checksum = $3, checksum_type = $4, file_size = $5,
    release_notes = $6, release_date = $7, required = $8, minimum_version = $9, metadata = $10
WHERE id = $1;

-- name: DeleteRelease :exec
DELETE FROM releases
WHERE application_id = $1 AND version = $2 AND platform = $3 AND architecture = $4;