-- Reverts 000017: restores the migration-era index names and the NOT NULL
-- timestamps. Rows written with a NULL timestamp in between (not possible
-- through GORM) would make SET NOT NULL fail; they are back-filled first.

DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
        SELECT * FROM (VALUES
            ('idx_wcr_entries_tenant_id',                'idx_wcr_entries_tenant'),
            ('idx_wcr_entries_scenario_id',              'idx_wcr_entries_scenario'),
            ('idx_budget_monthly_overrides_tenant_id',   'idx_budget_monthly_overrides_tenant'),
            ('idx_budget_monthly_overrides_scenario_id', 'idx_budget_monthly_overrides_scenario'),
            ('idx_cash_monthly_overrides_tenant_id',     'idx_cash_monthly_overrides_tenant'),
            ('idx_cash_monthly_overrides_scenario_id',   'idx_cash_monthly_overrides_scenario')
        ) AS t(new_name, old_name)
    LOOP
        IF to_regclass('public.' || r.new_name) IS NOT NULL AND to_regclass('public.' || r.old_name) IS NULL THEN
            EXECUTE format('ALTER INDEX %I RENAME TO %I', r.new_name, r.old_name);
        END IF;
    END LOOP;

    FOR r IN
        SELECT * FROM (VALUES
            ('wcr_entries'), ('budget_monthly_overrides'), ('cash_monthly_overrides')
        ) AS t(tbl)
    LOOP
        IF to_regclass('public.' || r.tbl) IS NOT NULL THEN
            EXECUTE format('UPDATE %I SET created_at = NOW() WHERE created_at IS NULL', r.tbl);
            EXECUTE format('UPDATE %I SET updated_at = NOW() WHERE updated_at IS NULL', r.tbl);
            EXECUTE format('ALTER TABLE %I ALTER COLUMN created_at SET NOT NULL', r.tbl);
            EXECUTE format('ALTER TABLE %I ALTER COLUMN updated_at SET NOT NULL', r.tbl);
        END IF;
    END LOOP;
END $$;
