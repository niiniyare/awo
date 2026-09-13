DROP POLICY IF EXISTS tenant_isolation ON iam_user_role;
DROP INDEX  IF EXISTS idx_iam_user_role_role_name;
DROP INDEX  IF EXISTS idx_iam_user_role_user_id;
DROP TABLE  IF EXISTS iam_user_role;
