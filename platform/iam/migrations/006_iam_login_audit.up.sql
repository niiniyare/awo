-- IAM 006: iam_login_audit — tamper-evident append-only authentication event log.
--
-- IMMUTABLE: LoginAuditImmutableGuard hook blocks all UPDATE and DELETE attempts
-- at the application layer. No PostgreSQL-level update policy is needed because
-- the table has no UPDATE policy declared, but defense-in-depth is provided by
-- the FORCE ROW LEVEL SECURITY + the restrictive INSERT-only policy below.
--
-- This table is readable by tenant.admin and platform-admin only
-- (loginAuditTenantAdminPolicy). Ordinary users have no read access.
--
-- event values:
--   login           — successful human or service-account login
--   logout          — explicit logout
--   failed_login    — bad credentials; failure_reason populated
--   token_use       — API token authentication event
--   session_revoked — session explicitly revoked

CREATE TABLE IF NOT EXISTS iam_login_audit (
    id                  uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           uuid         NOT NULL REFERENCES platform_tenant(id),
    created_at          timestamptz  NOT NULL DEFAULT NOW(),
    -- No updated_at: append-only log.

    event               varchar(30)  NOT NULL
                            CHECK (event IN ('login','logout','failed_login','token_use','session_revoked')),

    -- One of these is set for authenticated events; both null for failed_login
    -- attempts where the user could not be identified.
    user_id             uuid         REFERENCES iam_user(id) ON DELETE SET NULL,
    service_account_id  uuid         REFERENCES iam_service_account(id) ON DELETE SET NULL,

    ip_address          varchar(45),
    device_id           varchar(255),
    -- Populated only for failed_login events.
    failure_reason      varchar(255)
);

COMMENT ON TABLE iam_login_audit IS
    'Tamper-evident append-only authentication event log. '
    'Immutable at application layer via LoginAuditImmutableGuard hook. '
    'Readable only by tenant.admin and platform-admin roles.';

COMMENT ON COLUMN iam_login_audit.event IS
    'Authentication event type. See definition for valid values.';

ALTER TABLE iam_login_audit ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam_login_audit FORCE ROW LEVEL SECURITY;

-- Read: visible to tenant_admin and platform_admin (enforced at app layer via
-- loginAuditTenantAdminPolicy — the policy returns filter.Eq("id", uuid.Nil) for
-- non-admin users). RLS here allows all tenant rows; app policy restricts further.
CREATE POLICY tenant_isolation ON iam_login_audit
    AS PERMISSIVE FOR ALL TO PUBLIC
    USING (tenant_id = current_tenant_id());

-- No updated_at trigger — append-only.

-- Lookup by user for audit queries.
CREATE INDEX IF NOT EXISTS idx_iam_login_audit_user_id    ON iam_login_audit (tenant_id, user_id)
    WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_iam_login_audit_event      ON iam_login_audit (tenant_id, event);
CREATE INDEX IF NOT EXISTS idx_iam_login_audit_created_at ON iam_login_audit (tenant_id, created_at DESC);
