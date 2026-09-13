-- Migration 000452: Unified Audit Infrastructure (platform_audit_log)
--
-- Creates the platform_audit_log partitioned table, supporting tables,
-- indexes, initial monthly partitions, and the pg_cron maintenance job.
--
-- ADR-018: platform_audit_log is a global table (no RLS). Monthly range
-- partitioning on created_at. Append-only for awo_app database role.
--
-- ADR-019: Feature flag table (platform_audit_config) gates the cutover
-- from the legacy dual-audit system. Deploy with flag=false; enable in
-- Phase 3 after validating the unified system.
--
-- ADR-020: Partition maintenance via pg_cron. The maintenance function
-- platform_create_next_audit_partition() is registered with pg_cron to
-- run on the 25th of each month. If pg_cron is not installed, the function
-- still exists and can be called manually or by an external scheduler.
--
-- Zero-downtime: creates new tables only. No changes to existing tables.
-- Rollback: 000452_platform_audit_log.down.sql

-- ── 1. Feature flag table ──────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS platform_audit_config (
    key         text        PRIMARY KEY,
    value       text        NOT NULL,
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- Seed feature flag: disabled until Phase 3 validation is complete.
INSERT INTO platform_audit_config (key, value)
VALUES ('feature.unified_audit.enabled', 'false')
ON CONFLICT (key) DO NOTHING;

-- ── 2. Main audit table (partitioned) ─────────────────────────────────────────

CREATE TABLE IF NOT EXISTS platform_audit_log (
    -- Identity
    id                  uuid        NOT NULL DEFAULT gen_random_uuid(),
    tenant_id           uuid        NOT NULL,
    request_id          text,

    -- Entity
    entity_name         text        NOT NULL,
    record_id           uuid,
    operation           text        NOT NULL
                        CHECK (operation IN ('create','update','delete','action','login','logout','system')),

    -- Actor (exactly one of actor_id, service_account_id, system_actor is set)
    actor_id            uuid,
    service_account_id  uuid,
    system_actor        text,

    -- Request context
    ip_address          text,
    session_id          text,

    -- Snapshots (sensitive fields stripped before storage)
    before_data         jsonb,
    after_data          jsonb,
    changed_fields      text[],

    -- Classification
    event_category      text        NOT NULL DEFAULT 'DATA'
                        CHECK (event_category IN ('DATA','AUTH','ACCESS','ADMIN','WORKFLOW','SYSTEM','OUTBOUND','SECURITY')),
    severity            text        NOT NULL DEFAULT 'INFO'
                        CHECK (severity IN ('CRITICAL','HIGH','MEDIUM','LOW','INFO')),
    risk_score          smallint    NOT NULL DEFAULT 0 CHECK (risk_score BETWEEN 0 AND 100),
    compliance_flags    jsonb,

    -- Event-type-specific context (schema varies by event_category)
    context             jsonb,

    -- Partition key (must be in PK for partitioned tables)
    created_at          timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (id, created_at)
)
PARTITION BY RANGE (created_at);

-- Disable RLS: platform_audit_log is a global table (ADR-018).
-- Tenant isolation is enforced at the application layer via permission checks.
ALTER TABLE platform_audit_log DISABLE ROW LEVEL SECURITY;

-- ── 3. Indexes (created on parent; inherited by all partitions) ────────────────

-- Primary access pattern: entity change history
CREATE INDEX IF NOT EXISTS audit_entity_history
    ON platform_audit_log (entity_name, record_id, created_at DESC);

-- Actor audit trail
CREATE INDEX IF NOT EXISTS audit_actor
    ON platform_audit_log (actor_id, created_at DESC)
    WHERE actor_id IS NOT NULL;

-- Tenant-scoped audit queries
CREATE INDEX IF NOT EXISTS audit_tenant
    ON platform_audit_log (tenant_id, created_at DESC);

-- Compliance: category + severity queries
CREATE INDEX IF NOT EXISTS audit_category_severity
    ON platform_audit_log (event_category, severity, created_at DESC);

-- Request correlation (X-Request-ID)
CREATE INDEX IF NOT EXISTS audit_request
    ON platform_audit_log (request_id)
    WHERE request_id IS NOT NULL;

-- Session correlation (HMAC digest)
CREATE INDEX IF NOT EXISTS audit_session
    ON platform_audit_log (session_id)
    WHERE session_id IS NOT NULL;

-- Risk score: high-risk event queries
CREATE INDEX IF NOT EXISTS audit_risk_score
    ON platform_audit_log (risk_score DESC, created_at DESC)
    WHERE risk_score >= 50;

-- ── 4. Initial partitions ─────────────────────────────────────────────────────
-- Current month + 2 months ahead. pg_cron creates subsequent partitions.
-- Adjust dates dynamically via the maintenance function (see §8).

DO $$
DECLARE
    base_date   date := date_trunc('month', CURRENT_DATE)::date;
    month_start date;
    month_end   date;
    tbl_name    text;
BEGIN
    -- Create partitions for current month and 2 months ahead
    FOR i IN 0..2 LOOP
        month_start := (base_date + (i || ' months')::interval)::date;
        month_end   := (month_start + '1 month'::interval)::date;
        tbl_name    := 'platform_audit_log_' || to_char(month_start, 'YYYY_MM');

        IF NOT EXISTS (
            SELECT 1 FROM pg_class c
            JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE c.relname = tbl_name AND n.nspname = 'public'
        ) THEN
            EXECUTE format(
                'CREATE TABLE %I PARTITION OF platform_audit_log FOR VALUES FROM (%L) TO (%L)',
                tbl_name, month_start, month_end
            );
        END IF;
    END LOOP;
END $$;

-- Default partition: catches any rows that fall outside all defined ranges.
-- Should always be empty in normal operation; non-empty triggers an alert.
CREATE TABLE IF NOT EXISTS platform_audit_log_default
    PARTITION OF platform_audit_log DEFAULT;

-- ── 5. Integrity checkpoint table ─────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS platform_audit_checkpoint (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    partition_name  text        NOT NULL,
    row_count       bigint      NOT NULL,
    checksum        text        NOT NULL,   -- SHA-256 of sorted(id||created_at) for all rows
    checkpoint_at   timestamptz NOT NULL DEFAULT now(),
    created_by      text        NOT NULL    -- 'system:scheduler' or 'system:manual'
);

REVOKE UPDATE, DELETE ON platform_audit_checkpoint FROM awo_app;
GRANT SELECT, INSERT ON platform_audit_checkpoint TO awo_app;

-- ── 6. Historical migration tracking ──────────────────────────────────────────

CREATE TABLE IF NOT EXISTS platform_audit_migration_log (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    source_table    text        NOT NULL,
    last_migrated_id uuid,
    rows_migrated   bigint      NOT NULL DEFAULT 0,
    status          text        NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','in_progress','complete','failed')),
    started_at      timestamptz,
    completed_at    timestamptz,
    error           text
);

-- ── 7. Permissions ────────────────────────────────────────────────────────────

-- awo_app: INSERT + SELECT only (append-only enforcement).
-- UPDATE and DELETE are reserved for audit_retention_role (GDPR anonymization).
REVOKE UPDATE, DELETE ON platform_audit_log FROM awo_app;
GRANT SELECT, INSERT ON platform_audit_log TO awo_app;

-- Create audit_retention_role if it does not already exist.
DO $$ BEGIN
    CREATE ROLE audit_retention_role NOLOGIN;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

GRANT UPDATE, DELETE ON platform_audit_log TO audit_retention_role;

-- ── 8. Partition maintenance function ─────────────────────────────────────────

CREATE OR REPLACE FUNCTION platform_create_next_audit_partition() RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
    -- Target: the month after next (run on 25th → create partition 2 months out)
    target_start date := date_trunc('month', CURRENT_DATE + interval '2 months')::date;
    target_end   date := (target_start + interval '1 month')::date;
    tbl_name     text := 'platform_audit_log_' || to_char(target_start, 'YYYY_MM');
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE c.relname = tbl_name AND n.nspname = 'public'
    ) THEN
        EXECUTE format(
            'CREATE TABLE %I PARTITION OF platform_audit_log FOR VALUES FROM (%L) TO (%L)',
            tbl_name, target_start, target_end
        );
        RAISE NOTICE 'Created audit partition: %', tbl_name;
    END IF;
END $$;

-- ── 9. pg_cron job (ADR-020) ──────────────────────────────────────────────────
-- Run on the 25th of each month at 09:00 UTC to create the next partition.
-- Conditional: only schedules if pg_cron extension is installed.
-- If pg_cron is unavailable, call platform_create_next_audit_partition() manually.

DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_cron') THEN
        -- Remove any existing job before re-scheduling (idempotent).
        PERFORM cron.unschedule('platform-create-audit-partition')
        WHERE EXISTS (
            SELECT 1 FROM cron.job WHERE jobname = 'platform-create-audit-partition'
        );

        PERFORM cron.schedule(
            'platform-create-audit-partition',
            '0 9 25 * *',
            $$SELECT platform_create_next_audit_partition()$$
        );
    ELSE
        RAISE NOTICE 'pg_cron not installed; schedule platform_create_next_audit_partition() externally on the 25th of each month';
    END IF;
END $$;

-- ── 10. Permission seed ───────────────────────────────────────────────────────

INSERT INTO iam_role_permissions (role_name, permission_identifier)
VALUES
    ('role:platform-admin', 'platform.audit_log.read'),
    ('role:tenant.admin',   'platform.audit_log.read')
ON CONFLICT DO NOTHING;
