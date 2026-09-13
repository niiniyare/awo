-- platform_setting: hierarchical key-value configuration.
-- RLS enabled — tenant-scoped. System-scope records have scope_ref empty
-- and are visible to all tenants (read-only via seeded data).
CREATE TABLE IF NOT EXISTS platform_setting (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    namespace   varchar(100) NOT NULL,     -- e.g. "finance.naming_series"
    key         varchar(100) NOT NULL,
    scope       varchar(20)  NOT NULL DEFAULT 'tenant'
                    CHECK (scope IN ('system','tenant','branch')),
    scope_ref   varchar(36)  NOT NULL DEFAULT '',
    value       jsonb,
    value_type  varchar(20)  NOT NULL DEFAULT 'string'
                    CHECK (value_type IN ('string','number','boolean','json')),
    description varchar(1024),
    created_at  timestamptz  NOT NULL DEFAULT now(),
    updated_at  timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT platform_setting_unique UNIQUE (namespace, key, scope, scope_ref)
);

ALTER TABLE platform_setting ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_setting FORCE ROW LEVEL SECURITY;
-- System-scope settings (scope_ref = '') are visible to all tenants.
-- Tenant/branch settings are tenant-scoped.
CREATE POLICY tenant_isolation ON platform_setting
    USING (scope = 'system' OR scope_ref = current_tenant_id()::text OR scope_ref = '');

CREATE INDEX IF NOT EXISTS platform_setting_ns_key_idx  ON platform_setting (namespace, key);
CREATE INDEX IF NOT EXISTS platform_setting_scope_idx   ON platform_setting (scope, scope_ref);
CREATE INDEX IF NOT EXISTS platform_setting_key_trgm    ON platform_setting USING GIN (key gin_trgm_ops);
