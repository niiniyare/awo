-- platform_module: installed modules available on the platform.
-- Global table (no RLS) — one record per module key system-wide.
CREATE TABLE IF NOT EXISTS platform_module (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key          varchar(50)  NOT NULL UNIQUE,
    label        varchar(255),
    description  varchar(1024),
    version      varchar(20)  NOT NULL DEFAULT '1.0.0',
    active       boolean      NOT NULL DEFAULT true,
    dependencies jsonb,
    created_at   timestamptz  NOT NULL DEFAULT now(),
    updated_at   timestamptz  NOT NULL DEFAULT now()
);

-- platform_tenant_module: per-tenant module activation state.
-- RLS enabled — tenant-scoped.
CREATE TABLE IF NOT EXISTS platform_tenant_module (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    uuid NOT NULL REFERENCES platform_tenant (id),
    module_key   varchar(50)  NOT NULL,
    status       varchar(20)  NOT NULL DEFAULT 'installing'
                     CHECK (status IN ('installing','active','suspended','uninstalling')),
    installed_at timestamptz,
    config       jsonb,
    created_at   timestamptz  NOT NULL DEFAULT now(),
    updated_at   timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT platform_tenant_module_unique UNIQUE (tenant_id, module_key)
);

ALTER TABLE platform_tenant_module ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_tenant_module FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON platform_tenant_module
    USING (tenant_id = current_tenant_id());

CREATE INDEX IF NOT EXISTS platform_module_key_idx          ON platform_module (key);
CREATE INDEX IF NOT EXISTS platform_tenant_module_tenant_idx ON platform_tenant_module (tenant_id);
CREATE INDEX IF NOT EXISTS platform_tenant_module_status_idx ON platform_tenant_module (status);
