-- +goose Up

-- Applications table (SQLite version)
CREATE TABLE applications (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    platforms TEXT NOT NULL DEFAULT '[]',
    config TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
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
    metadata TEXT DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    version_major INTEGER NOT NULL DEFAULT 0,
    version_minor INTEGER NOT NULL DEFAULT 0,
    version_patch INTEGER NOT NULL DEFAULT 0,
    version_pre_release TEXT,

    FOREIGN KEY (application_id) REFERENCES applications(id) ON DELETE CASCADE,
    UNIQUE(application_id, version, platform, architecture)
);

-- API keys table (SQLite version)
CREATE TABLE api_keys (
    id          TEXT NOT NULL PRIMARY KEY,
    name        TEXT NOT NULL,
    key_hash    TEXT NOT NULL UNIQUE,
    prefix      TEXT NOT NULL,
    permissions TEXT NOT NULL DEFAULT '[]',
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Application indexes
CREATE INDEX idx_applications_name ON applications(name);

-- Release indexes
CREATE INDEX idx_releases_app_platform_arch ON releases(application_id, platform, architecture);
CREATE INDEX idx_releases_version ON releases(version);
CREATE INDEX idx_releases_date ON releases(release_date DESC);
CREATE INDEX idx_releases_required ON releases(required);
CREATE INDEX idx_releases_required_date ON releases(required, release_date DESC);
CREATE INDEX idx_releases_app_version ON releases(application_id, version);
CREATE INDEX idx_releases_version_sort ON releases(application_id, version_major DESC, version_minor DESC, version_patch DESC);

-- SQLite-specific indexes
CREATE INDEX idx_applications_config ON applications(config) WHERE config != '{}';

-- API key indexes
CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);

-- Triggers (RFC3339 format)
-- +goose StatementBegin
CREATE TRIGGER update_applications_updated_at
    AFTER UPDATE ON applications
    FOR EACH ROW
BEGIN
    UPDATE applications SET updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER update_api_keys_updated_at
    AFTER UPDATE ON api_keys
    FOR EACH ROW
BEGIN
    UPDATE api_keys SET updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS update_api_keys_updated_at;
DROP TRIGGER IF EXISTS update_applications_updated_at;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS releases;
DROP TABLE IF EXISTS applications;
