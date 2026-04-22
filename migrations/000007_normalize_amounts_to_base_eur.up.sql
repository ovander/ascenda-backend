-- Migration: normalize all monetary amounts to base € (SAFE + IDEMPOTENT)

-- 1. Ensure flag table exists (IMPORTANT: outside DO)
CREATE TABLE IF NOT EXISTS migration_flags (
                                               name TEXT PRIMARY KEY,
                                               created_at TIMESTAMPTZ DEFAULT now()
    );

DO $$
BEGIN
    -- Prevent double execution
    IF EXISTS (
        SELECT 1 FROM migration_flags
        WHERE name = 'normalize_amounts_done'
    ) THEN
        RAISE NOTICE 'Normalization already applied — skipping';
        RETURN;
END IF;

    -- 1. Opex manual entries
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'opex_manual_entries') THEN
UPDATE opex_manual_entries SET amount = amount * 1000;
END IF;

    -- 2. Capex entries
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'capex_entries') THEN
UPDATE capex_entries SET amount = amount * 1000;
END IF;

    -- 3. Opex per hire
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'opex_per_hire') THEN
UPDATE opex_per_hire
SET
    property_rentals       = property_rentals       * 1000,
    postage_telecom        = postage_telecom        * 1000,
    supplies_purchases     = supplies_purchases     * 1000,
    studies_documentation  = studies_documentation  * 1000,
    travel_transportation  = travel_transportation  * 1000,
    mission_representation = mission_representation * 1000;
END IF;

    -- 4. Capex per hire
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'capex_per_hire') THEN
UPDATE capex_per_hire
SET
    furniture_per_hire = furniture_per_hire * 1000,
    it_equip_per_hire  = it_equip_per_hire  * 1000;
END IF;

    -- 5. Multi year adjustments
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'multi_year_adjustments') THEN
UPDATE multi_year_adjustments
SET
    previous_depreciation = previous_depreciation * 1000,
    potential_tax_credits = potential_tax_credits * 1000;
END IF;

    -- 6. Opening balances
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'opening_balances') THEN
UPDATE opening_balances
SET
    noncurrent_assets    = noncurrent_assets    * 1000,
    inventories          = inventories          * 1000,
    customer_receivables = customer_receivables * 1000,
    cash_and_securities  = cash_and_securities  * 1000,
    share_capital        = share_capital        * 1000,
    retained_earnings    = retained_earnings    * 1000,
    loans_and_debt       = loans_and_debt       * 1000,
    supplier_payables    = supplier_payables    * 1000,
    social_and_tax_debts = social_and_tax_debts * 1000;
END IF;

    -- 7. Plan config
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'plan_configs') THEN
UPDATE plan_configs
SET prior_year_turnover = prior_year_turnover * 1000
WHERE prior_year_turnover > 0;
END IF;

    -- Mark migration as done
INSERT INTO migration_flags(name)
VALUES ('normalize_amounts_done')
    ON CONFLICT DO NOTHING;

END $$;