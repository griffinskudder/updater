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

-- name: UpsertRelease :exec
INSERT INTO releases (
    id, application_id, version, platform, architecture, download_url,
    checksum, checksum_type, file_size, release_notes, release_date,
    required, minimum_version, metadata, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
ON CONFLICT (application_id, version, platform, architecture) DO UPDATE SET
    download_url = EXCLUDED.download_url,
    checksum = EXCLUDED.checksum,
    checksum_type = EXCLUDED.checksum_type,
    file_size = EXCLUDED.file_size,
    release_notes = EXCLUDED.release_notes,
    release_date = EXCLUDED.release_date,
    required = EXCLUDED.required,
    minimum_version = EXCLUDED.minimum_version,
    metadata = EXCLUDED.metadata;

-- name: DeleteRelease :exec
DELETE FROM releases
WHERE application_id = $1 AND version = $2 AND platform = $3 AND architecture = $4;