-- Migration: Add additional performance indexes
-- This is an example of how future migrations would be structured

-- Add index for release lookups by required status and date
CREATE INDEX IF NOT EXISTS idx_releases_required_date ON releases(required, release_date DESC);

-- Add partial index for draft releases (PostgreSQL specific)
CREATE INDEX IF NOT EXISTS idx_releases_metadata_gin ON releases USING GIN(metadata);

-- Add composite index for version comparisons
CREATE INDEX IF NOT EXISTS idx_releases_app_version ON releases(application_id, version);