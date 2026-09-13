-- Seed the built-in platform roles.
-- These roles are referenced by iam_user_roles and iam_role_permissions.
-- is_system = TRUE prevents deletion of these roles via the admin UI.
--
-- Role naming convention: "role:{domain}.{name}"
--   role:platform-admin  → cross-tenant platform administrator (bypasses Casbin entirely)
--   role:tenant.admin    → full access within one tenant
--   role:tenant.user     → standard authenticated user within a tenant
--   role:api-client      → machine-to-machine, limited scopes (service accounts)
--
-- Role-to-permission bindings are declared in module-level seed migrations, not
-- here. This migration only establishes the role catalogue.

INSERT INTO iam_roles (name, label, description, is_system) VALUES
    (
        'role:platform-admin',
        'Platform Administrator',
        'Unrestricted access across all tenants. Bypasses all Casbin authorization checks. Assign with extreme caution.',
        TRUE
    ),
    (
        'role:tenant.admin',
        'Tenant Administrator',
        'Full read and write access within a single tenant. Can manage users, roles, and all module data.',
        TRUE
    ),
    (
        'role:tenant.user',
        'Tenant User',
        'Standard authenticated user. Access is restricted by module-level permission grants.',
        TRUE
    ),
    (
        'role:api-client',
        'API Client',
        'Machine-to-machine service account role. Limited to explicitly granted permission scopes.',
        TRUE
    )
ON CONFLICT (name) DO NOTHING;
