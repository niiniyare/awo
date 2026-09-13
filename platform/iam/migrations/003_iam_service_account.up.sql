-- IAM 003: iam_service_account — machine principal for API integrations.
--
-- Service accounts authenticate via long-lived API tokens (iam_api_token).
-- Sessions for service accounts are stateless — no Redis entry is created;
-- instead a 60-second cache of the token hash lookup is used.

CREATE TABLE IF NOT EXISTS iam_service_account (
    id          uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid         NOT NULL REFERENCES platform_tenant(id),
    created_at  timestamptz  NOT NULL DEFAULT NOW(),
    updated_at  timestamptz  NOT NULL DEFAULT NOW(),

    name        varchar(128) NOT NULL,
    description varchar(1024),
    status      varchar(20)  NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active', 'inactive')),

    -- Name unique within tenant.
    CONSTRAINT uq_iam_service_account_tenant_name UNIQUE (tenant_id, name)
);

COMMENT ON TABLE iam_service_account IS
    'Machine principal for integrations and automation. Authenticates via iam_api_token. '
    'Stateless sessions — no Redis entry; auth uses a 60-second token hash cache.';

ALTER TABLE iam_service_account ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam_service_account FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON iam_service_account
    AS PERMISSIVE FOR ALL TO PUBLIC
    USING (tenant_id = current_tenant_id());

CREATE TRIGGER trg_set_updated_at
    BEFORE UPDATE ON iam_service_account
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX IF NOT EXISTS idx_iam_service_account_status ON iam_service_account (tenant_id, status);
