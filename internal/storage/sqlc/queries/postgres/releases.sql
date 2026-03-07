-- name: GetReleasesByAppID :many
SELECT id, application_id, version, platform, architecture, download_url,
       checksum, checksum_type, file_size, release_notes, release_date,
       required, minimum_version, metadata, created_at,
       version_major, version_minor, version_patch, version_pre_release
FROM releases
WHERE application_id = $1
ORDER BY release_date DESC;

-- name: GetReleaseByID :one
SELECT id, application_id, version, platform, architecture, download_url,
       checksum, checksum_type, file_size, release_notes, release_date,
       required, minimum_version, metadata, created_at,
       version_major, version_minor, version_patch, version_pre_release
FROM releases
WHERE id = $1;

-- name: GetRelease :one
SELECT id, application_id, version, platform, architecture, download_url,
       checksum, checksum_type, file_size, release_notes, release_date,
       required, minimum_version, metadata, created_at,
       version_major, version_minor, version_patch, version_pre_release
FROM releases
WHERE application_id = $1 AND version = $2 AND platform = $3 AND architecture = $4;

-- name: GetReleasesByPlatformArch :many
SELECT id, application_id, version, platform, architecture, download_url,
       checksum, checksum_type, file_size, release_notes, release_date,
       required, minimum_version, metadata, created_at,
       version_major, version_minor, version_patch, version_pre_release
FROM releases
WHERE application_id = $1 AND platform = $2 AND architecture = $3
ORDER BY release_date DESC;

-- name: UpsertRelease :exec
INSERT INTO releases (
    id, application_id, version, platform, architecture, download_url,
    checksum, checksum_type, file_size, release_notes, release_date,
    required, minimum_version, metadata, created_at,
    version_major, version_minor, version_patch, version_pre_release
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
ON CONFLICT (application_id, version, platform, architecture) DO UPDATE SET
    download_url        = EXCLUDED.download_url,
    checksum            = EXCLUDED.checksum,
    checksum_type       = EXCLUDED.checksum_type,
    file_size           = EXCLUDED.file_size,
    release_notes       = EXCLUDED.release_notes,
    release_date        = EXCLUDED.release_date,
    required            = EXCLUDED.required,
    minimum_version     = EXCLUDED.minimum_version,
    metadata            = EXCLUDED.metadata,
    version_major       = EXCLUDED.version_major,
    version_minor       = EXCLUDED.version_minor,
    version_patch       = EXCLUDED.version_patch,
    version_pre_release = EXCLUDED.version_pre_release;

-- name: DeleteRelease :exec
DELETE FROM releases
WHERE application_id = $1 AND version = $2 AND platform = $3 AND architecture = $4;

-- name: GetLatestStableRelease :one
SELECT id, application_id, version, platform, architecture, download_url,
       checksum, checksum_type, file_size, release_notes, release_date,
       required, minimum_version, metadata, created_at,
       version_major, version_minor, version_patch, version_pre_release
FROM releases
WHERE application_id = $1 AND platform = $2 AND architecture = $3
  AND version_pre_release IS NULL
ORDER BY version_major DESC, version_minor DESC, version_patch DESC
LIMIT 1;

-- name: GetApplicationStats :one
SELECT
    COUNT(*) AS total_releases,
    COUNT(*) FILTER (WHERE r1.required) AS required_releases,
    COUNT(DISTINCT r1.platform) AS platform_count,
    MAX(r1.release_date) AS latest_release_date,
    (
        SELECT r2.version FROM releases r2
        WHERE r2.application_id = $1
        ORDER BY r2.version_major DESC, r2.version_minor DESC, r2.version_patch DESC,
                 (r2.version_pre_release IS NULL) DESC,
                 r2.version_pre_release ASC
        LIMIT 1
    ) AS latest_version
FROM releases r1 WHERE r1.application_id = $1;