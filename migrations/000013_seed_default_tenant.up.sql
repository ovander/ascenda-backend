-- Seed the default workspace tenant used when no tenant_id is present in the JWT.
-- The middleware falls back to this UUID: 00000000-0000-0000-0000-000000000001
-- ON CONFLICT ensures this is safe to run on existing databases.
INSERT INTO tenants (id, name, slug, type, plan, is_active, ai_credits)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'Default Workspace',
    'default',
    'workspace',
    'pro',
    true,
    0
) ON CONFLICT (id) DO NOTHING;
