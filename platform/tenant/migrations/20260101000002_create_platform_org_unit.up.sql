-- platform_org_unit: organisational hierarchy (company → division → department → team).
-- RLS enabled — tenant-scoped.
CREATE TABLE IF NOT EXISTS platform_org_unit (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid NOT NULL REFERENCES platform_tenant (id),
    parent_id   uuid REFERENCES platform_org_unit (id),
    name        varchar(255) NOT NULL,
    code        varchar(50)  NOT NULL,
    type        varchar(20)
                    CHECK (type IN ('company','division','department','branch','team')),
    active      boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT platform_org_unit_code_unique UNIQUE (tenant_id, code)
);

ALTER TABLE platform_org_unit ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_org_unit FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON platform_org_unit
    USING (tenant_id = current_tenant_id());

CREATE INDEX IF NOT EXISTS platform_org_unit_tenant_idx  ON platform_org_unit (tenant_id);
CREATE INDEX IF NOT EXISTS platform_org_unit_parent_idx  ON platform_org_unit (parent_id);
CREATE INDEX IF NOT EXISTS platform_org_unit_name_trgm   ON platform_org_unit USING GIN (name gin_trgm_ops);
