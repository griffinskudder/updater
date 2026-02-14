-- Migration: Add additional performance indexes (SQLite version)
-- This is an example of how future migrations would be structured

-- Add index for release lookups by required status and date
CREATE INDEX IF NOT EXISTS idx_releases_required_date ON releases(required, release_date DESC);

-- Add composite index for version comparisons
CREATE INDEX IF NOT EXISTS idx_releases_app_version ON releases(application_id, version);

-- SQLite-specific optimizations
CREATE INDEX IF NOT EXISTS idx_applications_config ON applications(config) WHERE config != '{}';