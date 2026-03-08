-- +goose Up

-- Applications table
CREATE TABLE applications (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    platforms JSONB NOT NULL DEFAULT '[]',
    config JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Releases table
CREATE TABLE releases (
    id TEXT PRIMARY KEY,
    application_id TEXT NOT NULL,
    version TEXT NOT NULL,
    platform TEXT NOT NULL,
    architecture TEXT NOT NULL,
    download_url TEXT NOT NULL,
    checksum TEXT NOT NULL,
    checksum_type TEXT NOT NULL DEFAULT 'sha256',
    file_size BIGINT NOT NULL,
    release_notes TEXT,
    release_date TIMESTAMPTZ NOT NULL,
    required BOOLEAN NOT NULL DEFAULT FALSE,
    minimum_version TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version_major BIGINT NOT NULL DEFAULT 0,
    version_minor BIGINT NOT NULL DEFAULT 0,
    version_patch BIGINT NOT NULL DEFAULT 0,
    version_pre_release TEXT,

    FOREIGN KEY (application_id) REFERENCES applications(id) ON DELETE RESTRICT,
    UNIQUE(application_id, version, platform, architecture)
);

-- API keys table
CREATE TABLE api_keys (
    id          TEXT        NOT NULL PRIMARY KEY,
    name        TEXT        NOT NULL,
    key_hash    TEXT        NOT NULL UNIQUE,
    prefix      TEXT        NOT NULL,
    permissions JSONB       NOT NULL DEFAULT '[]',
    enabled     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Application indexes
CREATE INDEX idx_applications_name ON applications(name);
CREATE INDEX idx_applications_platforms ON applications USING GIN(platforms);

-- Release indexes
CREATE INDEX idx_releases_app_platform_arch ON releases(application_id, platform, architecture);
CREATE INDEX idx_releases_version ON releases(version);
CREATE INDEX idx_releases_date ON releases(release_date DESC);
CREATE INDEX idx_releases_required ON releases(required);
CREATE INDEX idx_releases_required_date ON releases(required, release_date DESC);
CREATE INDEX idx_releases_metadata_gin ON releases USING GIN(metadata);
CREATE INDEX idx_releases_app_version ON releases(application_id, version);
CREATE INDEX idx_releases_version_sort ON releases(application_id, version_major DESC, version_minor DESC, version_patch DESC);

-- API key indexes
CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);

-- Function to automatically update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers
CREATE TRIGGER update_applications_updated_at
    BEFORE UPDATE ON applications
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_api_keys_updated_at
    BEFORE UPDATE ON api_keys
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- +goose Down
DROP TRIGGER IF EXISTS update_api_keys_updated_at ON api_keys;
DROP TRIGGER IF EXISTS update_applications_updated_at ON applications;
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS releases;
DROP TABLE IF EXISTS applications;
