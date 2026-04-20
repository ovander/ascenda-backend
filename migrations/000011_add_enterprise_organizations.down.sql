ALTER TABLE tenants
    DROP COLUMN IF EXISTS organization_id,
    DROP COLUMN IF EXISTS type,
    DROP COLUMN IF EXISTS plan;
DROP INDEX IF EXISTS idx_tenants_organization_id;
DROP TABLE IF EXISTS organizations;
