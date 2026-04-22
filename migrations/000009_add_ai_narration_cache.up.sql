-- Migration: persistent AI narration cache (SAFE VERSION)

DO $$
BEGIN
    -- Create table
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'ai_narration_cache'
    ) THEN
CREATE TABLE ai_narration_cache (
                                    cache_key   VARCHAR(120)  NOT NULL,
                                    tenant_id   UUID          NOT NULL,
                                    output      JSONB         NOT NULL,
                                    hit_count   INTEGER       NOT NULL DEFAULT 0,
                                    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
                                    last_hit_at TIMESTAMPTZ,

                                    PRIMARY KEY (tenant_id, cache_key)
);
END IF;

    -- Create index safely
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'ai_narration_cache'
    ) THEN
CREATE INDEX IF NOT EXISTS idx_ai_narration_cache_tenant
    ON ai_narration_cache (tenant_id);
END IF;

END $$;