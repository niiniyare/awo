-- iam_audit_log: immutable, tamper-evident record of all mutations.
-- No RLS — global table readable by platform admins only.
-- Entries span tenants; compliance queries do not context-switch per row.
CREATE TABLE IF NOT EXISTS iam_audit_log (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id_ref   varchar(36),        -- denormalized; not a FK (cross-tenant scope)
    actor_id        varchar(36),
    actor_email     varchar(254),
    ip_address      varchar(45),
    request_id      varchar(36),
    operation       varchar(20)  NOT NULL
                        CHECK (operation IN ('create','update','delete','action','login','logout')),
    entity_name     varchar(100) NOT NULL,
    record_id       varchar(36),
    before_snapshot jsonb,
    after_snapshot  jsonb,
    diff            jsonb,
    created_at      timestamptz  NOT NULL DEFAULT now(),
    updated_at      timestamptz  NOT NULL DEFAULT now()
);

-- No RLS on audit log — intentional.
CREATE INDEX IF NOT EXISTS iam_audit_log_tenant_ref_idx  ON iam_audit_log (tenant_id_ref);
CREATE INDEX IF NOT EXISTS iam_audit_log_actor_idx       ON iam_audit_log (actor_id);
CREATE INDEX IF NOT EXISTS iam_audit_log_entity_idx      ON iam_audit_log (entity_name, record_id);
CREATE INDEX IF NOT EXISTS iam_audit_log_created_at_idx  ON iam_audit_log (created_at DESC);
