-- 000017_align_late_tables_with_models
--
-- The tables created by hand-written migrations after the baseline
-- (000001 country_rate_configs, 000009 ai_narration_cache, 000011
-- organizations, 000014 wcr_entries, 000015 budget/cash monthly overrides)
-- drifted from their GORM models. Most of the drift is fixed on the model
-- side in the same change (column types, NOT NULL, defaults, the
-- organizations.slug unique constraint). Two points are fixed here instead:
--
-- 1. Index names. The migrations named the tenant/scenario indexes
--    idx_<table>_tenant / idx_<table>_scenario; the models' `index` tags
--    expect idx_<table>_tenant_id / idx_<table>_scenario_id, so GORM
--    AutoMigrate (development) created a second, duplicate index next to
--    each. The indexes are renamed; where a development database already has
--    the GORM-named duplicate, the migration-named one is dropped instead.
--
-- 2. created_at / updated_at on the three tables that embed
--    model.TenantScoped were NOT NULL, unlike the same columns on the other
--    48 TenantScoped tables. GORM fills them on every write
--    (autoCreateTime / autoUpdateTime), so the constraint is dropped for
--    consistency; the NOW() defaults stay.
--
-- Metadata-only operations: no table rewrite, no data change.

DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
        SELECT * FROM (VALUES
            ('wcr_entries',              'idx_wcr_entries_tenant',                'idx_wcr_entries_tenant_id'),
            ('wcr_entries',              'idx_wcr_entries_scenario',              'idx_wcr_entries_scenario_id'),
            ('budget_monthly_overrides', 'idx_budget_monthly_overrides_tenant',   'idx_budget_monthly_overrides_tenant_id'),
            ('budget_monthly_overrides', 'idx_budget_monthly_overrides_scenario', 'idx_budget_monthly_overrides_scenario_id'),
            ('cash_monthly_overrides',   'idx_cash_monthly_overrides_tenant',     'idx_cash_monthly_overrides_tenant_id'),
            ('cash_monthly_overrides',   'idx_cash_monthly_overrides_scenario',   'idx_cash_monthly_overrides_scenario_id')
        ) AS t(tbl, old_name, new_name)
    LOOP
        IF to_regclass('public.' || r.old_name) IS NOT NULL THEN
            IF to_regclass('public.' || r.new_name) IS NOT NULL THEN
                EXECUTE format('DROP INDEX %I', r.old_name);
                RAISE NOTICE '%: dropped % (duplicate of %)', r.tbl, r.old_name, r.new_name;
            ELSE
                EXECUTE format('ALTER INDEX %I RENAME TO %I', r.old_name, r.new_name);
            END IF;
        END IF;
    END LOOP;

    FOR r IN
        SELECT * FROM (VALUES
            ('wcr_entries'), ('budget_monthly_overrides'), ('cash_monthly_overrides')
        ) AS t(tbl)
    LOOP
        IF to_regclass('public.' || r.tbl) IS NOT NULL THEN
            EXECUTE format('ALTER TABLE %I ALTER COLUMN created_at DROP NOT NULL', r.tbl);
            EXECUTE format('ALTER TABLE %I ALTER COLUMN updated_at DROP NOT NULL', r.tbl);
        END IF;
    END LOOP;
END $$;
