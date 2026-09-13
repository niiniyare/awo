-- Migration: create platform_tenant table
-- This table lives in the global schema (no per-tenant RLS) because it must
-- be accessible before any per-tenant context is established.

CREATE TABLE IF NOT EXISTS platform_tenant (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    slug            VARCHAR(63)  NOT NULL UNIQUE,
    status          VARCHAR(20)  NOT NULL DEFAULT 'PENDING'
                        CHECK (status IN ('PENDING','ACTIVE','SUSPENDED','ARCHIVED')),
    plan            VARCHAR(20)  NOT NULL DEFAULT 'free'
                        CHECK (plan IN ('free','starter','growth','enterprise')),
    country         VARCHAR(2),
    locale          VARCHAR(10)  NOT NULL DEFAULT 'en-KE',
    timezone        VARCHAR(64)  NOT NULL DEFAULT 'Africa/Nairobi',
    currency        VARCHAR(3)   NOT NULL DEFAULT 'KES',
    company_size    VARCHAR(10)
                        CHECK (company_size IN ('MICRO','SMALL','MEDIUM','LARGE')),
    contact_email   VARCHAR(254) NOT NULL,
    contact_phone   VARCHAR(30),
    trial_ends_at   TIMESTAMPTZ,
    suspended_at    TIMESTAMPTZ,
    suspension_reason VARCHAR(1024),
    custom_fields   JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS platform_tenant_slug_idx ON platform_tenant (slug);
CREATE INDEX IF NOT EXISTS platform_tenant_status_idx ON platform_tenant (status);
