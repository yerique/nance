-- +migrate Up
-- Platform default cache TTL is 60 seconds for all collections (opt-in via _cache suffix).
ALTER TABLE cache_policies
    ALTER COLUMN default_ttl_seconds SET DEFAULT 60;

UPDATE cache_policies
SET default_ttl_seconds = 60,
    updated_at = NOW()
WHERE default_ttl_seconds = 2;
