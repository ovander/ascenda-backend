-- Migration: enterprise organization support
--
-- An Organization is the billing/administrative umbrella for enterprise clients.
-- One org contains N tenants (departments). Workspace tenants have no org.
--
-- Changes:
--   1. Create `organizations` table.
--   2. Add `organization_id` FK to `tenants` (nullable — workspace tenants have no org).
--   3. Add `type`  column to tenants: "workspace" (default) | "enterprise".
--   4. Add `plan`  column to tenants: effective commercial plan for tier gating.
--      For enterprise tenants this overrides individual user.plan values.
--      For workspace tenants this column is unused (user.plan governs).

-- ── 1. Organizations ─────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS organizations (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(255) NOT NULL,
    slug          VARCHAR(100) NOT NULL UNIQUE,
    plan          VARCHAR(50)  NOT NULL DEFAULT 'enterprise',
    max_users     INT          NOT NULL DEFAULT 0,  -- 0 = unlimited
    billing_email VARCHAR(255),
    domain        VARCHAR(255),                     -- future: auto-join by email domain
    is_active     BOOL         NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- ── 2. Tenants: organization FK ───────────────────────────────────────────────

ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_tenants_organization_id ON tenants(organization_id);

-- ── 3. Tenants: type (workspace | enterprise) ────────────────────────────────

ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS type VARCHAR(50) NOT NULL DEFAULT 'workspace';

-- ── 4. Tenants: effective plan for tier gating ───────────────────────────────
--
-- For enterprise tenants this is set to the org's plan at provisioning time
-- and updated when the org plan changes.  For workspace tenants it is unused.

ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS plan VARCHAR(50) NOT NULL DEFAULT 'freemium';
