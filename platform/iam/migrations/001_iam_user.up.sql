-- IAM 001: iam_user — authenticated human principal.
--
-- password_hash stores only the bcrypt digest of the user's password.
-- The raw password is NEVER stored. The UserPasswordHasher hook replaces
-- incoming plaintext with a bcrypt hash before the record is persisted.
--
-- email is unique per tenant (not globally) — the same email can register
-- in multiple tenants. The UNIQUE constraint is on (tenant_id, email).
--
-- status machine: active ↔ inactive, active → locked (locked requires admin unlock)

CREATE TABLE IF NOT EXISTS iam_user (
    id            uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     uuid         NOT NULL REFERENCES platform_tenant(id),
    created_at    timestamptz  NOT NULL DEFAULT NOW(),
    updated_at    timestamptz  NOT NULL DEFAULT NOW(),

    email         varchar(254) NOT NULL,
    full_name     varchar(255) NOT NULL,
    -- bcrypt hash only; raw password never stored.
    -- Hidden from API responses (Sensitive=true in EntityDefinition).
    password_hash varchar(72)  NOT NULL DEFAULT '',
    status        varchar(20)  NOT NULL DEFAULT 'active'
                      CHECK (status IN ('active', 'inactive', 'locked')),
    email_verified boolean     NOT NULL DEFAULT false,
    -- Written exclusively by AuthService.Login; never by API update.
    last_login_at timestamptz,

    -- Email unique within tenant.
    CONSTRAINT uq_iam_user_tenant_email UNIQUE (tenant_id, email)
);

COMMENT ON TABLE iam_user IS
    'Authenticated human principal. One row per user per tenant. '
    'password_hash stores bcrypt digest only — raw password never persisted.';

COMMENT ON COLUMN iam_user.password_hash IS
    'bcrypt digest of the user password. Hidden from API responses (Sensitive). '
    'Populated exclusively by UserPasswordHasher hook on Create and Update.';

ALTER TABLE iam_user ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam_user FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON iam_user
    AS PERMISSIVE FOR ALL TO PUBLIC
    USING (tenant_id = current_tenant_id());

CREATE TRIGGER trg_set_updated_at
    BEFORE UPDATE ON iam_user
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Indexes for common lookup patterns.
CREATE INDEX IF NOT EXISTS idx_iam_user_tenant_status ON iam_user (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_iam_user_email         ON iam_user (email);
