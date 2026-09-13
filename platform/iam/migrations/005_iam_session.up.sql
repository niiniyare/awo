-- IAM 005: iam_session — durable SQL audit trail of sessions.
--
-- This table is an IMMUTABLE AUDIT LOG distinct from the live Redis session.
-- The Redis session (session:{token}) is the authoritative auth check.
-- This table records who logged in when, from where, for how long.
--
-- token_hash stores SHA-256 hex of the raw bearer token — never the raw token.
-- Revocation is only via the "revoke" action (sets revoked_at); no UPDATE via API.
--
-- Either user_id or service_account_id is set; never both, never neither.

CREATE TABLE IF NOT EXISTS iam_session (
    id                  uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           uuid         NOT NULL REFERENCES platform_tenant(id),
    created_at          timestamptz  NOT NULL DEFAULT NOW(),
    -- No updated_at: sessions are immutable after creation; only revoke action permitted.

    -- SHA-256 hex of raw session token. Sensitive — excluded from API responses.
    token_hash          varchar(64)  NOT NULL,

    -- Principal: exactly one of user_id / service_account_id is non-null.
    user_id             uuid         REFERENCES iam_user(id) ON DELETE SET NULL,
    service_account_id  uuid         REFERENCES iam_service_account(id) ON DELETE SET NULL,

    issued_at           timestamptz  NOT NULL,
    expires_at          timestamptz  NOT NULL,
    revoked_at          timestamptz,  -- set by the "revoke" action

    -- Client metadata (optional).
    device_id           varchar(255),
    ip_address          varchar(45),   -- IPv6 max = 45 chars

    CONSTRAINT uq_iam_session_token_hash UNIQUE (token_hash),
    -- Business rule: at least one principal must be set.
    CONSTRAINT chk_iam_session_principal CHECK (
        (user_id IS NOT NULL AND service_account_id IS NULL) OR
        (user_id IS NULL     AND service_account_id IS NOT NULL)
    )
);

COMMENT ON TABLE iam_session IS
    'Durable audit trail of authenticated sessions. Distinct from live Redis session. '
    'token_hash = SHA-256(raw_token). Immutable after creation; revoke via action.';

COMMENT ON COLUMN iam_session.token_hash IS
    'SHA-256 hex of the raw session token. 64 chars. Sensitive — excluded from logs.';

ALTER TABLE iam_session ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam_session FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON iam_session
    AS PERMISSIVE FOR ALL TO PUBLIC
    USING (tenant_id = current_tenant_id());

-- No updated_at trigger — record is immutable.

-- Lookup by token hash (RevocationCheck path).
CREATE INDEX IF NOT EXISTS idx_iam_session_token_hash ON iam_session (token_hash);
-- Lookup active sessions by user (session listing, role change hook).
CREATE INDEX IF NOT EXISTS idx_iam_session_user_id    ON iam_session (tenant_id, user_id)
    WHERE user_id IS NOT NULL AND revoked_at IS NULL;
-- Expiry cleanup index.
CREATE INDEX IF NOT EXISTS idx_iam_session_expires_at ON iam_session (expires_at)
    WHERE revoked_at IS NULL;
