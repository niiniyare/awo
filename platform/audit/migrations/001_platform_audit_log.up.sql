-- Audit 001: platform_audit_log — partitioned unified audit event store.
--
-- This is a PLATFORM-LEVEL globally-scoped table. It does NOT use RLS.
-- Access control is application-layer only:
--   - awo_app: INSERT + SELECT (append-only in normal operation)
--   - audit_retention_role: UPDATE + DELETE (GDPR anonymization + archival)
--
-- ADR-018: No RLS because cross-tenant compliance queries (platform-admin) would
-- require disabling RLS per-query or a superuser connection — both worse than
-- application-layer filtering.
--
-- Partition strategy: RANGE on created_at (monthly partitions).
-- Bootstrap creates current month + 2 months ahead. A maintenance job (pg_cron
-- or Temporal scheduled workflow) creates the next month's partition on the
-- 25th of each month. The default partition catches any out-of-range inserts.
--
-- Immutability guarantee: audit records are never hard-deleted by the application.
-- Archival is an ops concern: pg_dump partition → object storage → drop partition.

-- ── Role: audit_retention_role ────────────────────────────────────────────────
--
-- Dedicated PostgreSQL role for GDPR anonymization and archival operations.
-- Only principals explicitly granted this role can UPDATE or DELETE audit rows.
-- The role has NOLOGIN — it must be granted to a DBA user, never used directly.

DO $$
BEGIN
    CREATE ROLE audit_retention_role NOLOGIN;
EXCEPTION
    WHEN duplicate_object THEN NULL;
END
$$;

COMMENT ON ROLE audit_retention_role IS
    'Dedicated PostgreSQL role for GDPR anonymization of platform_audit_log rows. '
    'Grants UPDATE (to null PII columns: actor_id, ip_address, before_data, after_data) '
    'and DELETE (for partition archival). NOLOGIN — granted to DBA users explicitly.';

-- ── Table: platform_audit_log ─────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS platform_audit_log (
    -- Identity
    id                  uuid        NOT NULL DEFAULT gen_random_uuid(),
    tenant_id           uuid        NOT NULL,               -- tenant at event time; NOT a FK (intentional — see comment below)
    request_id          text,                               -- X-Request-ID; NULL for system ops

    -- Entity
    entity_name         text        NOT NULL,               -- qualified name e.g. "finance_invoice"
    record_id           uuid,                               -- affected record PK; NULL for session/access events
    operation           text        NOT NULL
                        CHECK (operation IN ('create','update','delete','action','login','logout','system')),

    -- Actor: exactly one of actor_id, service_account_id, system_actor is non-NULL.
    -- Enforced at the application layer by AuditRecord.Validate(); not a DB constraint
    -- because a CHECK across three nullable columns cannot express the invariant cleanly.
    actor_id            uuid,                               -- human user UUID; NULL for SA/system ops
    service_account_id  uuid,                               -- service account UUID; NULL for human/system ops
    system_actor        text,                               -- e.g. 'system:bootstrap'; NULL for human/SA ops

    -- Request context
    ip_address          text,                               -- client IP (IPv4 or IPv6); NULL for system ops
    session_id          text,                               -- HMAC-SHA256(token, server_secret) hex-encoded; NULL for system ops

    -- Snapshots (sensitive fields stripped before population)
    before_data         jsonb,                              -- NULL on create; sensitive fields stripped
    after_data          jsonb,                              -- NULL on delete; sensitive fields stripped
    changed_fields      text[],                             -- populated on update only; computed from stripped maps

    -- Classification
    event_category      text        NOT NULL DEFAULT 'DATA'
                        CHECK (event_category IN ('DATA','AUTH','ACCESS','ADMIN','WORKFLOW','SYSTEM','OUTBOUND','SECURITY')),
    severity            text        NOT NULL DEFAULT 'INFO'
                        CHECK (severity IN ('CRITICAL','HIGH','MEDIUM','LOW','INFO')),
    risk_score          smallint    NOT NULL DEFAULT 0 CHECK (risk_score BETWEEN 0 AND 100),
    compliance_flags    jsonb,                              -- {"GDPR": true, "PCI_DSS": false, "KRA_ETIMS": true}

    -- Event-type-specific context (JSONB; schema varies by event_category — see AUDIT_STORAGE.md §3)
    context             jsonb,

    -- Partition key: must be included in the PRIMARY KEY for partitioned tables.
    created_at          timestamptz NOT NULL DEFAULT now(),

    -- PK includes created_at because PostgreSQL requires the partition key
    -- to be part of every unique constraint on a partitioned table.
    PRIMARY KEY (id, created_at)
)
PARTITION BY RANGE (created_at);

COMMENT ON TABLE platform_audit_log IS
    'Unified audit event log. Monthly RANGE partitions on created_at. '
    'Append-only for awo_app; UPDATE reserved for GDPR anonymization only (audit_retention_role). '
    'No RLS — tenant scoping is enforced at the application layer (ADR-018).';

COMMENT ON COLUMN platform_audit_log.tenant_id IS
    'Tenant UUID at time of event. Intentionally NOT a foreign key: tenants may be '
    'archived or hard-deleted, but their audit records must be retained indefinitely.';

COMMENT ON COLUMN platform_audit_log.session_id IS
    'HMAC-SHA256(session_token, server_secret) hex-encoded (64 chars). '
    'Raw session token is NEVER stored. Derived by audit.HashSessionToken().';

COMMENT ON COLUMN platform_audit_log.compliance_flags IS
    'Compliance framework applicability flags. '
    'Example: {"GDPR": true, "KRA_ETIMS": true, "PCI_DSS": false}. '
    'Populated per EntityAuditConfig.ComplianceFlags at write time.';

COMMENT ON COLUMN platform_audit_log.context IS
    'Event-type-specific supplementary data. JSONB schema varies by event_category. '
    'See AUDIT_STORAGE.md §3 for per-category schema definitions.';

-- Disable RLS: global table, no per-row tenant isolation at DB level. ADR-018.
ALTER TABLE platform_audit_log DISABLE ROW LEVEL SECURITY;

-- ── Permissions ───────────────────────────────────────────────────────────────
--
-- awo_app: INSERT + SELECT only. Immutability enforced at the role level.
-- audit_retention_role: UPDATE + DELETE for GDPR anonymization and archival.
--
-- Both GRANTs are wrapped in exception handlers to be safe in environments
-- where awo_app may not exist (e.g., early development with a superuser account).

DO $$
BEGIN
    REVOKE UPDATE, DELETE ON platform_audit_log FROM awo_app;
EXCEPTION
    WHEN undefined_object THEN NULL;  -- awo_app not provisioned in this environment
    WHEN insufficient_privilege THEN NULL;
END
$$;

DO $$
BEGIN
    GRANT SELECT, INSERT ON platform_audit_log TO awo_app;
EXCEPTION
    WHEN undefined_object THEN NULL;
END
$$;

DO $$
BEGIN
    GRANT UPDATE, DELETE ON platform_audit_log TO audit_retention_role;
EXCEPTION
    WHEN undefined_object THEN NULL;
END
$$;

-- ── Default partition ─────────────────────────────────────────────────────────
--
-- Catches inserts whose created_at falls outside all defined range partitions.
-- Should never hold rows in normal operation — its presence triggers an alert.
-- Created before range partitions so PostgreSQL has somewhere to route
-- any rows inserted during the DO block below if a race occurs.

CREATE TABLE IF NOT EXISTS platform_audit_log_default
    PARTITION OF platform_audit_log DEFAULT;

-- ── Initial range partitions ──────────────────────────────────────────────────
--
-- Creates partitions for: current month, next month, and two months ahead.
-- Uses dynamic SQL to avoid hardcoded dates — safe to run at any time.
-- Idempotent: skips partitions that already exist (checked via pg_class).

DO $$
DECLARE
    i       int;
    m       timestamptz;
    p_start date;
    p_end   date;
    p_name  text;
BEGIN
    FOR i IN 0..2 LOOP
        m       := date_trunc('month', now()) + (i || ' months')::interval;
        p_start := m::date;
        p_end   := (m + interval '1 month')::date;
        p_name  := 'platform_audit_log_' || to_char(m, 'YYYY_MM');

        IF NOT EXISTS (
            SELECT FROM pg_class c
            JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE n.nspname = current_schema()
              AND c.relname = p_name
        ) THEN
            EXECUTE format(
                'CREATE TABLE %I PARTITION OF platform_audit_log '
                'FOR VALUES FROM (%L) TO (%L)',
                p_name, p_start::text, p_end::text
            );
        END IF;
    END LOOP;
END
$$;

-- ── Indexes ───────────────────────────────────────────────────────────────────
--
-- Indexes on the parent table are automatically inherited by all existing and
-- future partitions. Created after partitions to avoid index rebuilds.

-- Primary access pattern: entity mutation history
CREATE INDEX IF NOT EXISTS audit_entity_history
    ON platform_audit_log (entity_name, record_id, created_at DESC);

-- Actor audit trail (partial: only rows with a human actor_id)
CREATE INDEX IF NOT EXISTS audit_actor
    ON platform_audit_log (actor_id, created_at DESC)
    WHERE actor_id IS NOT NULL;

-- Tenant-scoped audit queries (compliance reports, tenant-admin dashboards)
CREATE INDEX IF NOT EXISTS audit_tenant
    ON platform_audit_log (tenant_id, created_at DESC);

-- Compliance reporting by category and severity
CREATE INDEX IF NOT EXISTS audit_category_severity
    ON platform_audit_log (event_category, severity, created_at DESC);

-- Request correlation: link all audit events from one HTTP request
CREATE INDEX IF NOT EXISTS audit_request
    ON platform_audit_log (request_id)
    WHERE request_id IS NOT NULL;

-- Session correlation: link all audit events from one session
CREATE INDEX IF NOT EXISTS audit_session
    ON platform_audit_log (session_id)
    WHERE session_id IS NOT NULL;

-- High-risk event queries (partial: risk_score >= 50 only)
CREATE INDEX IF NOT EXISTS audit_risk_score
    ON platform_audit_log (risk_score DESC, created_at DESC)
    WHERE risk_score >= 50;
