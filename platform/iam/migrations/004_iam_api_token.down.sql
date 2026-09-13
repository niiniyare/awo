DROP POLICY IF EXISTS tenant_isolation ON iam_api_token;
DROP INDEX  IF EXISTS idx_iam_api_token_revoked;
DROP INDEX  IF EXISTS idx_iam_api_token_sa;
DROP INDEX  IF EXISTS idx_iam_api_token_hash;
DROP TABLE  IF EXISTS iam_api_token;
