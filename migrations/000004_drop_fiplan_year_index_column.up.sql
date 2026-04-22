-- Migration: drop orphaned year_index column from fiplan_entries (SAFE VERSION)

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_name = 'fiplan_entries'
    ) THEN
ALTER TABLE fiplan_entries DROP COLUMN IF EXISTS year_index;
END IF;
END $$;