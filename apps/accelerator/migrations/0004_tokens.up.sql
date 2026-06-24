-- +migrate Up
CREATE TABLE IF NOT EXISTS tokens (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_tokens_tenant ON tokens(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tokens_active ON tokens(tenant_id) WHERE revoked_at IS NULL AND (expires_at IS NULL OR expires_at > NOW());
