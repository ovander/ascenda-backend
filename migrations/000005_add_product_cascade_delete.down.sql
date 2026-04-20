ALTER TABLE product_assumptions
    DROP CONSTRAINT IF EXISTS fk_product_assumptions_product;
ALTER TABLE product_assumptions
    ADD CONSTRAINT fk_product_assumptions_product
        FOREIGN KEY (product_id) REFERENCES products(id);

ALTER TABLE product_sales_volumes
    DROP CONSTRAINT IF EXISTS fk_product_sales_volumes_product;
ALTER TABLE product_sales_volumes
    ADD CONSTRAINT fk_product_sales_volumes_product
        FOREIGN KEY (product_id) REFERENCES products(id);

ALTER TABLE product_distributor_margins
    DROP CONSTRAINT IF EXISTS fk_product_distributor_margins_product;
ALTER TABLE product_distributor_margins
    ADD CONSTRAINT fk_product_distributor_margins_product
        FOREIGN KEY (product_id) REFERENCES products(id);
