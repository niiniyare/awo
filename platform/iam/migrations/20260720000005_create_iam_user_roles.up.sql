-- iam_user_roles: tenant-scoped assignment of a platform role to a user.
-- One record per (user_id, role_name) within a tenant.
-- AfterCreate/AfterDelete on this table trigger session revocation.

CREATE TABLE iam_user_roles (
    id             UUID         NOT NULL DEFAULT gen_random_uuid(),
    tenant_id      UUID         NOT NULL,
    user_id        UUID         NOT NULL REFERENCES iam_users(id) ON DELETE CASCADE,
    role_name      VARCHAR(128) NOT NULL REFERENCES iam_roles(name) ON DELETE CASCADE,
    granted_by_id  UUID         REFERENCES iam_users(id),
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT iam_user_roles_pkey PRIMARY KEY (id),
    CONSTRAINT iam_user_roles_unique UNIQUE (tenant_id, user_id, role_name)
);

CREATE INDEX iam_user_roles_user_idx ON iam_user_roles (tenant_id, user_id);

-- RLS: each tenant sees only its own role assignments.
ALTER TABLE iam_user_roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam_user_roles FORCE ROW LEVEL SECURITY;

CREATE POLICY iam_user_roles_tenant_isolation ON iam_user_roles
    USING (tenant_id = current_tenant_id());
