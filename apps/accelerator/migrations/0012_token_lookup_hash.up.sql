-- +migrate Up
-- SHA-256 hex of raw proxy token for O(1) auth lookup (bcrypt remains for legacy tokens).
ALTER TABLE tokens ADD COLUMN IF NOT EXISTS lookup_hash TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_tokens_lookup_hash
    ON tokens (lookup_hash)
    WHERE lookup_hash IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_tokens_tenant_lookup
    ON tokens (tenant_id, lookup_hash)
    WHERE revoked_at IS NULL AND lookup_hash IS NOT NULL;
