ALTER TABLE cap_table_rounds
    DROP COLUMN IF EXISTS opening_balance_synced,
    DROP COLUMN IF EXISTS opening_balance_synced_amount_k;
