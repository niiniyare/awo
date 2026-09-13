-- Reverse: drop all organization tables.
-- WARNING: destroys all org hierarchy, type registry, and assignment data.
-- Ensure backup exists before running in any environment with real tenant data.

DROP TABLE IF EXISTS platform_org_assignment;
DROP TABLE IF EXISTS platform_org_type;
DROP TABLE IF EXISTS platform_organization;
