-- platform_organization: arbitrary-depth org tree node.
--
-- TWO-STAGE ISOLATION MODEL
-- =========================
--
-- Stage 1 — Tenant isolation (database layer)
--   Every org table carries tenant_id and has standard tenant RLS.
--   This guarantees cross-tenant data never leaks, identical to all
--   other business entity tables. RLS fires: tenant_id = current_tenant_id().
--
-- Stage 2 — Organization visibility (application layer)
--   Which org nodes a user may see within their tenant is determined
--   entirely by OrganizationService.ResolveScope(). The result is a
--   []uuid.UUID passed as an explicit IN predicate by the application
--   service. RLS has no knowledge of organizational hierarchy.
--
-- These two stages are orthogonal. RLS handles tenant boundaries.
-- The application handles org visibility. Never conflate them.
--
-- Materialized path (path column): "/root-id/parent-id/self-id/"
-- depth: redundant with path; stored for cheap depth-limit enforcement.
-- code: immutable after creation — embedded in workflow IDs and audit records.
-- type: free-form VARCHAR, not a CHECK constraint — metadata-driven.

CREATE TABLE IF NOT EXISTS platform_organization (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES platform_tenant(id),
    parent_id   UUID REFERENCES platform_organization(id),
    name        VARCHAR(255) NOT NULL,
    code        VARCHAR(50)  NOT NULL,
    type        VARCHAR(50),
    path        VARCHAR(4096) NOT NULL DEFAULT '/',
    depth       INTEGER NOT NULL DEFAULT 0,
    description VARCHAR(1024),
    active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    custom_fields JSONB NOT NULL DEFAULT '{}',

    CONSTRAINT platform_organization_code_tenant_unique UNIQUE (tenant_id, code),
    CONSTRAINT platform_organization_depth_nonneg CHECK (depth >= 0)
);

-- Tenant-scoped code lookups.
CREATE INDEX IF NOT EXISTS idx_platform_organization_tenant_code
    ON platform_organization (tenant_id, code);

-- Path-prefix queries: "WHERE path LIKE '/root-id/%'".
CREATE INDEX IF NOT EXISTS idx_platform_organization_path
    ON platform_organization USING btree (path text_pattern_ops);

-- Parent traversal.
CREATE INDEX IF NOT EXISTS idx_platform_organization_parent
    ON platform_organization (parent_id)
    WHERE parent_id IS NOT NULL;

-- Active filter.
CREATE INDEX IF NOT EXISTS idx_platform_organization_tenant_active
    ON platform_organization (tenant_id, active);

-- GIN index for custom_fields JSONB queries.
CREATE INDEX IF NOT EXISTS idx_platform_organization_custom_fields
    ON platform_organization USING GIN (custom_fields);

-- Stage 1: standard tenant RLS — same as every other business table.
-- Stage 2: org visibility is NOT enforced here. Application layer only.
ALTER TABLE platform_organization ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_organization FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON platform_organization
    USING (tenant_id = current_tenant_id());

-- ─────────────────────────────────────────────────────────────────────────────
-- platform_org_type: tenant-defined organization type registry.
-- Types are metadata-driven — tenant admins register valid types.
-- The framework never assumes predefined type values.
CREATE TABLE IF NOT EXISTS platform_org_type (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES platform_tenant(id),
    name        VARCHAR(50) NOT NULL,
    label       VARCHAR(255) NOT NULL,
    description VARCHAR(1024),
    sort_order  INTEGER NOT NULL DEFAULT 0,
    active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT platform_org_type_name_tenant_unique UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_platform_org_type_tenant
    ON platform_org_type (tenant_id, active, sort_order);

ALTER TABLE platform_org_type ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_org_type FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON platform_org_type
    USING (tenant_id = current_tenant_id());

-- ─────────────────────────────────────────────────────────────────────────────
-- platform_org_assignment: user ↔ organization membership.
-- Users may belong to multiple organizations with different roles in each.
-- is_primary: home organization (at most one per user per tenant).
CREATE TABLE IF NOT EXISTS platform_org_assignment (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES platform_tenant(id),
    user_id         UUID NOT NULL REFERENCES iam_user(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES platform_organization(id) ON DELETE CASCADE,
    role            VARCHAR(50) NOT NULL DEFAULT 'member',
    is_primary      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT platform_org_assignment_unique UNIQUE (tenant_id, user_id, organization_id)
);

CREATE INDEX IF NOT EXISTS idx_platform_org_assignment_user
    ON platform_org_assignment (tenant_id, user_id);

CREATE INDEX IF NOT EXISTS idx_platform_org_assignment_org
    ON platform_org_assignment (tenant_id, organization_id);

-- Enforce at most one primary org per user per tenant.
CREATE UNIQUE INDEX IF NOT EXISTS idx_platform_org_assignment_primary
    ON platform_org_assignment (tenant_id, user_id)
    WHERE is_primary = TRUE;

ALTER TABLE platform_org_assignment ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_org_assignment FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON platform_org_assignment
    USING (tenant_id = current_tenant_id());

-- ─────────────────────────────────────────────────────────────────────────────
-- Migration compatibility:
-- platform_org_unit → platform_organization (type preserved)
-- platform_branch   → platform_organization (type = 'branch')
-- Data migration is out-of-band; a separate script transforms existing rows.
