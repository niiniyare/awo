-- Audit 002: audit support tables.
--
-- Provides:
--   platform_audit_checkpoint    — partition integrity checksum records
--   platform_audit_config        — runtime feature flags and configuration
--   platform_audit_migration_log — historical data migration progress tracking
--
-- All three tables are global (no RLS, no tenant scoping). They are written
-- exclusively by privileged background operations (maintenance jobs, migration
-- workers) and the audit system itself.

-- ── Table: platform_audit_checkpoint ──────────────────────────────────────────
--
-- Written by the partition maintenance job (same job that creates new partitions).
-- A checkpoint covers the state of a single partition at a point in time:
-- row count and SHA-256 of sorted(id || created_at) for all rows in the partition.
-- Missing or deleted rows are detectable by re-computing the checksum.
--
-- awo_app: INSERT + SELECT (write checkpoints; read for verification).
-- No UPDATE, no DELETE — checkpoints are immutable once written.

CREATE TABLE IF NOT EXISTS platform_audit_checkpoint (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    partition_name  text        NOT NULL,   -- e.g. 'platform_audit_log_2026_07'
    row_count       bigint      NOT NULL,
    checksum        text        NOT NULL,   -- SHA-256 of sorted(id || created_at) for all rows
    checkpoint_at   timestamptz NOT NULL DEFAULT now(),
    created_by      text        NOT NULL    -- 'system:scheduler' | 'system:manual'
);

COMMENT ON TABLE platform_audit_checkpoint IS
    'Partition integrity checkpoint records. One row per checkpoint per partition. '
    'Immutable once written — UPDATE and DELETE are reserved for audit_retention_role only.';

COMMENT ON COLUMN platform_audit_checkpoint.checksum IS
    'SHA-256 hex digest of the concatenation of sorted(id::text || created_at::text) '
    'for all rows in the partition at checkpoint_at. '
    'Recompute and compare to detect missing or tampered rows.';

DO $$
BEGIN
    REVOKE UPDATE, DELETE ON platform_audit_checkpoint FROM awo_app;
EXCEPTION
    WHEN undefined_object THEN NULL;
    WHEN insufficient_privilege THEN NULL;
END
$$;

DO $$
BEGIN
    GRANT SELECT, INSERT ON platform_audit_checkpoint TO awo_app;
EXCEPTION
    WHEN undefined_object THEN NULL;
END
$$;

-- ── Table: platform_audit_config ──────────────────────────────────────────────
--
-- Key-value configuration table for the unified audit system.
-- Used for feature flags (e.g. feature.unified_audit.enabled) and runtime
-- tuning parameters without requiring a code deploy.
--
-- The TransactionalWriter checks feature.unified_audit.enabled on first call.
-- If false, Write() is a no-op and the legacy audit path remains active.
-- This enables zero-downtime rollout and instant rollback (set value = 'false').

CREATE TABLE IF NOT EXISTS platform_audit_config (
    key         text        PRIMARY KEY,
    value       text        NOT NULL,
    description text        NOT NULL DEFAULT '',
    updated_at  timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE platform_audit_config IS
    'Runtime configuration for the unified audit system. '
    'Key-value pairs. Feature flags use the ''feature.*'' prefix. '
    'All changes take effect on the next TransactionalWriter.Write() call.';

COMMENT ON COLUMN platform_audit_config.key IS
    'Configuration key. Reserved namespace: feature.* for feature flags. '
    'Example keys: feature.unified_audit.enabled, audit.risk_score.sensitive_premium.';

CREATE TRIGGER trg_set_updated_at
    BEFORE UPDATE ON platform_audit_config
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Seed the unified audit feature flag as disabled.
-- Phase 3 of the migration strategy enables it:
--   UPDATE platform_audit_config SET value = 'true' WHERE key = 'feature.unified_audit.enabled';
INSERT INTO platform_audit_config (key, value, description) VALUES (
    'feature.unified_audit.enabled',
    'false',
    'Master switch for the unified audit system (platform_audit_log). '
    'When false, TransactionalWriter.Write() is a no-op and the legacy audit path remains active. '
    'Set to ''true'' after validating Phase 2 deployment. Rollback: set to ''false''.'
)
ON CONFLICT (key) DO NOTHING;

DO $$
BEGIN
    GRANT SELECT, INSERT, UPDATE ON platform_audit_config TO awo_app;
EXCEPTION
    WHEN undefined_object THEN NULL;
END
$$;

-- ── Table: platform_audit_migration_log ───────────────────────────────────────
--
-- Tracks progress of the Phase 4 historical migration from legacy audit tables
-- (iam_audit_log, audit_log) to platform_audit_log. One row per source table.
-- The migration job is restartable: it resumes from last_migrated_id on restart.

CREATE TABLE IF NOT EXISTS platform_audit_migration_log (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    source_table    text        NOT NULL,   -- 'iam_audit_log' | 'audit_log'
    last_migrated_id uuid,                  -- last ID migrated from source; NULL = not started
    rows_migrated   bigint      NOT NULL DEFAULT 0,
    status          text        NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','in_progress','complete','failed')),
    started_at      timestamptz,
    completed_at    timestamptz,
    error           text,                   -- last error message; NULL if no error

    CONSTRAINT uq_audit_migration_source UNIQUE (source_table)
);

COMMENT ON TABLE platform_audit_migration_log IS
    'Progress tracking for historical audit data migration (Phase 4). '
    'One row per source table. Restartable: resume from last_migrated_id on restart. '
    'Status machine: pending → in_progress → complete | failed.';

DO $$
BEGIN
    GRANT SELECT, INSERT, UPDATE ON platform_audit_migration_log TO awo_app;
EXCEPTION
    WHEN undefined_object THEN NULL;
END
$$;
