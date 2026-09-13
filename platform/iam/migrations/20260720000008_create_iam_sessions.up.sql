-- iam_sessions: SQL audit trail for authentication sessions.
-- Distinct from the Redis session (which holds live session state for
-- sub-millisecond validation). This table is the durable record for forensics,
-- compliance, and forced-logout by device.
--
-- token_hash is the SHA-256 hex digest of the raw session token.
-- The raw token is NEVER stored in SQL.

CREATE TABLE iam_sessions (
    id                  UUID         NOT NULL DEFAULT gen_random_uuid(),
    tenant_id           UUID         NOT NULL,
    token_hash          CHAR(64)     NOT NULL,  -- SHA-256 hex digest of raw session token
    user_id             UUID         REFERENCES iam_users(id),
    service_account_id  UUID         REFERENCES iam_service_accounts(id),
    issued_at           TIMESTAMPTZ  NOT NULL,
    expires_at          TIMESTAMPTZ  NOT NULL,
    revoked_at          TIMESTAMPTZ,            -- NULL for active/naturally-expired sessions
    device_id           VARCHAR(255),
    ip_address          VARCHAR(45),
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT iam_sessions_pkey PRIMARY KEY (id),
    -- Exactly one of user_id or service_account_id must be non-null.
    CONSTRAINT iam_sessions_principal_check CHECK (
        (user_id IS NOT NULL AND service_account_id IS NULL) OR
        (user_id IS NULL AND service_account_id IS NOT NULL)
    )
);

-- token_hash lookup: used by background job to mark expired sessions.
CREATE UNIQUE INDEX iam_sessions_hash_uidx    ON iam_sessions (token_hash);
-- User session lookup: used for forced logout and audit queries.
CREATE INDEX        iam_sessions_user_idx      ON iam_sessions (tenant_id, user_id) WHERE user_id IS NOT NULL;
CREATE INDEX        iam_sessions_sa_idx        ON iam_sessions (tenant_id, service_account_id) WHERE service_account_id IS NOT NULL;
-- Expiry index: used by background cleanup job.
CREATE INDEX        iam_sessions_expires_idx   ON iam_sessions (expires_at) WHERE revoked_at IS NULL;

ALTER TABLE iam_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam_sessions FORCE ROW LEVEL SECURITY;

CREATE POLICY iam_sessions_tenant_isolation ON iam_sessions
    USING (tenant_id = current_tenant_id());
