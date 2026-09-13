-- Rollback 000452: Unified Audit Infrastructure
--
-- Drops all objects created by 000452_platform_audit_log.up.sql in
-- reverse dependency order. Dropping platform_audit_log CASCADE removes
-- all monthly partitions and the default partition automatically.
--
-- Data loss warning: all audit records are permanently deleted.
-- Only run after confirming audit data is no longer needed or has been
-- exported per retention policy.

-- ── 1. pg_cron job ────────────────────────────────────────────────────────────

DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_cron') THEN
        PERFORM cron.unschedule('platform-create-audit-partition')
        WHERE EXISTS (
            SELECT 1 FROM cron.job WHERE jobname = 'platform-create-audit-partition'
        );
    END IF;
END $$;

-- ── 2. Partition maintenance function ─────────────────────────────────────────

DROP FUNCTION IF EXISTS platform_create_next_audit_partition();

-- ── 3. Supporting tables ───────────────────────────────────────────────────────

DROP TABLE IF EXISTS platform_audit_migration_log;
DROP TABLE IF EXISTS platform_audit_checkpoint;

-- ── 4. Main audit table (CASCADE drops all partitions) ────────────────────────

DROP TABLE IF EXISTS platform_audit_log CASCADE;

-- ── 5. Feature flag table ─────────────────────────────────────────────────────

DROP TABLE IF EXISTS platform_audit_config;

-- ── 6. Permission seeds ───────────────────────────────────────────────────────

DELETE FROM iam_role_permissions
WHERE permission_identifier = 'platform.audit_log.read';
