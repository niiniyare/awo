-- iam_role_permissions: role-to-permission bindings.
-- Global table: no tenant_id, no RLS.
-- Consumed by AuthService.LoadRolePermissions() at startup to initialize
-- CasbinEvaluator Phase 2. Changes require CasbinEvaluator.Reload().

CREATE TABLE iam_role_permissions (
    role_name               VARCHAR(128) NOT NULL REFERENCES iam_roles(name) ON DELETE CASCADE,
    permission_identifier   VARCHAR(255) NOT NULL REFERENCES iam_permissions(identifier) ON DELETE CASCADE,
    granted_at              TIMESTAMPTZ  NOT NULL DEFAULT now(),
    granted_by              VARCHAR(255),  -- platform admin identifier (informational)

    CONSTRAINT iam_role_permissions_pkey PRIMARY KEY (role_name, permission_identifier)
);

CREATE INDEX iam_role_permissions_role_idx ON iam_role_permissions (role_name);
CREATE INDEX iam_role_permissions_perm_idx ON iam_role_permissions (permission_identifier);

COMMENT ON TABLE iam_role_permissions IS 'Role-to-permission bindings. Global table — no RLS. Loaded at startup for Casbin Phase 2.';
