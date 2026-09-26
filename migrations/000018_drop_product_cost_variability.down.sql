-- Restores the columns (empty): their values carried no meaning.
ALTER TABLE products
    ADD COLUMN IF NOT EXISTS direct_cost_variability numeric(5,4),
    ADD COLUMN IF NOT EXISTS external_charge_variability numeric(5,4),
    ADD COLUMN IF NOT EXISTS tax_variability numeric(5,4),
    ADD COLUMN IF NOT EXISTS staff_variability numeric(5,4),
    ADD COLUMN IF NOT EXISTS depreciation_variability numeric(5,4);
