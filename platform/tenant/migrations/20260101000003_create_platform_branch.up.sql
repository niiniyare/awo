-- platform_branch: physical branch / location within a tenant.
CREATE TABLE IF NOT EXISTS platform_branch (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    uuid NOT NULL REFERENCES platform_tenant (id),
    org_unit_id  uuid REFERENCES platform_org_unit (id),
    name         varchar(255) NOT NULL,
    code         varchar(50)  NOT NULL,
    address      varchar(1024),
    phone        varchar(30),
    active       boolean NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT platform_branch_code_unique UNIQUE (tenant_id, code)
);

ALTER TABLE platform_branch ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_branch FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON platform_branch
    USING (tenant_id = current_tenant_id());

CREATE INDEX IF NOT EXISTS platform_branch_tenant_idx   ON platform_branch (tenant_id);
CREATE INDEX IF NOT EXISTS platform_branch_org_unit_idx ON platform_branch (org_unit_id);
CREATE INDEX IF NOT EXISTS platform_branch_name_trgm    ON platform_branch USING GIN (name gin_trgm_ops);
