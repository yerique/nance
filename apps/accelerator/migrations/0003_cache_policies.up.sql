-- +migrate Up
CREATE TABLE IF NOT EXISTS cache_policies (
    tenant_id TEXT PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    default_ttl_seconds INTEGER NOT NULL DEFAULT 60,
    collections JSONB NOT NULL DEFAULT '{}'::jsonb,
    cache_key_version INTEGER NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
