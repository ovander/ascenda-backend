-- Add ON DELETE CASCADE foreign keys for product child tables so that a
-- DELETE on products automatically removes all dependent rows at the DB level.
-- This is idempotent: each block drops the old constraint (if it exists under
-- any name) and recreates it with CASCADE.

-- product_assumptions
ALTER TABLE product_assumptions
    DROP CONSTRAINT IF EXISTS fk_product_assumptions_product;
ALTER TABLE product_assumptions
    ADD CONSTRAINT fk_product_assumptions_product
        FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE;

-- product_sales_volumes
ALTER TABLE product_sales_volumes
    DROP CONSTRAINT IF EXISTS fk_product_sales_volumes_product;
ALTER TABLE product_sales_volumes
    ADD CONSTRAINT fk_product_sales_volumes_product
        FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE;

-- product_distributor_margins
ALTER TABLE product_distributor_margins
    DROP CONSTRAINT IF EXISTS fk_product_distributor_margins_product;
ALTER TABLE product_distributor_margins
    ADD CONSTRAINT fk_product_distributor_margins_product
        FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE;
