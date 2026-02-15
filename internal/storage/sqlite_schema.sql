-- SQLite schema initialization for runtime use.
-- Uses IF NOT EXISTS to support idempotent initialization.

CREATE TABLE IF NOT EXISTS applications (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    platforms TEXT NOT NULL DEFAULT '[]',
    config TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS releases (
    id TEXT PRIMARY KEY,
    application_id TEXT NOT NULL,
    version TEXT NOT NULL,
    platform TEXT NOT NULL,
    architecture TEXT NOT NULL,
    download_url TEXT NOT NULL,
    checksum TEXT NOT NULL,
    checksum_type TEXT NOT NULL DEFAULT 'sha256',
    file_size INTEGER NOT NULL,
    release_notes TEXT,
    release_date TEXT NOT NULL,
    required BOOLEAN NOT NULL DEFAULT 0,
    minimum_version TEXT,
    metadata TEXT DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),

    FOREIGN KEY (application_id) REFERENCES applications(id) ON DELETE CASCADE,
    UNIQUE(application_id, version, platform, architecture)
);

CREATE INDEX IF NOT EXISTS idx_applications_name ON applications(name);
CREATE INDEX IF NOT EXISTS idx_releases_app_platform_arch ON releases(application_id, platform, architecture);
CREATE INDEX IF NOT EXISTS idx_releases_version ON releases(version);
CREATE INDEX IF NOT EXISTS idx_releases_date ON releases(release_date DESC);
CREATE INDEX IF NOT EXISTS idx_releases_required ON releases(required);

-- Note: CREATE TRIGGER IF NOT EXISTS is not supported in SQLite.
-- The trigger is created only if it doesn't already exist using a workaround.
