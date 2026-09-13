-- Migration: create audit log, feature flags, settings, metadata, module registry,
--            and the transactional outbox table.

-- Audit log (global schema — no RLS, read-only for app role)
CREATE TABLE IF NOT EXISTS iam_audit_log (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID NOT NULL REFERENCES platform_tenant(id),
    tenant_id_ref    VARCHAR(36),
    actor_id         VARCHAR(36),
    actor_email      VARCHAR(254),
    ip_address       VARCHAR(45),
    request_id       VARCHAR(36),
    operation        VARCHAR(20) NOT NULL
                         CHECK (operation IN ('create','update','delete','action','login','logout')),
    entity_name      VARCHAR(100) NOT NULL,
    record_id        VARCHAR(36),
    before_snapshot  JSONB,
    after_snapshot   JSONB,
    diff             JSONB,
    custom_fields    JSONB NOT NULL DEFAULT '{}',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Audit log is globally readable by platform admins and filtered by tenant for tenant admins.
-- No RLS — the app layer filters by tenant_id_ref.
CREATE INDEX IF NOT EXISTS iam_audit_log_tenant_idx   ON iam_audit_log (tenant_id_ref);
CREATE INDEX IF NOT EXISTS iam_audit_log_entity_idx   ON iam_audit_log (entity_name, record_id);
CREATE INDEX IF NOT EXISTS iam_audit_log_actor_idx    ON iam_audit_log (actor_id);
CREATE INDEX IF NOT EXISTS iam_audit_log_created_idx  ON iam_audit_log (created_at DESC);

-- Feature flags (global — no RLS)
CREATE TABLE IF NOT EXISTS platform_feature_flag (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID REFERENCES platform_tenant(id), -- NULL = platform-level flag
    key                 VARCHAR(100) NOT NULL UNIQUE,
    label               VARCHAR(255),
    description         VARCHAR(1024),
    default_enabled     BOOLEAN NOT NULL DEFAULT FALSE,
    enabled             BOOLEAN NOT NULL DEFAULT FALSE,
    rollout_percentage  INTEGER NOT NULL DEFAULT 0,
    custom_fields       JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Tenant-level flag overrides
CREATE TABLE IF NOT EXISTS platform_flag_tenant_override (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL REFERENCES platform_tenant(id),
    flag_id       UUID NOT NULL REFERENCES platform_feature_flag(id),
    enabled       BOOLEAN NOT NULL,
    custom_fields JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, flag_id)
);

ALTER TABLE platform_flag_tenant_override ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_flag_tenant_override FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON platform_flag_tenant_override
    USING (tenant_id = current_tenant_id());

-- Settings
CREATE TABLE IF NOT EXISTS platform_setting (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID REFERENCES platform_tenant(id), -- NULL = system scope
    namespace     VARCHAR(100) NOT NULL,
    key           VARCHAR(100) NOT NULL,
    scope         VARCHAR(10) NOT NULL DEFAULT 'tenant'
                      CHECK (scope IN ('system','tenant','branch')),
    scope_ref     VARCHAR(36),
    value         JSONB,
    value_type    VARCHAR(10) NOT NULL DEFAULT 'string'
                      CHECK (value_type IN ('string','number','boolean','json')),
    description   VARCHAR(1024),
    custom_fields JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (namespace, key, scope, scope_ref)
);

ALTER TABLE platform_setting ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_setting FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON platform_setting
    USING (tenant_id IS NULL OR tenant_id = current_tenant_id());

-- Custom fields (metadata)
CREATE TABLE IF NOT EXISTS platform_custom_field (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL REFERENCES platform_tenant(id),
    entity_name   VARCHAR(100) NOT NULL,
    field_name    VARCHAR(100) NOT NULL,
    label         VARCHAR(255),
    field_type    VARCHAR(20) NOT NULL,
    options       JSONB,
    required      BOOLEAN NOT NULL DEFAULT FALSE,
    default_value JSONB,
    active        BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order    INTEGER NOT NULL DEFAULT 0,
    custom_fields JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, entity_name, field_name)
);

ALTER TABLE platform_custom_field ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_custom_field FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON platform_custom_field
    USING (tenant_id = current_tenant_id());

-- Module registry
CREATE TABLE IF NOT EXISTS platform_module (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID REFERENCES platform_tenant(id), -- NULL = platform module
    key           VARCHAR(50) NOT NULL UNIQUE,
    label         VARCHAR(255),
    description   VARCHAR(1024),
    version       VARCHAR(20) NOT NULL DEFAULT '1.0.0',
    active        BOOLEAN NOT NULL DEFAULT TRUE,
    dependencies  JSONB,
    custom_fields JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS platform_tenant_module (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL REFERENCES platform_tenant(id),
    module_key    VARCHAR(50) NOT NULL,
    status        VARCHAR(15) NOT NULL DEFAULT 'installing'
                      CHECK (status IN ('installing','active','suspended','uninstalling')),
    installed_at  TIMESTAMPTZ,
    config        JSONB,
    custom_fields JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, module_key)
);

ALTER TABLE platform_tenant_module ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_tenant_module FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON platform_tenant_module
    USING (tenant_id = current_tenant_id());

-- Transactional outbox table (global — no RLS)
CREATE TABLE IF NOT EXISTS events_outbox (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID,
    type          VARCHAR(50) NOT NULL,
    entity_name   VARCHAR(100) NOT NULL,
    record_id     UUID,
    actor_id      UUID,
    action_name   VARCHAR(100),
    payload       JSONB,
    occurred_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at  TIMESTAMPTZ,
    attempts      INTEGER NOT NULL DEFAULT 0,
    last_error    TEXT
);

CREATE INDEX IF NOT EXISTS events_outbox_pending_idx
    ON events_outbox (occurred_at ASC)
    WHERE delivered_at IS NULL AND attempts < 5;
