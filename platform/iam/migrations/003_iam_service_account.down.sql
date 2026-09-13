DROP TRIGGER IF EXISTS trg_set_updated_at ON iam_service_account;
DROP POLICY  IF EXISTS tenant_isolation   ON iam_service_account;
DROP INDEX   IF EXISTS idx_iam_service_account_status;
DROP TABLE   IF EXISTS iam_service_account;
