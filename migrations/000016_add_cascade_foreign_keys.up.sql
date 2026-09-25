-- Cascade foreign keys along the plan → scenario → data hierarchy.
--
-- Until now no foreign key linked a scenario to its plan or any data row to
-- its scenario. PlanRepo.Delete and ScenarioRepo.Delete removed only the
-- parent row, so every delete orphaned products, staff, entries, snapshots,
-- BEP and cap-table rows for good (docs/AUDIT-2026-09-25.md §3.1, High).
-- PurgeDemoPlans hand-deleted 24 tables to work around it.
--
-- Each block below first deletes rows whose parent no longer exists (the
-- orphans those earlier deletes left behind — unreachable through the API,
-- and exactly what ON DELETE CASCADE would have removed), reports the count,
-- then adds the constraint. Blocks are idempotent (constraint name check),
-- so re-running on a database that already has them is a no-op.
--
-- Not covered on purpose: tenant/user ownership (tenant deletion is not a
-- product operation) and audit_logs (history must outlive its subject).


DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_scenarios_plan_id') THEN
        DELETE FROM scenarios c
        WHERE c.plan_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM business_plans p WHERE p.id = c.plan_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_scenarios_plan_id: removed % orphaned scenarios row(s)', orphans;
        END IF;
        ALTER TABLE scenarios
            ADD CONSTRAINT fk_scenarios_plan_id
            FOREIGN KEY (plan_id) REFERENCES business_plans(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_plan_members_plan_id') THEN
        DELETE FROM plan_members c
        WHERE c.plan_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM business_plans p WHERE p.id = c.plan_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_plan_members_plan_id: removed % orphaned plan_members row(s)', orphans;
        END IF;
        ALTER TABLE plan_members
            ADD CONSTRAINT fk_plan_members_plan_id
            FOREIGN KEY (plan_id) REFERENCES business_plans(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_plan_shareholders_plan_id') THEN
        DELETE FROM plan_shareholders c
        WHERE c.plan_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM business_plans p WHERE p.id = c.plan_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_plan_shareholders_plan_id: removed % orphaned plan_shareholders row(s)', orphans;
        END IF;
        ALTER TABLE plan_shareholders
            ADD CONSTRAINT fk_plan_shareholders_plan_id
            FOREIGN KEY (plan_id) REFERENCES business_plans(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_bep_snapshots_scenario_id') THEN
        DELETE FROM bep_snapshots c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_bep_snapshots_scenario_id: removed % orphaned bep_snapshots row(s)', orphans;
        END IF;
        ALTER TABLE bep_snapshots
            ADD CONSTRAINT fk_bep_snapshots_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_budget_monthly_overrides_scenario_id') THEN
        DELETE FROM budget_monthly_overrides c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_budget_monthly_overrides_scenario_id: removed % orphaned budget_monthly_overrides row(s)', orphans;
        END IF;
        ALTER TABLE budget_monthly_overrides
            ADD CONSTRAINT fk_budget_monthly_overrides_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_cap_table_companies_scenario_id') THEN
        DELETE FROM cap_table_companies c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_cap_table_companies_scenario_id: removed % orphaned cap_table_companies row(s)', orphans;
        END IF;
        ALTER TABLE cap_table_companies
            ADD CONSTRAINT fk_cap_table_companies_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_cap_table_rounds_scenario_id') THEN
        DELETE FROM cap_table_rounds c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_cap_table_rounds_scenario_id: removed % orphaned cap_table_rounds row(s)', orphans;
        END IF;
        ALTER TABLE cap_table_rounds
            ADD CONSTRAINT fk_cap_table_rounds_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_cap_table_scenario_branches_scenario_id') THEN
        DELETE FROM cap_table_scenario_branches c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_cap_table_scenario_branches_scenario_id: removed % orphaned cap_table_scenario_branches row(s)', orphans;
        END IF;
        ALTER TABLE cap_table_scenario_branches
            ADD CONSTRAINT fk_cap_table_scenario_branches_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_cap_table_share_classes_scenario_id') THEN
        DELETE FROM cap_table_share_classes c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_cap_table_share_classes_scenario_id: removed % orphaned cap_table_share_classes row(s)', orphans;
        END IF;
        ALTER TABLE cap_table_share_classes
            ADD CONSTRAINT fk_cap_table_share_classes_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_cap_table_shareholders_scenario_id') THEN
        DELETE FROM cap_table_shareholders c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_cap_table_shareholders_scenario_id: removed % orphaned cap_table_shareholders row(s)', orphans;
        END IF;
        ALTER TABLE cap_table_shareholders
            ADD CONSTRAINT fk_cap_table_shareholders_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_capex_entries_scenario_id') THEN
        DELETE FROM capex_entries c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_capex_entries_scenario_id: removed % orphaned capex_entries row(s)', orphans;
        END IF;
        ALTER TABLE capex_entries
            ADD CONSTRAINT fk_capex_entries_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_capex_per_hire_scenario_id') THEN
        DELETE FROM capex_per_hire c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_capex_per_hire_scenario_id: removed % orphaned capex_per_hire row(s)', orphans;
        END IF;
        ALTER TABLE capex_per_hire
            ADD CONSTRAINT fk_capex_per_hire_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_cash_monthly_overrides_scenario_id') THEN
        DELETE FROM cash_monthly_overrides c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_cash_monthly_overrides_scenario_id: removed % orphaned cash_monthly_overrides row(s)', orphans;
        END IF;
        ALTER TABLE cash_monthly_overrides
            ADD CONSTRAINT fk_cash_monthly_overrides_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_fiplan_entries_scenario_id') THEN
        DELETE FROM fiplan_entries c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_fiplan_entries_scenario_id: removed % orphaned fiplan_entries row(s)', orphans;
        END IF;
        ALTER TABLE fiplan_entries
            ADD CONSTRAINT fk_fiplan_entries_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_multi_year_adjustments_scenario_id') THEN
        DELETE FROM multi_year_adjustments c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_multi_year_adjustments_scenario_id: removed % orphaned multi_year_adjustments row(s)', orphans;
        END IF;
        ALTER TABLE multi_year_adjustments
            ADD CONSTRAINT fk_multi_year_adjustments_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_opening_balances_scenario_id') THEN
        DELETE FROM opening_balances c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_opening_balances_scenario_id: removed % orphaned opening_balances row(s)', orphans;
        END IF;
        ALTER TABLE opening_balances
            ADD CONSTRAINT fk_opening_balances_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_opex_manual_entries_scenario_id') THEN
        DELETE FROM opex_manual_entries c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_opex_manual_entries_scenario_id: removed % orphaned opex_manual_entries row(s)', orphans;
        END IF;
        ALTER TABLE opex_manual_entries
            ADD CONSTRAINT fk_opex_manual_entries_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_opex_per_hire_scenario_id') THEN
        DELETE FROM opex_per_hire c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_opex_per_hire_scenario_id: removed % orphaned opex_per_hire row(s)', orphans;
        END IF;
        ALTER TABLE opex_per_hire
            ADD CONSTRAINT fk_opex_per_hire_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_plan_configs_scenario_id') THEN
        DELETE FROM plan_configs c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_plan_configs_scenario_id: removed % orphaned plan_configs row(s)', orphans;
        END IF;
        ALTER TABLE plan_configs
            ADD CONSTRAINT fk_plan_configs_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_plan_snapshots_scenario_id') THEN
        DELETE FROM plan_snapshots c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_plan_snapshots_scenario_id: removed % orphaned plan_snapshots row(s)', orphans;
        END IF;
        ALTER TABLE plan_snapshots
            ADD CONSTRAINT fk_plan_snapshots_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_pnl_cash_entries_scenario_id') THEN
        DELETE FROM pnl_cash_entries c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_pnl_cash_entries_scenario_id: removed % orphaned pnl_cash_entries row(s)', orphans;
        END IF;
        ALTER TABLE pnl_cash_entries
            ADD CONSTRAINT fk_pnl_cash_entries_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_pnl_manual_entries_scenario_id') THEN
        DELETE FROM pnl_manual_entries c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_pnl_manual_entries_scenario_id: removed % orphaned pnl_manual_entries row(s)', orphans;
        END IF;
        ALTER TABLE pnl_manual_entries
            ADD CONSTRAINT fk_pnl_manual_entries_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_products_scenario_id') THEN
        DELETE FROM products c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_products_scenario_id: removed % orphaned products row(s)', orphans;
        END IF;
        ALTER TABLE products
            ADD CONSTRAINT fk_products_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_reports_scenario_id') THEN
        DELETE FROM reports c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_reports_scenario_id: removed % orphaned reports row(s)', orphans;
        END IF;
        ALTER TABLE reports
            ADD CONSTRAINT fk_reports_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_staff_headcounts_scenario_id') THEN
        DELETE FROM staff_headcounts c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_staff_headcounts_scenario_id: removed % orphaned staff_headcounts row(s)', orphans;
        END IF;
        ALTER TABLE staff_headcounts
            ADD CONSTRAINT fk_staff_headcounts_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_staff_incentives_scenario_id') THEN
        DELETE FROM staff_incentives c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_staff_incentives_scenario_id: removed % orphaned staff_incentives row(s)', orphans;
        END IF;
        ALTER TABLE staff_incentives
            ADD CONSTRAINT fk_staff_incentives_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_staff_salaries_scenario_id') THEN
        DELETE FROM staff_salaries c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_staff_salaries_scenario_id: removed % orphaned staff_salaries row(s)', orphans;
        END IF;
        ALTER TABLE staff_salaries
            ADD CONSTRAINT fk_staff_salaries_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_stock_option_plans_scenario_id') THEN
        DELETE FROM stock_option_plans c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_stock_option_plans_scenario_id: removed % orphaned stock_option_plans row(s)', orphans;
        END IF;
        ALTER TABLE stock_option_plans
            ADD CONSTRAINT fk_stock_option_plans_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_valuation_scenarios_scenario_id') THEN
        DELETE FROM valuation_scenarios c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_valuation_scenarios_scenario_id: removed % orphaned valuation_scenarios row(s)', orphans;
        END IF;
        ALTER TABLE valuation_scenarios
            ADD CONSTRAINT fk_valuation_scenarios_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_wcr_entries_scenario_id') THEN
        DELETE FROM wcr_entries c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_wcr_entries_scenario_id: removed % orphaned wcr_entries row(s)', orphans;
        END IF;
        ALTER TABLE wcr_entries
            ADD CONSTRAINT fk_wcr_entries_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_working_capital_configs_scenario_id') THEN
        DELETE FROM working_capital_configs c
        WHERE c.scenario_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM scenarios p WHERE p.id = c.scenario_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_working_capital_configs_scenario_id: removed % orphaned working_capital_configs row(s)', orphans;
        END IF;
        ALTER TABLE working_capital_configs
            ADD CONSTRAINT fk_working_capital_configs_scenario_id
            FOREIGN KEY (scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_bep_optimisation_plans_snapshot_id') THEN
        DELETE FROM bep_optimisation_plans c
        WHERE c.snapshot_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM bep_snapshots p WHERE p.id = c.snapshot_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_bep_optimisation_plans_snapshot_id: removed % orphaned bep_optimisation_plans row(s)', orphans;
        END IF;
        ALTER TABLE bep_optimisation_plans
            ADD CONSTRAINT fk_bep_optimisation_plans_snapshot_id
            FOREIGN KEY (snapshot_id) REFERENCES bep_snapshots(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_bep_sensitivity_configs_snapshot_id') THEN
        DELETE FROM bep_sensitivity_configs c
        WHERE c.snapshot_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM bep_snapshots p WHERE p.id = c.snapshot_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_bep_sensitivity_configs_snapshot_id: removed % orphaned bep_sensitivity_configs row(s)', orphans;
        END IF;
        ALTER TABLE bep_sensitivity_configs
            ADD CONSTRAINT fk_bep_sensitivity_configs_snapshot_id
            FOREIGN KEY (snapshot_id) REFERENCES bep_snapshots(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_fixed_cost_lines_snapshot_id') THEN
        DELETE FROM fixed_cost_lines c
        WHERE c.snapshot_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM bep_snapshots p WHERE p.id = c.snapshot_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_fixed_cost_lines_snapshot_id: removed % orphaned fixed_cost_lines row(s)', orphans;
        END IF;
        ALTER TABLE fixed_cost_lines
            ADD CONSTRAINT fk_fixed_cost_lines_snapshot_id
            FOREIGN KEY (snapshot_id) REFERENCES bep_snapshots(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_variable_cost_lines_snapshot_id') THEN
        DELETE FROM variable_cost_lines c
        WHERE c.snapshot_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM bep_snapshots p WHERE p.id = c.snapshot_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_variable_cost_lines_snapshot_id: removed % orphaned variable_cost_lines row(s)', orphans;
        END IF;
        ALTER TABLE variable_cost_lines
            ADD CONSTRAINT fk_variable_cost_lines_snapshot_id
            FOREIGN KEY (snapshot_id) REFERENCES bep_snapshots(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_bep_fixed_cost_savings_plan_id') THEN
        DELETE FROM bep_fixed_cost_savings c
        WHERE c.plan_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM bep_optimisation_plans p WHERE p.id = c.plan_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_bep_fixed_cost_savings_plan_id: removed % orphaned bep_fixed_cost_savings row(s)', orphans;
        END IF;
        ALTER TABLE bep_fixed_cost_savings
            ADD CONSTRAINT fk_bep_fixed_cost_savings_plan_id
            FOREIGN KEY (plan_id) REFERENCES bep_optimisation_plans(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_bep_variable_cost_savings_plan_id') THEN
        DELETE FROM bep_variable_cost_savings c
        WHERE c.plan_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM bep_optimisation_plans p WHERE p.id = c.plan_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_bep_variable_cost_savings_plan_id: removed % orphaned bep_variable_cost_savings row(s)', orphans;
        END IF;
        ALTER TABLE bep_variable_cost_savings
            ADD CONSTRAINT fk_bep_variable_cost_savings_plan_id
            FOREIGN KEY (plan_id) REFERENCES bep_optimisation_plans(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_bep_pcg_review_items_plan_id') THEN
        DELETE FROM bep_pcg_review_items c
        WHERE c.plan_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM bep_optimisation_plans p WHERE p.id = c.plan_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_bep_pcg_review_items_plan_id: removed % orphaned bep_pcg_review_items row(s)', orphans;
        END IF;
        ALTER TABLE bep_pcg_review_items
            ADD CONSTRAINT fk_bep_pcg_review_items_plan_id
            FOREIGN KEY (plan_id) REFERENCES bep_optimisation_plans(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_bep_fixed_cost_savings_fixed_cost_line_id') THEN
        DELETE FROM bep_fixed_cost_savings c
        WHERE c.fixed_cost_line_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM fixed_cost_lines p WHERE p.id = c.fixed_cost_line_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_bep_fixed_cost_savings_fixed_cost_line_id: removed % orphaned bep_fixed_cost_savings row(s)', orphans;
        END IF;
        ALTER TABLE bep_fixed_cost_savings
            ADD CONSTRAINT fk_bep_fixed_cost_savings_fixed_cost_line_id
            FOREIGN KEY (fixed_cost_line_id) REFERENCES fixed_cost_lines(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_bep_variable_cost_savings_variable_cost_line_id') THEN
        DELETE FROM bep_variable_cost_savings c
        WHERE c.variable_cost_line_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM variable_cost_lines p WHERE p.id = c.variable_cost_line_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_bep_variable_cost_savings_variable_cost_line_id: removed % orphaned bep_variable_cost_savings row(s)', orphans;
        END IF;
        ALTER TABLE bep_variable_cost_savings
            ADD CONSTRAINT fk_bep_variable_cost_savings_variable_cost_line_id
            FOREIGN KEY (variable_cost_line_id) REFERENCES variable_cost_lines(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_option_grants_plan_id') THEN
        DELETE FROM option_grants c
        WHERE c.plan_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM stock_option_plans p WHERE p.id = c.plan_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_option_grants_plan_id: removed % orphaned option_grants row(s)', orphans;
        END IF;
        ALTER TABLE option_grants
            ADD CONSTRAINT fk_option_grants_plan_id
            FOREIGN KEY (plan_id) REFERENCES stock_option_plans(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_option_grants_shareholder_id') THEN
        DELETE FROM option_grants c
        WHERE c.shareholder_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM cap_table_shareholders p WHERE p.id = c.shareholder_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_option_grants_shareholder_id: removed % orphaned option_grants row(s)', orphans;
        END IF;
        ALTER TABLE option_grants
            ADD CONSTRAINT fk_option_grants_shareholder_id
            FOREIGN KEY (shareholder_id) REFERENCES cap_table_shareholders(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_option_grants_round_id') THEN
        DELETE FROM option_grants c
        WHERE c.round_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM cap_table_rounds p WHERE p.id = c.round_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_option_grants_round_id: removed % orphaned option_grants row(s)', orphans;
        END IF;
        ALTER TABLE option_grants
            ADD CONSTRAINT fk_option_grants_round_id
            FOREIGN KEY (round_id) REFERENCES cap_table_rounds(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_cap_table_positions_round_id') THEN
        DELETE FROM cap_table_positions c
        WHERE c.round_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM cap_table_rounds p WHERE p.id = c.round_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_cap_table_positions_round_id: removed % orphaned cap_table_positions row(s)', orphans;
        END IF;
        ALTER TABLE cap_table_positions
            ADD CONSTRAINT fk_cap_table_positions_round_id
            FOREIGN KEY (round_id) REFERENCES cap_table_rounds(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_cap_table_positions_shareholder_id') THEN
        DELETE FROM cap_table_positions c
        WHERE c.shareholder_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM cap_table_shareholders p WHERE p.id = c.shareholder_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_cap_table_positions_shareholder_id: removed % orphaned cap_table_positions row(s)', orphans;
        END IF;
        ALTER TABLE cap_table_positions
            ADD CONSTRAINT fk_cap_table_positions_shareholder_id
            FOREIGN KEY (shareholder_id) REFERENCES cap_table_shareholders(id) ON DELETE CASCADE;
    END IF;
END $$;

DO $$
DECLARE orphans BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_stock_option_plans_round_id') THEN
        DELETE FROM stock_option_plans c
        WHERE c.round_id IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM cap_table_rounds p WHERE p.id = c.round_id);
        GET DIAGNOSTICS orphans = ROW_COUNT;
        IF orphans > 0 THEN
            RAISE NOTICE 'fk_stock_option_plans_round_id: removed % orphaned stock_option_plans row(s)', orphans;
        END IF;
        ALTER TABLE stock_option_plans
            ADD CONSTRAINT fk_stock_option_plans_round_id
            FOREIGN KEY (round_id) REFERENCES cap_table_rounds(id) ON DELETE SET NULL;
    END IF;
END $$;
