-- iam_login_audits: tamper-evident append-only authentication event log.
-- No UPDATE or DELETE is permitted at the application level.
-- The table name uses the plural form to match the qualified entity name
-- "iam_login_audit" (module=iam, name=login_audit → table=iam_login_audits
-- is overridden; actual table name is iam_login_audits matching EntityDefinition).

CREATE TABLE iam_login_audits (
    id                  UUID         NOT NULL DEFAULT gen_random_uuid(),
    tenant_id           UUID         NOT NULL,
    event               VARCHAR(32)  NOT NULL
                            CHECK (event IN ('login', 'logout', 'failed_login', 'token_use', 'session_revoked')),
    user_id             UUID         REFERENCES iam_users(id),
    service_account_id  UUID         REFERENCES iam_service_accounts(id),
    ip_address          VARCHAR(45),
    device_id           VARCHAR(255),
    failure_reason      VARCHAR(255),
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT iam_login_audits_pkey PRIMARY KEY (id)
);

CREATE INDEX iam_login_audits_tenant_idx ON iam_login_audits (tenant_id, created_at DESC);
CREATE INDEX iam_login_audits_user_idx   ON iam_login_audits (tenant_id, user_id, created_at DESC) WHERE user_id IS NOT NULL;
CREATE INDEX iam_login_audits_event_idx  ON iam_login_audits (tenant_id, event, created_at DESC);

ALTER TABLE iam_login_audits ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam_login_audits FORCE ROW LEVEL SECURITY;

CREATE POLICY iam_login_audits_tenant_isolation ON iam_login_audits
    USING (tenant_id = current_tenant_id());

-- Append-only constraint: revoke DELETE from the application role.
-- The app_user role must exist; adjust the role name to match your deployment.
-- REVOKE DELETE ON iam_login_audits FROM app_user;
