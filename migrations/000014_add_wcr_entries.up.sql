-- Migration: create wcr_entries table for working capital requirement user inputs.
-- wcr_entries stores 5 adjustment lines × 5 years = up to 25 rows per scenario.
-- Previously only created by GORM AutoMigrate (dev-only); this makes it production-safe.

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'wcr_entries'
    ) THEN
        CREATE TABLE wcr_entries (
            id          UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
            tenant_id   UUID            NOT NULL,
            scenario_id UUID            NOT NULL,
            line_id     VARCHAR(100)    NOT NULL,
            year_index  INTEGER         NOT NULL,
            amount      NUMERIC(15,2)   NOT NULL DEFAULT 0,
            created_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
            updated_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW()
        );
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'wcr_entries'
    ) THEN
        CREATE INDEX IF NOT EXISTS idx_wcr_entries_tenant
            ON wcr_entries (tenant_id);

        CREATE INDEX IF NOT EXISTS idx_wcr_entries_scenario
            ON wcr_entries (scenario_id);
    END IF;
END $$;
