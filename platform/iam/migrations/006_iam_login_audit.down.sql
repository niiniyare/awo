DROP POLICY IF EXISTS tenant_isolation ON iam_login_audit;
DROP INDEX  IF EXISTS idx_iam_login_audit_created_at;
DROP INDEX  IF EXISTS idx_iam_login_audit_event;
DROP INDEX  IF EXISTS idx_iam_login_audit_user_id;
DROP TABLE  IF EXISTS iam_login_audit;
