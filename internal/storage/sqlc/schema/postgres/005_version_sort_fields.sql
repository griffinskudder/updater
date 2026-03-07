-- Add version sort columns for SQL-level semver ordering

ALTER TABLE releases ADD COLUMN version_major      BIGINT NOT NULL DEFAULT 0;
ALTER TABLE releases ADD COLUMN version_minor      BIGINT NOT NULL DEFAULT 0;
ALTER TABLE releases ADD COLUMN version_patch      BIGINT NOT NULL DEFAULT 0;
ALTER TABLE releases ADD COLUMN version_pre_release TEXT;

-- Backfill existing rows using postgres regex
UPDATE releases SET
    version_major       = (regexp_matches(version, '^(\d+)\.'))[1]::integer,
    version_minor       = (regexp_matches(version, '^\d+\.(\d+)\.'))[1]::integer,
    version_patch       = (regexp_matches(version, '^\d+\.\d+\.(\d+)'))[1]::integer,
    version_pre_release = NULLIF(
        coalesce((regexp_matches(version, '^\d+\.\d+\.\d+-([^+]+)'))[1], ''),
        ''
    )
WHERE version ~ '^\d+\.\d+\.\d+';

CREATE INDEX IF NOT EXISTS idx_releases_version_sort
    ON releases(application_id, version_major DESC, version_minor DESC, version_patch DESC);