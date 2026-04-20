-- Migration: remove corrupted product data
-- Background: a frontend bug was using 0-based yearIndex (0–4) when saving
-- to the API, while the backend expects 1-based (1–5). This left orphaned
-- rows with year=0 that the compute engine ignores (year - 1 = -1, out of
-- bounds), plus rows with year=1–4 whose slot may be ambiguous.
-- Safest fix: delete all product assumption/volume/margin rows where
-- year = 0 (definitely wrong), then let the user re-enter year 1 data.
-- If you want a full reset for a specific product, uncomment the second block.

-- Remove year=0 rows (definitely corrupted — valid range is 1–5)
DELETE FROM product_assumptions        WHERE year = 0;
DELETE FROM product_sales_volumes      WHERE year = 0;
DELETE FROM product_distributor_margins WHERE year = 0;

-- ── Optional: full reset for a specific product ────────────────────────────
-- Replace '<your-product-uuid>' with the actual product ID.
--
-- DELETE FROM product_assumptions        WHERE product_id = '<your-product-uuid>';
-- DELETE FROM product_sales_volumes      WHERE product_id = '<your-product-uuid>';
-- DELETE FROM product_distributor_margins WHERE product_id = '<your-product-uuid>';
-- ──────────────────────────────────────────────────────────────────────────
