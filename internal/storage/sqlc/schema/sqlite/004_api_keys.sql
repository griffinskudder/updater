-- API keys table (SQLite version)
CREATE TABLE api_keys (
    id          TEXT NOT NULL PRIMARY KEY,
    name        TEXT NOT NULL,
    key_hash    TEXT NOT NULL UNIQUE,
    prefix      TEXT NOT NULL,
    permissions TEXT NOT NULL DEFAULT '[]',
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);

-- Uses strftime to produce RFC3339-compatible timestamps; superseded by 006_fix_datetime_triggers.sql.
CREATE TRIGGER update_api_keys_updated_at
    AFTER UPDATE ON api_keys
    FOR EACH ROW
BEGIN
    UPDATE api_keys SET updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = NEW.id;
END;
