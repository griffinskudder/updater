-- Fix updated_at triggers to produce RFC3339 timestamps.
-- The original triggers used datetime('now') which produces SQLite's native
-- "YYYY-MM-DD HH:MM:SS" format instead of RFC3339 "YYYY-MM-DDTHH:MM:SSZ".
-- Also backfill existing rows so that reads no longer encounter non-RFC3339 values.

DROP TRIGGER IF EXISTS update_applications_updated_at;
CREATE TRIGGER update_applications_updated_at
    AFTER UPDATE ON applications
    FOR EACH ROW
BEGIN
    UPDATE applications SET updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = NEW.id;
END;

DROP TRIGGER IF EXISTS update_api_keys_updated_at;
CREATE TRIGGER update_api_keys_updated_at
    AFTER UPDATE ON api_keys
    FOR EACH ROW
BEGIN
    UPDATE api_keys SET updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = NEW.id;
END;

-- Backfill rows written before this migration (datetime('now') format has no 'T').
UPDATE applications SET updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', updated_at) WHERE updated_at NOT LIKE '%T%';
UPDATE applications SET created_at = strftime('%Y-%m-%dT%H:%M:%SZ', created_at) WHERE created_at NOT LIKE '%T%';
UPDATE api_keys SET updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', updated_at) WHERE updated_at NOT LIKE '%T%';
UPDATE api_keys SET created_at = strftime('%Y-%m-%dT%H:%M:%SZ', created_at) WHERE created_at NOT LIKE '%T%';