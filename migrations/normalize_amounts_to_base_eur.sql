-- Migration: normalize all monetary amounts to base €
-- Background: the backend previously stored opex/capex entries in k€ and
-- used per-capita settings values also in k€.  The compute layer now
-- expects all amounts in base € and the frontend applies the ÷1 000
-- (or ÷1 000 000) conversion for display only.
--
-- This migration multiplies all affected columns by 1 000.
--
-- ⚠️  Run once.  Running again will multiply by 1 000 a second time.
-- ─────────────────────────────────────────────────────────────────────────────

-- 1. Opex manual entries (stored in k€ → base €)
UPDATE opex_manual_entries
SET    amount = amount * 1000;

-- 2. Capex entries (stored in k€ → base €)
UPDATE capex_entries
SET    amount = amount * 1000;

-- 3. OpexPerHire per-capita annual amounts (k€/person/year → €/person/year)
--    Percentage fields (insurance_costs_pct_sales, royalty_payments_pct_sales,
--    recruit_training_pct_payroll) are left unchanged — they are dimensionless.
UPDATE opex_per_hire
SET
    property_rentals       = property_rentals       * 1000,
    postage_telecom        = postage_telecom        * 1000,
    supplies_purchases     = supplies_purchases     * 1000,
    studies_documentation  = studies_documentation  * 1000,
    travel_transportation  = travel_transportation  * 1000,
    mission_representation = mission_representation * 1000;

-- 4. CapexPerHire per-hire amounts (k€/hire → €/hire)
UPDATE capex_per_hire
SET
    furniture_per_hire = furniture_per_hire * 1000,
    it_equip_per_hire  = it_equip_per_hire  * 1000;

-- 5. MultiYearAdjustment amounts (k€ → €)
--    These rows are seeded as zero in existing demos; included for completeness.
UPDATE multi_year_adjustments
SET
    previous_depreciation = previous_depreciation * 1000,
    potential_tax_credits  = potential_tax_credits  * 1000;

-- 6. Opening balances (amounts entered by users — assumed k€ in legacy UI)
--    NB: If any scenario was already entered in base €, adjust accordingly.
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

-- 7. Plan config — prior-year turnover was entered in k€ on the old settings form
UPDATE plan_configs
SET    prior_year_turnover = prior_year_turnover * 1000
WHERE  prior_year_turnover > 0;

-- ─────────────────────────────────────────────────────────────────────────────
-- Fields intentionally NOT migrated (already in base € or dimensionless):
--   • product_assumptions.base_unit_price          (€/unit — already base €)
--   • product_assumptions.raw_material_cost etc.   (€/unit — already base €)
--   • staff_entries.monthly_salary                 (€/month — already base €)
--   • staff_incentives.specific_incentives         (€/year — already base €)
--   • plan_configs.*_rate (VAT, corporate tax …)   (dimensionless percentages)
--   • captable.*                                   (uses explicit K suffixes)
--   • fiplan_lines.amount                          (already base €)
-- ─────────────────────────────────────────────────────────────────────────────
