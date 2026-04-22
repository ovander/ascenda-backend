-- Migration: add ON DELETE CASCADE to product child tables (SAFE VERSION)

DO $$
BEGIN
    -- product_assumptions
    IF EXISTS (
        SELECT 1 FROM information_schema.tables WHERE table_name = 'product_assumptions'
    ) AND EXISTS (
        SELECT 1 FROM information_schema.tables WHERE table_name = 'products'
    ) THEN
ALTER TABLE product_assumptions
DROP CONSTRAINT IF EXISTS fk_product_assumptions_product;

ALTER TABLE product_assumptions
    ADD CONSTRAINT fk_product_assumptions_product
        FOREIGN KEY (product_id)
            REFERENCES products(id)
            ON DELETE CASCADE;
END IF;

    -- product_sales_volumes
    IF EXISTS (
        SELECT 1 FROM information_schema.tables WHERE table_name = 'product_sales_volumes'
    ) AND EXISTS (
        SELECT 1 FROM information_schema.tables WHERE table_name = 'products'
    ) THEN
ALTER TABLE product_sales_volumes
DROP CONSTRAINT IF EXISTS fk_product_sales_volumes_product;

ALTER TABLE product_sales_volumes
    ADD CONSTRAINT fk_product_sales_volumes_product
        FOREIGN KEY (product_id)
            REFERENCES products(id)
            ON DELETE CASCADE;
END IF;

    -- product_distributor_margins
    IF EXISTS (
        SELECT 1 FROM information_schema.tables WHERE table_name = 'product_distributor_margins'
    ) AND EXISTS (
        SELECT 1 FROM information_schema.tables WHERE table_name = 'products'
    ) THEN
ALTER TABLE product_distributor_margins
DROP CONSTRAINT IF EXISTS fk_product_distributor_margins_product;

ALTER TABLE product_distributor_margins
    ADD CONSTRAINT fk_product_distributor_margins_product
        FOREIGN KEY (product_id)
            REFERENCES products(id)
            ON DELETE CASCADE;
END IF;
END $$;