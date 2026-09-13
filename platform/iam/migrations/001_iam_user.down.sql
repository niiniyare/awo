DROP TRIGGER  IF EXISTS trg_set_updated_at ON iam_user;
DROP POLICY   IF EXISTS tenant_isolation   ON iam_user;
DROP INDEX    IF EXISTS idx_iam_user_email;
DROP INDEX    IF EXISTS idx_iam_user_tenant_status;
DROP TABLE    IF EXISTS iam_user;
