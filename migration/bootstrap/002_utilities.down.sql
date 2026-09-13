-- Rollback bootstrap 002: remove shared utility functions and tables.

DROP FUNCTION IF EXISTS next_naming_series(varchar, uuid, int);
DROP TABLE IF EXISTS awo_naming_series;
DROP FUNCTION IF EXISTS set_updated_at();
DROP FUNCTION IF EXISTS current_tenant_id();
