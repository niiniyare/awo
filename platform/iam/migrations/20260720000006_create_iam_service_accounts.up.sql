-- iam_service_accounts: machine principals for API-to-API integrations.
-- Tenant-scoped: each tenant manages its own service accounts.

CREATE TABLE iam_service_accounts (
    id            UUID         NOT NULL DEFAULT gen_random_uuid(),
    tenant_id     UUID         NOT NULL,
    name          VARCHAR(128) NOT NULL,
    description   VARCHAR(1024),
    status        VARCHAR(16)  NOT NULL DEFAULT 'active'
                      CHECK (status IN ('active', 'inactive')),
    custom_fields JSONB        NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT iam_service_accounts_pkey PRIMARY KEY (id),
    CONSTRAINT iam_service_accounts_name_unique UNIQUE (tenant_id, name)
);

CREATE INDEX iam_service_accounts_custom_fields_idx
    ON iam_service_accounts USING GIN (custom_fields);

ALTER TABLE iam_service_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam_service_accounts FORCE ROW LEVEL SECURITY;

CREATE POLICY iam_service_accounts_tenant_isolation ON iam_service_accounts
    USING (tenant_id = current_tenant_id());
