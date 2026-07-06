-- +migrate Up
-- Cache TTL defaults and collection overrides are per connection (not org-wide).

CREATE TABLE IF NOT EXISTS connection_cache_policies (
    connection_id TEXT PRIMARY KEY REFERENCES connections(id) ON DELETE CASCADE,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    default_ttl_seconds INTEGER NOT NULL DEFAULT 60,
    collections JSONB NOT NULL DEFAULT '{}'::jsonb,
    cache_key_version INTEGER NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_connection_cache_policies_tenant ON connection_cache_policies(tenant_id);

-- Copy legacy tenant policies onto every connection for that tenant
INSERT INTO connection_cache_policies (connection_id, tenant_id, default_ttl_seconds, collections, cache_key_version, updated_at)
SELECT
    c.id,
    c.tenant_id,
    COALESCE(p.default_ttl_seconds, 60),
    COALESCE(p.collections, '{}'::jsonb),
    COALESCE(p.cache_key_version, 1),
    COALESCE(p.updated_at, NOW())
FROM connections c
LEFT JOIN cache_policies p ON p.tenant_id = c.tenant_id
ON CONFLICT (connection_id) DO NOTHING;
