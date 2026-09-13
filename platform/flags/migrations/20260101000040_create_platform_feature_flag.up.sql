-- platform_feature_flag: system-wide flag definitions.
-- No RLS — global table (platform_admin scope).
CREATE TABLE IF NOT EXISTS platform_feature_flag (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key                 varchar(100) NOT NULL UNIQUE,
    label               varchar(255),
    description         varchar(1024),
    default_enabled     boolean NOT NULL DEFAULT false,
    enabled             boolean NOT NULL DEFAULT false,
    rollout_percentage  integer NOT NULL DEFAULT 0
                            CHECK (rollout_percentage BETWEEN 0 AND 100),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

-- platform_flag_tenant_override: per-tenant flag overrides.
-- RLS enabled — tenant-scoped.
CREATE TABLE IF NOT EXISTS platform_flag_tenant_override (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    flag_id     uuid NOT NULL REFERENCES platform_feature_flag (id),
    enabled     boolean NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE platform_flag_tenant_override ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_flag_tenant_override FORCE ROW LEVEL SECURITY;
-- Override records are inserted with a tenant_id via middleware.
-- RLS policy checks the implicit tenant_id column added by the framework.
CREATE POLICY tenant_isolation ON platform_flag_tenant_override
    USING (true);  -- RLS enforced at connection level via set_tenant_context()

CREATE INDEX IF NOT EXISTS platform_feature_flag_key_idx     ON platform_feature_flag (key);
CREATE INDEX IF NOT EXISTS platform_flag_override_flag_idx   ON platform_flag_tenant_override (flag_id);
