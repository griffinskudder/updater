-- Add version sort columns for SQL-level semver ordering

ALTER TABLE releases ADD COLUMN version_major      INTEGER NOT NULL DEFAULT 0;
ALTER TABLE releases ADD COLUMN version_minor      INTEGER NOT NULL DEFAULT 0;
ALTER TABLE releases ADD COLUMN version_patch      INTEGER NOT NULL DEFAULT 0;
ALTER TABLE releases ADD COLUMN version_pre_release TEXT;

-- Backfill major (before first dot)
UPDATE releases
SET version_major = CAST(
    substr(version, 1, instr(version, '.') - 1)
AS INTEGER)
WHERE instr(version, '.') > 0
  AND instr(substr(version, instr(version, '.') + 1), '.') > 0;

-- Backfill minor (between first and second dot)
UPDATE releases
SET version_minor = CAST(
    substr(
        substr(version, instr(version, '.') + 1),
        1,
        instr(substr(version, instr(version, '.') + 1), '.') - 1
    )
AS INTEGER)
WHERE instr(version, '.') > 0
  AND instr(substr(version, instr(version, '.') + 1), '.') > 0;

-- Backfill patch (after second dot, before - or +)
UPDATE releases
SET version_patch = CAST(
    CASE
        WHEN instr(
            substr(version, instr(version, '.') + instr(substr(version, instr(version, '.') + 1), '.') + 2),
            '-'
        ) > 0
        THEN substr(
            substr(version, instr(version, '.') + instr(substr(version, instr(version, '.') + 1), '.') + 2),
            1,
            instr(
                substr(version, instr(version, '.') + instr(substr(version, instr(version, '.') + 1), '.') + 2),
                '-'
            ) - 1
        )
        WHEN instr(
            substr(version, instr(version, '.') + instr(substr(version, instr(version, '.') + 1), '.') + 2),
            '+'
        ) > 0
        THEN substr(
            substr(version, instr(version, '.') + instr(substr(version, instr(version, '.') + 1), '.') + 2),
            1,
            instr(
                substr(version, instr(version, '.') + instr(substr(version, instr(version, '.') + 1), '.') + 2),
                '+'
            ) - 1
        )
        ELSE substr(version, instr(version, '.') + instr(substr(version, instr(version, '.') + 1), '.') + 2)
    END
AS INTEGER)
WHERE instr(version, '.') > 0
  AND instr(substr(version, instr(version, '.') + 1), '.') > 0;

-- Backfill pre-release (segment after '-' and before '+', if present)
UPDATE releases
SET version_pre_release = CASE
    WHEN instr(
        substr(version, instr(version, '.') + instr(substr(version, instr(version, '.') + 1), '.') + 2),
        '-'
    ) > 0
    THEN CASE
        WHEN instr(
            substr(
                substr(version, instr(version, '.') + instr(substr(version, instr(version, '.') + 1), '.') + 2),
                instr(
                    substr(version, instr(version, '.') + instr(substr(version, instr(version, '.') + 1), '.') + 2),
                    '-'
                ) + 1
            ),
            '+'
        ) > 0
        THEN substr(
            substr(
                substr(version, instr(version, '.') + instr(substr(version, instr(version, '.') + 1), '.') + 2),
                instr(
                    substr(version, instr(version, '.') + instr(substr(version, instr(version, '.') + 1), '.') + 2),
                    '-'
                ) + 1
            ),
            1,
            instr(
                substr(
                    substr(version, instr(version, '.') + instr(substr(version, instr(version, '.') + 1), '.') + 2),
                    instr(
                        substr(version, instr(version, '.') + instr(substr(version, instr(version, '.') + 1), '.') + 2),
                        '-'
                    ) + 1
                ),
                '+'
            ) - 1
        )
        ELSE substr(
            substr(version, instr(version, '.') + instr(substr(version, instr(version, '.') + 1), '.') + 2),
            instr(
                substr(version, instr(version, '.') + instr(substr(version, instr(version, '.') + 1), '.') + 2),
                '-'
            ) + 1
        )
    END
    ELSE NULL
END
WHERE instr(version, '.') > 0
  AND instr(substr(version, instr(version, '.') + 1), '.') > 0;

CREATE INDEX IF NOT EXISTS idx_releases_version_sort
    ON releases(application_id, version_major DESC, version_minor DESC, version_patch DESC);