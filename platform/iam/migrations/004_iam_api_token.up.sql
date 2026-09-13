-- IAM 004: iam_api_token — long-lived API key for service accounts.
--
-- The raw token is generated once at creation and returned to the caller.
-- It is NEVER stored in the database. Only the SHA-256 hex digest (token_hash)
-- is persisted. Revocation sets is_revoked=true; the record is retained for audit.
--
-- Immutable after creation: delete or use the "revoke" action to invalidate.

CREATE TABLE IF NOT EXISTS iam_api_token (
    id                  uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           uuid         NOT NULL REFERENCES platform_tenant(id),
    created_at          timestamptz  NOT NULL DEFAULT NOW(),
    -- No updated_at: immutable after creation (only revoke action permitted).

    service_account_id  uuid         NOT NULL REFERENCES iam_service_account(id) ON DELETE CASCADE,
    name                varchar(128) NOT NULL,
    -- SHA-256 hex of raw token; raw token never stored. Always 64 chars.
    -- Hidden from API responses (Sensitive=true).
    token_hash          varchar(64)  NOT NULL,
    expires_at          timestamptz,  -- NULL = never expires
    last_used_at        timestamptz,  -- updated by AuthService on each token use
    is_revoked          boolean      NOT NULL DEFAULT false,

    -- A service account cannot have two tokens with the same name.
    CONSTRAINT uq_iam_api_token_sa_name UNIQUE (service_account_id, name),
    -- Token hash globally unique (SHA-256 collision resistance).
    CONSTRAINT uq_iam_api_token_hash    UNIQUE (token_hash)
);

COMMENT ON TABLE iam_api_token IS
    'Long-lived API key for service account authentication. '
    'token_hash is SHA-256 hex of raw token; raw token never stored. '
    'Revoke via action; record retained for audit trail.';

COMMENT ON COLUMN iam_api_token.token_hash IS
    'SHA-256 hex digest of the raw bearer token. 64 characters. '
    'Sensitive — excluded from all API responses and structured logs.';

ALTER TABLE iam_api_token ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam_api_token FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON iam_api_token
    AS PERMISSIVE FOR ALL TO PUBLIC
    USING (tenant_id = current_tenant_id());

-- No updated_at trigger — record is immutable (last_used_at updated directly by service).

-- Lookup by token hash (authentication hot path).
CREATE INDEX IF NOT EXISTS idx_iam_api_token_hash      ON iam_api_token (token_hash);
CREATE INDEX IF NOT EXISTS idx_iam_api_token_sa        ON iam_api_token (service_account_id);
CREATE INDEX IF NOT EXISTS idx_iam_api_token_revoked   ON iam_api_token (is_revoked) WHERE NOT is_revoked;
