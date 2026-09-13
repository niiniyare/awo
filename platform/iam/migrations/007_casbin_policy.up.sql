-- IAM 007: Casbin policy storage for the AWO PolicyEvaluator.
--
-- This table stores RBAC policy rules in Casbin's standard format.
-- The AWO framework uses Casbin v2 as the default PolicyEvaluator backend.
--
-- Row format (ptype='p'): sub=role_name, obj=permission_id, act="allow"
-- Row format (ptype='g'): sub=user_id or role_name, obj=role_name
--
-- This table is NOT tenant-scoped by RLS. Instead, policies are namespaced
-- by tenant_id as a data column. The Casbin adapter filters by tenant_id
-- at query time, giving per-tenant RBAC isolation without PostgreSQL RLS.
-- Platform-admin policies use tenant_id = '00000000-0000-0000-0000-000000000000'
-- (uuid.Nil sentinel) to represent platform-wide grants.

CREATE TABLE IF NOT EXISTS casbin_rule (
    id         bigserial    PRIMARY KEY,
    ptype      varchar(10)  NOT NULL,   -- 'p' (policy) or 'g' (role grouping)
    tenant_id  uuid         NOT NULL,   -- tenant scope; uuid.Nil for platform-wide
    v0         varchar(256),
    v1         varchar(256),
    v2         varchar(256),
    v3         varchar(256),
    v4         varchar(256),
    v5         varchar(256),

    -- Ensure no duplicate rules per tenant.
    CONSTRAINT uq_casbin_rule UNIQUE (ptype, tenant_id, v0, v1, v2, v3, v4, v5)
);

COMMENT ON TABLE casbin_rule IS
    'Casbin RBAC policy storage. Rows are namespaced by tenant_id. '
    'uuid.Nil tenant_id is reserved for platform-wide (cross-tenant) grants. '
    'Not RLS-scoped; isolation enforced by Casbin adapter WHERE clause.';

-- Lookup index used by the Casbin adapter on every policy check.
CREATE INDEX IF NOT EXISTS idx_casbin_rule_tenant ON casbin_rule (tenant_id, ptype);
