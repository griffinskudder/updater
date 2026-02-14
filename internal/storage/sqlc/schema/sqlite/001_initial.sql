-- Applications table (SQLite version)
CREATE TABLE applications (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    platforms TEXT NOT NULL DEFAULT '[]', -- JSON array as TEXT in SQLite
    config TEXT NOT NULL DEFAULT '{}',    -- Application configuration as JSON TEXT
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Releases table (SQLite version)
CREATE TABLE releases (
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
    metadata TEXT DEFAULT '{}', -- JSON as TEXT in SQLite
    created_at TEXT NOT NULL DEFAULT (datetime('now')),

    FOREIGN KEY (application_id) REFERENCES applications(id) ON DELETE CASCADE,
    UNIQUE(application_id, version, platform, architecture)
);

-- Indexes for better query performance (SQLite)
CREATE INDEX idx_applications_name ON applications(name);
CREATE INDEX idx_releases_app_platform_arch ON releases(application_id, platform, architecture);
CREATE INDEX idx_releases_version ON releases(version);
CREATE INDEX idx_releases_date ON releases(release_date DESC);
CREATE INDEX idx_releases_required ON releases(required);

-- Trigger to automatically update updated_at for applications (SQLite)
CREATE TRIGGER update_applications_updated_at
    AFTER UPDATE ON applications
    FOR EACH ROW
BEGIN
    UPDATE applications SET updated_at = datetime('now') WHERE id = NEW.id;
END;