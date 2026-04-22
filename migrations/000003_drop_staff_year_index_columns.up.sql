-- Migration: drop orphaned year_index columns from staff tables (SAFE VERSION)

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'staff_headcounts'
    ) THEN
ALTER TABLE staff_headcounts DROP COLUMN IF EXISTS year_index;
END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'staff_salaries'
    ) THEN
ALTER TABLE staff_salaries DROP COLUMN IF EXISTS year_index;
END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'staff_incentives'
    ) THEN
ALTER TABLE staff_incentives DROP COLUMN IF EXISTS year_index;
END IF;
END $$;