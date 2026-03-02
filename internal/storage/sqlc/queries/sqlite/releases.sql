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

-- name: UpsertRelease :exec
INSERT INTO releases (
    id, application_id, version, platform, architecture, download_url,
    checksum, checksum_type, file_size, release_notes, release_date,
    required, minimum_version, metadata, created_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (application_id, version, platform, architecture) DO UPDATE SET
    id = excluded.id,
    download_url = excluded.download_url,
    checksum = excluded.checksum,
    checksum_type = excluded.checksum_type,
    file_size = excluded.file_size,
    release_notes = excluded.release_notes,
    release_date = excluded.release_date,
    required = excluded.required,
    minimum_version = excluded.minimum_version,
    metadata = excluded.metadata;

-- name: DeleteRelease :exec
DELETE FROM releases
WHERE application_id = ? AND version = ? AND platform = ? AND architecture = ?;