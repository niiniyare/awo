-- Migration: create platform_tenant, platform_org_unit, platform_branch
-- These tables are in the global schema (not per-tenant) because they are
-- accessible before set_tenant_context() is called. No RLS on platform_tenant.

CREATE TABLE IF NOT EXISTS platform_tenant (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name         varchar(255) NOT NULL,
    slug         varchar(63)  NOT NULL UNIQUE,
    status       varchar(20)  NOT NULL DEFAULT 'PENDING'
                              CHECK (status IN ('PENDING','ACTIVE','SUSPENDED','ARCHIVED')),
    plan         varchar(30)  NOT NULL DEFAULT 'free'
                              CHECK (plan IN ('free','starter','growth','enterprise')),
    country      varchar(2),
    locale       varchar(10)  NOT NULL DEFAULT 'en-KE',
    timezone     varchar(64)  NOT NULL DEFAULT 'Africa/Nairobi',
    currency     varchar(3)   NOT NULL DEFAULT 'KES',
    company_size varchar(10)  CHECK (company_size IN ('MICRO','SMALL','MEDIUM','LARGE')),
    contact_email varchar(254) NOT NULL,
    contact_phone varchar(30),
    trial_ends_at    timestamptz,
    suspended_at     timestamptz,
    suspension_reason text,
    created_at   timestamptz  NOT NULL DEFAULT NOW(),
    updated_at   timestamptz  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_platform_tenant_status ON platform_tenant (status);
CREATE INDEX IF NOT EXISTS idx_platform_tenant_slug   ON platform_tenant (slug);

-- platform_org_unit: tenant-scoped organisational hierarchy.
CREATE TABLE IF NOT EXISTS platform_org_unit (
    id          uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid         NOT NULL REFERENCES platform_tenant (id),
    name        varchar(255) NOT NULL,
    code        varchar(50)  NOT NULL UNIQUE,
    parent_id   uuid         REFERENCES platform_org_unit (id),
    type        varchar(20)  CHECK (type IN ('company','division','department','branch','team')),
    active      boolean      NOT NULL DEFAULT TRUE,
    created_at  timestamptz  NOT NULL DEFAULT NOW(),
    updated_at  timestamptz  NOT NULL DEFAULT NOW()
);

ALTER TABLE platform_org_unit ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_org_unit FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON platform_org_unit
    USING (tenant_id = current_tenant_id());

CREATE INDEX IF NOT EXISTS idx_platform_org_unit_tenant ON platform_org_unit (tenant_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_platform_org_unit_name_trgm
    ON platform_org_unit USING gin (name gin_trgm_ops);

-- platform_branch: physical locations.
CREATE TABLE IF NOT EXISTS platform_branch (
    id          uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid         NOT NULL REFERENCES platform_tenant (id),
    org_unit_id uuid         REFERENCES platform_org_unit (id),
    name        varchar(255) NOT NULL,
    code        varchar(50)  NOT NULL UNIQUE,
    address     varchar(1024),
    phone       varchar(30),
    active      boolean      NOT NULL DEFAULT TRUE,
    created_at  timestamptz  NOT NULL DEFAULT NOW(),
    updated_at  timestamptz  NOT NULL DEFAULT NOW()
);

ALTER TABLE platform_branch ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_branch FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON platform_branch
    USING (tenant_id = current_tenant_id());

CREATE INDEX IF NOT EXISTS idx_platform_branch_tenant ON platform_branch (tenant_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_platform_branch_name_trgm
    ON platform_branch USING gin (name gin_trgm_ops);
