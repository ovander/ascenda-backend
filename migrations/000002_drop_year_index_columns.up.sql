-- Migration: drop orphaned year_index columns
-- Background: GORM originally mapped YearIndex → year_index, but the DB schema
-- uses "year" as the column name. After adding gorm:"column:year" to the model,
-- AutoMigrate left the old year_index columns in place (NOT NULL, no default),
-- causing every INSERT to fail. This migration removes the orphaned columns.

ALTER TABLE product_assumptions        DROP COLUMN IF EXISTS year_index;
ALTER TABLE product_sales_volumes      DROP COLUMN IF EXISTS year_index;
ALTER TABLE product_distributor_margins DROP COLUMN IF EXISTS year_index;
