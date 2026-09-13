-- Rollback audit 001: remove platform_audit_log, all partitions, and indexes.
--
-- WARNING: This is irreversible in production. All audit records will be
-- permanently lost. Only run in development environments with no data.
--
-- Rollback order: indexes → partitions → parent table (CASCADE handles partitions).
-- The audit_retention_role is NOT dropped here — roles are DBA-managed and
-- may be in use by other principal assignments.

DROP INDEX IF EXISTS audit_risk_score;
DROP INDEX IF EXISTS audit_session;
DROP INDEX IF EXISTS audit_request;
DROP INDEX IF EXISTS audit_category_severity;
DROP INDEX IF EXISTS audit_tenant;
DROP INDEX IF EXISTS audit_actor;
DROP INDEX IF EXISTS audit_entity_history;

-- CASCADE drops all range partitions and the default partition automatically.
DROP TABLE IF EXISTS platform_audit_log CASCADE;
