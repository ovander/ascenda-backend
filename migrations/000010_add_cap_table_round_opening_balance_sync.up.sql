-- Add opening-balance sync fields to cap_table_rounds.
-- Founding-capital rounds (France: capital social deposited before registration)
-- are synced to opening_balance rather than to the FiPlan capital_increase line.

ALTER TABLE cap_table_rounds
    ADD COLUMN IF NOT EXISTS opening_balance_synced          BOOLEAN        NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS opening_balance_synced_amount_k NUMERIC(15, 2) NULL;
