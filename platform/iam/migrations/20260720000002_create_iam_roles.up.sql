-- iam_roles: platform-level role catalogue.
-- Global table: no tenant_id, no RLS. Seeded at bootstrap via migration
-- 20260720000010_seed_iam_system_roles. Tenant admins cannot create or delete roles.

CREATE TABLE iam_roles (
    name        VARCHAR(128) NOT NULL,
    label       VARCHAR(255) NOT NULL,
    description TEXT,
    is_system   BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT iam_roles_pkey PRIMARY KEY (name)
);

COMMENT ON TABLE  iam_roles         IS 'Platform-level role catalogue. Global table — no RLS.';
COMMENT ON COLUMN iam_roles.is_system IS 'TRUE for built-in roles that cannot be deleted.';
