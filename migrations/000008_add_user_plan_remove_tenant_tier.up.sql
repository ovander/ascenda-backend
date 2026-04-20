-- Migration: add user commercial plan; remove legacy tenant-level tier/limits.
--
-- Commercial plans (freemium|pro|enterprise) are now a per-user concern stored
-- in the Ascenda users table.  The old tenant-level tier/max_users/max_plans
-- columns are no longer used.

-- Add plan column to users (default freemium for all existing users)
ALTER TABLE users ADD COLUMN IF NOT EXISTS plan VARCHAR(50) NOT NULL DEFAULT 'freemium';

-- Drop legacy tenant-level columns
ALTER TABLE tenants DROP COLUMN IF EXISTS tier;
ALTER TABLE tenants DROP COLUMN IF EXISTS max_users;
ALTER TABLE tenants DROP COLUMN IF EXISTS max_plans;
