-- Migration: create IAM tables (iam_user, iam_role, iam_session)
-- All tables are tenant-scoped (RLS enforced).

-- Users
CREATE TABLE IF NOT EXISTS iam_user (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES platform_tenant(id),
    email               VARCHAR(254) NOT NULL,
    password_hash       VARCHAR(255) NOT NULL,
    first_name          VARCHAR(100),
    last_name           VARCHAR(100),
    status              VARCHAR(30) NOT NULL DEFAULT 'pending_verification'
                            CHECK (status IN ('pending_verification','active','suspended','deleted')),
    tenant_id_fk        UUID REFERENCES platform_tenant(id),
    email_verified_at   TIMESTAMPTZ,
    last_login_at       TIMESTAMPTZ,
    mfa_secret          VARCHAR(255),
    mfa_enabled         BOOLEAN NOT NULL DEFAULT FALSE,
    locale              VARCHAR(10) NOT NULL DEFAULT 'en-KE',
    timezone            VARCHAR(64) NOT NULL DEFAULT 'Africa/Nairobi',
    custom_fields       JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, email)
);

ALTER TABLE iam_user ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam_user FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON iam_user
    USING (tenant_id = current_tenant_id());

CREATE INDEX IF NOT EXISTS iam_user_email_idx ON iam_user (tenant_id, email);
CREATE INDEX IF NOT EXISTS iam_user_status_idx ON iam_user (tenant_id, status);

-- Roles
CREATE TABLE IF NOT EXISTS iam_role (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL REFERENCES platform_tenant(id),
    name          VARCHAR(100) NOT NULL,
    label         VARCHAR(255),
    description   VARCHAR(1024),
    is_system     BOOLEAN NOT NULL DEFAULT FALSE,
    custom_fields JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, name)
);

ALTER TABLE iam_role ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam_role FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON iam_role
    USING (tenant_id = current_tenant_id());

-- Sessions
CREATE TABLE IF NOT EXISTS iam_session (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL REFERENCES platform_tenant(id),
    user_id       UUID NOT NULL REFERENCES iam_user(id),
    token_hash    VARCHAR(128) NOT NULL UNIQUE,
    ip_address    VARCHAR(45),
    user_agent    VARCHAR(1024),
    expires_at    TIMESTAMPTZ NOT NULL,
    revoked       BOOLEAN NOT NULL DEFAULT FALSE,
    revoked_at    TIMESTAMPTZ,
    custom_fields JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE iam_session ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam_session FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON iam_session
    USING (tenant_id = current_tenant_id());

CREATE INDEX IF NOT EXISTS iam_session_token_idx ON iam_session (token_hash) WHERE NOT revoked;
CREATE INDEX IF NOT EXISTS iam_session_user_idx  ON iam_session (user_id, expires_at);
