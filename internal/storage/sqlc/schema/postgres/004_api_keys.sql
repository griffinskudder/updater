-- API keys table (PostgreSQL version)
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

CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);

CREATE TRIGGER update_api_keys_updated_at
    BEFORE UPDATE ON api_keys
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
