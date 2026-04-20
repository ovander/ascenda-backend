-- Migration: persistent AI narration cache.
--
-- The cache is content-addressed: the key is a SHA-256 hash of the full
-- NarrationContext. If any input value changes the hash changes and the old
-- row is never matched again — no explicit invalidation is needed.
--
-- tenant_id is stored so that rows can be scoped per-tenant and bulk-evicted
-- (e.g. when a tenant is deleted or their data is reset).
-- There is intentionally no TTL column: the content hash provides all the
-- invalidation we need. Old rows that are never matched simply accumulate and
-- can be cleaned up by a periodic VACUUM or a maintenance job if needed.

CREATE TABLE IF NOT EXISTS ai_narration_cache (
    cache_key   VARCHAR(120)  NOT NULL,
    tenant_id   UUID          NOT NULL,
    output      JSONB         NOT NULL,
    hit_count   INTEGER       NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    last_hit_at TIMESTAMPTZ,

    PRIMARY KEY (tenant_id, cache_key)
);

-- Index to allow efficient per-tenant lookups and bulk deletes.
CREATE INDEX IF NOT EXISTS idx_ai_narration_cache_tenant ON ai_narration_cache (tenant_id);
