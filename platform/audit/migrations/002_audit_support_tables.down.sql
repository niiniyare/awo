-- Rollback audit 002: remove audit support tables.

DROP TRIGGER  IF EXISTS trg_set_updated_at             ON platform_audit_config;
DROP TABLE    IF EXISTS platform_audit_migration_log;
DROP TABLE    IF EXISTS platform_audit_config;
DROP TABLE    IF EXISTS platform_audit_checkpoint;
