-- Migration: drop orphaned year_index columns (SAFE VERSION)

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'product_assumptions'
    ) THEN
ALTER TABLE product_assumptions DROP COLUMN IF EXISTS year_index;
END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'product_sales_volumes'
    ) THEN
ALTER TABLE product_sales_volumes DROP COLUMN IF EXISTS year_index;
END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'product_distributor_margins'
    ) THEN
ALTER TABLE product_distributor_margins DROP COLUMN IF EXISTS year_index;
END IF;
END $$;