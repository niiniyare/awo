-- platform_tenant: root isolation boundary for every tenant-scoped record.
-- This table has NO RLS — it is a global table read by all service roles.
CREATE TABLE IF NOT EXISTS platform_tenant (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name            varchar(255) NOT NULL,
    slug            varchar(63)  NOT NULL,
    status          varchar(20)  NOT NULL DEFAULT 'PENDING'
                        CHECK (status IN ('PENDING','ACTIVE','SUSPENDED','ARCHIVED')),
    plan            varchar(20)  NOT NULL DEFAULT 'free'
                        CHECK (plan IN ('free','starter','growth','enterprise')),
    country         varchar(2),
    locale          varchar(10)  NOT NULL DEFAULT 'en-KE',
    timezone        varchar(64)  NOT NULL DEFAULT 'Africa/Nairobi',
    currency        varchar(3)   NOT NULL DEFAULT 'KES',
    company_size    varchar(20)
                        CHECK (company_size IN ('MICRO','SMALL','MEDIUM','LARGE')),
    contact_email   varchar(254) NOT NULL,
    contact_phone   varchar(30),
    trial_ends_at   timestamptz,
    suspended_at    timestamptz,
    suspension_reason text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT platform_tenant_slug_unique UNIQUE (slug)
);

CREATE INDEX IF NOT EXISTS platform_tenant_status_idx ON platform_tenant (status);
CREATE INDEX IF NOT EXISTS platform_tenant_slug_idx   ON platform_tenant (slug);
