-- +migrate Up
CREATE TABLE IF NOT EXISTS tenant_backends (
    tenant_id TEXT PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    uri_ciphertext BYTEA NOT NULL,
    nonce BYTEA NOT NULL,
    dek_version TEXT NOT NULL DEFAULT 'v1',
    last_validated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
