-- iam_users: authenticated human principals, one per user per tenant.
-- Tenant-scoped: RLS enforces tenant isolation.
-- password_hash stores a bcrypt digest; the raw password is never persisted.

CREATE TABLE iam_users (
    id               UUID         NOT NULL DEFAULT gen_random_uuid(),
    tenant_id        UUID         NOT NULL,
    email            VARCHAR(254) NOT NULL,
    full_name        VARCHAR(255) NOT NULL,
    password_hash    VARCHAR(72)  NOT NULL,
    status           VARCHAR(16)  NOT NULL DEFAULT 'active'
                         CHECK (status IN ('active', 'inactive', 'locked')),
    email_verified   BOOLEAN      NOT NULL DEFAULT FALSE,
    last_login_at    TIMESTAMPTZ,
    custom_fields    JSONB        NOT NULL DEFAULT '{}',
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT iam_users_pkey PRIMARY KEY (id)
);

-- Unique email per tenant (a user belongs to exactly one tenant account).
CREATE UNIQUE INDEX iam_users_tenant_email_uidx
    ON iam_users (tenant_id, email);

-- Trigram index for full_name search (Searchable: true on EntityDefinition).
CREATE INDEX iam_users_full_name_trgm_idx
    ON iam_users USING GIN (full_name gin_trgm_ops);

-- GIN index for custom fields.
CREATE INDEX iam_users_custom_fields_idx
    ON iam_users USING GIN (custom_fields);

-- RLS: each tenant sees only its own users.
ALTER TABLE iam_users ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam_users FORCE ROW LEVEL SECURITY;

CREATE POLICY iam_users_tenant_isolation ON iam_users
    USING (tenant_id = current_tenant_id());
