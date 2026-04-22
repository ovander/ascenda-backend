-- Migration: enterprise organization support (SAFE VERSION)

DO $$
BEGIN
    -- Ensure pgcrypto for UUID generation
    CREATE EXTENSION IF NOT EXISTS pgcrypto;

    -- ── 1. Organizations ─────────────────────────────────────────────

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'organizations'
    ) THEN
CREATE TABLE organizations (
                               id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
                               name          VARCHAR(255) NOT NULL,
                               slug          VARCHAR(100) NOT NULL UNIQUE,
                               plan          VARCHAR(50)  NOT NULL DEFAULT 'enterprise',
                               max_users     INT          NOT NULL DEFAULT 0,
                               billing_email VARCHAR(255),
                               domain        VARCHAR(255),
                               is_active     BOOL         NOT NULL DEFAULT true,
                               created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
                               updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);
END IF;

    -- ── 2. Tenants: organization FK ─────────────────────────────────

    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'tenants'
    ) THEN
ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS organization_id UUID;

-- Add FK safely
IF NOT EXISTS (
            SELECT 1 FROM pg_constraint
            WHERE conname = 'fk_tenants_organization'
        ) THEN
ALTER TABLE tenants
    ADD CONSTRAINT fk_tenants_organization
        FOREIGN KEY (organization_id)
            REFERENCES organizations(id)
            ON DELETE SET NULL;
END IF;

CREATE INDEX IF NOT EXISTS idx_tenants_organization_id
    ON tenants(organization_id);
END IF;

    -- ── 3. Tenants: type ─────────────────────────────────────────────

    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'tenants'
    ) THEN
ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS type VARCHAR(50) NOT NULL DEFAULT 'workspace';
END IF;

    -- ── 4. Tenants: plan ─────────────────────────────────────────────

    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'tenants'
    ) THEN
ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS plan VARCHAR(50) NOT NULL DEFAULT 'freemium';
END IF;

END $$;