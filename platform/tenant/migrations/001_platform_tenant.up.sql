-- Tenant 001: platform_tenant table.
--
-- platform_tenant is a PLATFORM-LEVEL table — it stores one row per registered
-- tenant and is owned by the platform layer, not by any tenant. It is NOT
-- subject to RLS by tenant_id. Instead, access is controlled by the
-- RequireAuth + PolicyEvaluator middleware: only platform-admins can Create,
-- Update, or Delete tenants.
--
-- Lifecycle: PENDING → ACTIVE → SUSPENDED → ARCHIVED
--            PENDING may also go directly to ARCHIVED (rejected signup).
-- Tenants are NEVER hard-deleted — they are archived to preserve audit trail.

CREATE TABLE IF NOT EXISTS platform_tenant (
    -- Standard identity columns.
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at  timestamptz NOT NULL DEFAULT NOW(),
    updated_at  timestamptz NOT NULL DEFAULT NOW(),

    -- Core fields (from TenantDefinition).
    name               varchar(255) NOT NULL,
    slug               varchar(63)  NOT NULL,
    status             varchar(20)  NOT NULL DEFAULT 'PENDING'
                           CHECK (status IN ('PENDING', 'ACTIVE', 'SUSPENDED', 'ARCHIVED')),
    plan               varchar(20)  NOT NULL DEFAULT 'free'
                           CHECK (plan IN ('free', 'starter', 'growth', 'enterprise')),

    -- Locale / regional.
    country            varchar(2),
    locale             varchar(10)  NOT NULL DEFAULT 'en-KE',
    timezone           varchar(64)  NOT NULL DEFAULT 'Africa/Nairobi',
    currency           varchar(3)   NOT NULL DEFAULT 'KES',

    -- Contact.
    contact_email      varchar(254) NOT NULL,
    contact_phone      varchar(30),

    -- Billing / lifecycle timestamps.
    trial_ends_at      timestamptz,
    suspended_at       timestamptz,
    suspension_reason  varchar(1024),

    -- Constraints.
    CONSTRAINT uq_platform_tenant_slug UNIQUE (slug)
);

COMMENT ON TABLE platform_tenant IS
    'One row per registered tenant. Platform-level table — not RLS-scoped. '
    'Tenants are never hard-deleted; set status=ARCHIVED to decommission.';

COMMENT ON COLUMN platform_tenant.slug IS
    'URL-safe identifier, immutable after creation. Embedded in workflow IDs, '
    'Redis keys, and migration history — never rename after first use.';

COMMENT ON COLUMN platform_tenant.status IS
    'Lifecycle state. Transitions: PENDING→ACTIVE, PENDING→ARCHIVED, '
    'ACTIVE→SUSPENDED, SUSPENDED→ACTIVE, ACTIVE→ARCHIVED, SUSPENDED→ARCHIVED.';

-- Auto-maintain updated_at.
CREATE TRIGGER trg_set_updated_at
    BEFORE UPDATE ON platform_tenant
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Performance indexes.
CREATE INDEX IF NOT EXISTS idx_platform_tenant_status ON platform_tenant (status);
CREATE INDEX IF NOT EXISTS idx_platform_tenant_plan   ON platform_tenant (plan);
