-- IAM 002: iam_user_role — role assignment join table.
--
-- Immutable after creation: no UPDATE is permitted via API.
-- AfterCreate and AfterDelete hooks (UserRoleChangeHook) revoke all active
-- Redis sessions for the affected user, forcing re-login so the new role set
-- is reflected in the next session token.
--
-- role_name uses the canonical format: "role:{domain}.{name}"
-- Examples: "role:tenant.admin", "role:finance.viewer"

CREATE TABLE IF NOT EXISTS iam_user_role (
    id             uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      uuid         NOT NULL REFERENCES platform_tenant(id),
    created_at     timestamptz  NOT NULL DEFAULT NOW(),
    -- No updated_at: record is immutable after creation.

    user_id        uuid         NOT NULL REFERENCES iam_user(id) ON DELETE CASCADE,
    role_name      varchar(128) NOT NULL,
    -- Records who granted the role; null for seeded/system-assigned roles.
    granted_by_id  uuid         REFERENCES iam_user(id) ON DELETE SET NULL,

    -- A user cannot hold the same role twice within a tenant.
    CONSTRAINT uq_iam_user_role_tenant_user_role UNIQUE (tenant_id, user_id, role_name)
);

COMMENT ON TABLE iam_user_role IS
    'User-role assignments. Immutable after creation — delete and re-create to change. '
    'AfterCreate/AfterDelete hooks revoke Redis sessions, forcing re-login.';

COMMENT ON COLUMN iam_user_role.role_name IS
    'Canonical role identifier: "role:{domain}.{name}" e.g. "role:tenant.admin". '
    'Matched against Casbin policy by PolicyEvaluator at request time.';

ALTER TABLE iam_user_role ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam_user_role FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON iam_user_role
    AS PERMISSIVE FOR ALL TO PUBLIC
    USING (tenant_id = current_tenant_id());

-- No updated_at trigger — record is immutable.

CREATE INDEX IF NOT EXISTS idx_iam_user_role_user_id   ON iam_user_role (tenant_id, user_id);
CREATE INDEX IF NOT EXISTS idx_iam_user_role_role_name ON iam_user_role (tenant_id, role_name);
