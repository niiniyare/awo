DROP POLICY IF EXISTS tenant_isolation ON iam_session;
DROP INDEX  IF EXISTS idx_iam_session_expires_at;
DROP INDEX  IF EXISTS idx_iam_session_user_id;
DROP INDEX  IF EXISTS idx_iam_session_token_hash;
DROP TABLE  IF EXISTS iam_session;
