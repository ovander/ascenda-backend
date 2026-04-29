-- Migration: create budget_monthly_overrides and cash_monthly_overrides tables.
-- Both were previously only created by GORM AutoMigrate (dev-only), so they were
-- absent in production, causing budget and cash views to return errors.

DO $$
BEGIN
    -- ── budget_monthly_overrides ──────────────────────────────────────────────
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'budget_monthly_overrides'
    ) THEN
        CREATE TABLE budget_monthly_overrides (
            id          UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
            tenant_id   UUID            NOT NULL,
            scenario_id UUID            NOT NULL,
            line_id     VARCHAR(100)    NOT NULL,
            year_index  INTEGER         NOT NULL,
            month       INTEGER         NOT NULL,
            amount      NUMERIC(15,2)   NOT NULL DEFAULT 0,
            created_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
            updated_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW()
        );
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'budget_monthly_overrides'
    ) THEN
        CREATE INDEX IF NOT EXISTS idx_budget_monthly_overrides_tenant
            ON budget_monthly_overrides (tenant_id);
        CREATE INDEX IF NOT EXISTS idx_budget_monthly_overrides_scenario
            ON budget_monthly_overrides (scenario_id);
    END IF;

    -- ── cash_monthly_overrides ────────────────────────────────────────────────
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'cash_monthly_overrides'
    ) THEN
        CREATE TABLE cash_monthly_overrides (
            id          UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
            tenant_id   UUID            NOT NULL,
            scenario_id UUID            NOT NULL,
            line_id     VARCHAR(100)    NOT NULL,
            year_index  INTEGER         NOT NULL,
            month       INTEGER         NOT NULL,
            sub_index   INTEGER         NOT NULL DEFAULT 0,
            amount      NUMERIC(15,2)   NOT NULL DEFAULT 0,
            created_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
            updated_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW()
        );
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'cash_monthly_overrides'
    ) THEN
        CREATE INDEX IF NOT EXISTS idx_cash_monthly_overrides_tenant
            ON cash_monthly_overrides (tenant_id);
        CREATE INDEX IF NOT EXISTS idx_cash_monthly_overrides_scenario
            ON cash_monthly_overrides (scenario_id);
    END IF;
END $$;
