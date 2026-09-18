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
--
-- Phase 1 fix: this table previously had NO tenant_id column and a RLS
-- policy of "USING (true)" — i.e. RLS was nominally "enabled" but enforced
-- nothing at all; every tenant could read and write every other tenant's
-- feature-flag overrides. TenantOverrideDefinition (platform/flags/definition.go)
-- has no explicit Scope, so it defaults to ScopeTenant (ADR-022) and the
-- generator (generator/generator.go generateEntitySQL) would emit a real
-- tenant_id column + "tenant_id = current_tenant_id()" policy for it — this
-- hand-written migration had drifted out of sync with that contract. See
-- AUDIT_REPORT.md / tasks.md Phase 1 for the finding.
CREATE TABLE IF NOT EXISTS platform_flag_tenant_override (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid NOT NULL REFERENCES platform_tenant (id),
    flag_id     uuid NOT NULL REFERENCES platform_feature_flag (id),
    enabled     boolean NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT platform_flag_tenant_override_uniq UNIQUE (tenant_id, flag_id)
);

ALTER TABLE platform_flag_tenant_override ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_flag_tenant_override FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON platform_flag_tenant_override
    USING (tenant_id = current_tenant_id());

CREATE INDEX IF NOT EXISTS platform_feature_flag_key_idx     ON platform_feature_flag (key);
CREATE INDEX IF NOT EXISTS platform_flag_override_tenant_idx ON platform_flag_tenant_override (tenant_id);
CREATE INDEX IF NOT EXISTS platform_flag_override_flag_idx   ON platform_flag_tenant_override (flag_id);
