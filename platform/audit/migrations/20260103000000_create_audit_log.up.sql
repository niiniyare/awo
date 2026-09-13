-- Migration: create iam_audit_log
-- The audit log lives in the global schema with no RLS — it intentionally
-- spans all tenants so platform admins can run cross-tenant compliance queries.
-- The application role has INSERT-only access; UPDATE and DELETE are denied.

CREATE TABLE IF NOT EXISTS iam_audit_log (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id_ref   varchar(36),
    actor_id        varchar(36),
    actor_email     varchar(254),
    ip_address      varchar(45),
    request_id      varchar(36),
    operation       varchar(20) NOT NULL
                    CHECK (operation IN ('create','update','delete','action','login','logout')),
    entity_name     varchar(100) NOT NULL,
    record_id       varchar(36),
    before_snapshot jsonb,
    after_snapshot  jsonb,
    diff            jsonb,
    created_at      timestamptz NOT NULL DEFAULT NOW(),
    updated_at      timestamptz NOT NULL DEFAULT NOW()
);

-- No RLS on this table — it is global by design.
-- Grant INSERT to app role; deny UPDATE/DELETE at the PostgreSQL role level.

CREATE INDEX IF NOT EXISTS idx_audit_log_tenant    ON iam_audit_log (tenant_id_ref);
CREATE INDEX IF NOT EXISTS idx_audit_log_actor     ON iam_audit_log (actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_entity    ON iam_audit_log (entity_name, record_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_operation ON iam_audit_log (operation);
CREATE INDEX IF NOT EXISTS idx_audit_log_created   ON iam_audit_log (created_at DESC);
