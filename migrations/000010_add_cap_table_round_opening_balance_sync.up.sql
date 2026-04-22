-- Migration: add opening-balance sync fields to cap_table_rounds (SAFE VERSION)

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'cap_table_rounds'
    ) THEN
ALTER TABLE cap_table_rounds
    ADD COLUMN IF NOT EXISTS opening_balance_synced BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS opening_balance_synced_amount_k NUMERIC(15, 2);
END IF;
END $$;