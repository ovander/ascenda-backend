-- The five cost-variability columns on products were never read by any
-- computation (break-even treats COGS as variable, payroll and opex as
-- fixed), and their values were accidental: 1 on seeded demo products, 0 on
-- everything else (the create endpoint ignored them; a rename reset them).
-- The UI no longer shows them.
ALTER TABLE products
    DROP COLUMN IF EXISTS direct_cost_variability,
    DROP COLUMN IF EXISTS external_charge_variability,
    DROP COLUMN IF EXISTS tax_variability,
    DROP COLUMN IF EXISTS staff_variability,
    DROP COLUMN IF EXISTS depreciation_variability;
