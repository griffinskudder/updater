-- name: GetReleasesByAppID :many
SELECT id, application_id, version, platform, architecture, download_url,
       checksum, checksum_type, file_size, release_notes, release_date,
       required, minimum_version, metadata, created_at
FROM releases
WHERE application_id = ?
ORDER BY release_date DESC;

-- name: GetReleaseByID :one
SELECT id, application_id, version, platform, architecture, download_url,
       checksum, checksum_type, file_size, release_notes, release_date,
       required, minimum_version, metadata, created_at
FROM releases
WHERE id = ?;

-- name: GetRelease :one
SELECT id, application_id, version, platform, architecture, download_url,
       checksum, checksum_type, file_size, release_notes, release_date,
       required, minimum_version, metadata, created_at
FROM releases
WHERE application_id = ? AND version = ? AND platform = ? AND architecture = ?;

-- name: GetReleasesByPlatformArch :many
SELECT id, application_id, version, platform, architecture, download_url,
       checksum, checksum_type, file_size, release_notes, release_date,
       required, minimum_version, metadata, created_at
FROM releases
WHERE application_id = ? AND platform = ? AND architecture = ?
ORDER BY release_date DESC;

-- name: CreateRelease :exec
INSERT INTO releases (
    id, application_id, version, platform, architecture, download_url,
    checksum, checksum_type, file_size, release_notes, release_date,
    required, minimum_version, metadata, created_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateRelease :exec
UPDATE releases
SET download_url = ?, checksum = ?, checksum_type = ?, file_size = ?,
    release_notes = ?, release_date = ?, required = ?, minimum_version = ?, metadata = ?
WHERE id = ?;

-- name: DeleteRelease :exec
DELETE FROM releases
WHERE application_id = ? AND version = ? AND platform = ? AND architecture = ?;