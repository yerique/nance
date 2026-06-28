-- +migrate Up
-- Idempotent: ensure default is 60s (safe if 0007 already applied).
ALTER TABLE cache_policies
    ALTER COLUMN default_ttl_seconds SET DEFAULT 60;

UPDATE cache_policies
SET default_ttl_seconds = 60,
    updated_at = NOW()
WHERE default_ttl_seconds = 2;
