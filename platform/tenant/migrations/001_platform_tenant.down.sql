-- Rollback tenant 001.

DROP TRIGGER IF EXISTS trg_set_updated_at ON platform_tenant;
DROP INDEX  IF EXISTS idx_platform_tenant_plan;
DROP INDEX  IF EXISTS idx_platform_tenant_status;
DROP TABLE  IF EXISTS platform_tenant;
