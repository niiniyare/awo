-- Remove system role seeds.
-- WARNING: This will cascade-delete all iam_role_permissions and iam_user_roles
-- referencing these roles. Only run in development or test environments.
DELETE FROM iam_roles WHERE is_system = TRUE;
