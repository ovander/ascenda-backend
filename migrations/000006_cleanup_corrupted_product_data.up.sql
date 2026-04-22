-- Migration: remove corrupted product data (SAFE VERSION)

DO $$
BEGIN
    -- product_assumptions
    IF EXISTS (
        SELECT 1 FROM information_schema.tables WHERE table_name = 'product_assumptions'
    ) THEN
DELETE FROM product_assumptions WHERE year = 0;
END IF;

    -- product_sales_volumes
    IF EXISTS (
        SELECT 1 FROM information_schema.tables WHERE table_name = 'product_sales_volumes'
    ) THEN
DELETE FROM product_sales_volumes WHERE year = 0;
END IF;

    -- product_distributor_margins
    IF EXISTS (
        SELECT 1 FROM information_schema.tables WHERE table_name = 'product_distributor_margins'
    ) THEN
DELETE FROM product_distributor_margins WHERE year = 0;
END IF;
END $$;