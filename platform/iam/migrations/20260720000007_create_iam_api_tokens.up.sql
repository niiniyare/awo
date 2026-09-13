-- iam_api_tokens: long-lived API keys for service accounts.
-- token_hash is the SHA-256 hex digest of the raw token; the raw token is
-- returned once at creation and never stored.

CREATE TABLE iam_api_tokens (
    id                  UUID         NOT NULL DEFAULT gen_random_uuid(),
    tenant_id           UUID         NOT NULL,
    service_account_id  UUID         NOT NULL REFERENCES iam_service_accounts(id) ON DELETE CASCADE,
    name                VARCHAR(128) NOT NULL,
    token_hash          CHAR(64)     NOT NULL,  -- SHA-256 hex: always 64 chars
    expires_at          TIMESTAMPTZ,
    last_used_at        TIMESTAMPTZ,
    is_revoked          BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT iam_api_tokens_pkey PRIMARY KEY (id)
);

-- Hash lookup is the critical path for every API token auth request.
CREATE UNIQUE INDEX iam_api_tokens_hash_uidx ON iam_api_tokens (token_hash);
CREATE INDEX        iam_api_tokens_sa_idx     ON iam_api_tokens (tenant_id, service_account_id);

ALTER TABLE iam_api_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam_api_tokens FORCE ROW LEVEL SECURITY;

CREATE POLICY iam_api_tokens_tenant_isolation ON iam_api_tokens
    USING (tenant_id = current_tenant_id());
