-- name: GetReleasesByAppID :many
SELECT id, application_id, version, platform, architecture, download_url,
       checksum, checksum_type, file_size, release_notes, release_date,
       required, minimum_version, metadata, created_at,
       version_major, version_minor, version_patch, version_pre_release
FROM releases
WHERE application_id = ?
ORDER BY release_date DESC;

-- name: GetReleaseByID :one
SELECT id, application_id, version, platform, architecture, download_url,
       checksum, checksum_type, file_size, release_notes, release_date,
       required, minimum_version, metadata, created_at,
       version_major, version_minor, version_patch, version_pre_release
FROM releases
WHERE id = ?;

-- name: GetRelease :one
SELECT id, application_id, version, platform, architecture, download_url,
       checksum, checksum_type, file_size, release_notes, release_date,
       required, minimum_version, metadata, created_at,
       version_major, version_minor, version_patch, version_pre_release
FROM releases
WHERE application_id = ? AND version = ? AND platform = ? AND architecture = ?;

-- name: GetReleasesByPlatformArch :many
SELECT id, application_id, version, platform, architecture, download_url,
       checksum, checksum_type, file_size, release_notes, release_date,
       required, minimum_version, metadata, created_at,
       version_major, version_minor, version_patch, version_pre_release
FROM releases
WHERE application_id = ? AND platform = ? AND architecture = ?
ORDER BY release_date DESC;

-- name: UpsertRelease :exec
INSERT INTO releases (
    id, application_id, version, platform, architecture, download_url,
    checksum, checksum_type, file_size, release_notes, release_date,
    required, minimum_version, metadata, created_at,
    version_major, version_minor, version_patch, version_pre_release
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (application_id, version, platform, architecture) DO UPDATE SET
    download_url        = excluded.download_url,
    checksum            = excluded.checksum,
    checksum_type       = excluded.checksum_type,
    file_size           = excluded.file_size,
    release_notes       = excluded.release_notes,
    release_date        = excluded.release_date,
    required            = excluded.required,
    minimum_version     = excluded.minimum_version,
    metadata            = excluded.metadata,
    version_major       = excluded.version_major,
    version_minor       = excluded.version_minor,
    version_patch       = excluded.version_patch,
    version_pre_release = excluded.version_pre_release;

-- name: DeleteRelease :exec
DELETE FROM releases
WHERE application_id = ? AND version = ? AND platform = ? AND architecture = ?;

-- name: GetLatestStableRelease :one
SELECT id, application_id, version, platform, architecture, download_url,
       checksum, checksum_type, file_size, release_notes, release_date,
       required, minimum_version, metadata, created_at,
       version_major, version_minor, version_patch, version_pre_release
FROM releases
WHERE application_id = ? AND platform = ? AND architecture = ?
  AND version_pre_release IS NULL
ORDER BY version_major DESC, version_minor DESC, version_patch DESC
LIMIT 1;

-- name: GetApplicationStats :one
WITH app_releases AS (
    SELECT * FROM releases WHERE application_id = ?
)
SELECT
    COUNT(*) AS total_releases,
    SUM(CASE WHEN required THEN 1 ELSE 0 END) AS required_releases,
    COUNT(DISTINCT platform) AS platform_count,
    MAX(release_date) AS latest_release_date,
    (
        SELECT version FROM app_releases
        ORDER BY version_major DESC, version_minor DESC, version_patch DESC,
                 (version_pre_release IS NULL) DESC,
                 version_pre_release ASC
        LIMIT 1
    ) AS latest_version
FROM app_releases;