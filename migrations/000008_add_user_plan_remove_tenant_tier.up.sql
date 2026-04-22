-- Migration: add user commercial plan; remove tenant tier (SAFE VERSION)

DO $$
BEGIN
    -- Add plan column to users
    IF EXISTS (
        SELECT 1 FROM information_schema.tables WHERE table_name = 'users'
    ) THEN
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS plan VARCHAR(50) NOT NULL DEFAULT 'freemium';
END IF;

    -- Drop legacy tenant-level columns
    IF EXISTS (
        SELECT 1 FROM information_schema.tables WHERE table_name = 'tenants'
    ) THEN
ALTER TABLE tenants DROP COLUMN IF EXISTS tier;
ALTER TABLE tenants DROP COLUMN IF EXISTS max_users;
ALTER TABLE tenants DROP COLUMN IF EXISTS max_plans;
END IF;
END $$;