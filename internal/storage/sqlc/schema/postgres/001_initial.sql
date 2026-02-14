-- Applications table
CREATE TABLE applications (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    platforms JSONB NOT NULL DEFAULT '[]', -- JSON array of supported platforms
    config JSONB NOT NULL DEFAULT '{}',    -- Application configuration
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

    FOREIGN KEY (application_id) REFERENCES applications(id) ON DELETE CASCADE,
    UNIQUE(application_id, version, platform, architecture)
);

-- Indexes for better query performance
CREATE INDEX idx_applications_name ON applications(name);
CREATE INDEX idx_applications_platforms ON applications USING GIN(platforms);

CREATE INDEX idx_releases_app_platform_arch ON releases(application_id, platform, architecture);
CREATE INDEX idx_releases_version ON releases(version);
CREATE INDEX idx_releases_date ON releases(release_date DESC);
CREATE INDEX idx_releases_required ON releases(required);

-- Function to automatically update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger to automatically update updated_at for applications
CREATE TRIGGER update_applications_updated_at
    BEFORE UPDATE ON applications
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();