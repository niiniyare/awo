-- Bootstrap 002: Shared utility functions and tables used by all AWO modules.
--
-- Provides:
--   set_tenant_context()         — the SOLE RLS enforcement point (validates + sets awo.tenant_id)
--   current_tenant_id()          — reads awo.tenant_id session variable (RLS gate)
--   set_updated_at()             — trigger function; auto-maintains updated_at column
--   awo_naming_series            — counter table for formatted sequential IDs
--   next_naming_series()         — atomic increment with upsert for naming series

-- ── Tenant context ────────────────────────────────────────────────────────────

-- set_tenant_context() is the single RLS enforcement point (docs/04-multitenancy/
-- RLS_SPEC.md, docs/04-multitenancy/TENANT_LIFECYCLE.md §5 normative requirement).
-- It MUST be called before any tenant-scoped query — contrib/pgx's driver calls
-- it via "SELECT set_tenant_context($1)" inside WithTx, before any DML.
--
-- Phase 1 fix: prior to this migration, no set_tenant_context() function
-- existed anywhere in the embedded (production-default) migration path at
-- all — contrib/pgx's "SELECT set_tenant_context($1)" call would fail with
-- "function set_tenant_context(uuid) does not exist" on any freshly
-- bootstrapped database. Where a compatible-signature version existed
-- elsewhere (generator/generator.go's template, the orphaned top-level
-- migrations/20260706000000_bootstrap.up.sql), it never validated tenant
-- existence or ACTIVE status, contradicting the "MUST reject non-ACTIVE
-- tenants" normative requirement and leaving zero defense-in-depth for any
-- caller other than the one HTTP middleware (api/middleware/tenant.go) that
-- happened to check status itself. See AUDIT_REPORT.md §4/§9 S1 and
-- tasks.md Phase 1.1.
--
-- This function is created before platform_tenant exists in migration
-- ordering (platform/tenant/migrations DependsOn "bootstrap", so it applies
-- after this file) — that is safe: plpgsql resolves table references at
-- EXECUTION time, not at CREATE FUNCTION time, and this function is never
-- invoked until real request traffic begins, long after platform_tenant
-- exists.
CREATE OR REPLACE FUNCTION set_tenant_context(p_tenant_id uuid) RETURNS void
    LANGUAGE plpgsql AS
$$
DECLARE
    v_status text;
BEGIN
    SELECT status INTO v_status FROM platform_tenant WHERE id = p_tenant_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'tenant_not_found: %', p_tenant_id
            USING ERRCODE = 'P0001';
    END IF;

    IF v_status != 'ACTIVE' THEN
        RAISE EXCEPTION 'tenant_not_active: % (status=%)', p_tenant_id, v_status
            USING ERRCODE = 'P0002';
    END IF;

    PERFORM set_config('awo.tenant_id', p_tenant_id::text, true);
END;
$$;

COMMENT ON FUNCTION set_tenant_context(uuid) IS
    'The sole RLS enforcement point. Validates the tenant exists and is '
    'ACTIVE (raising P0001/P0002 otherwise), then sets the transaction-local '
    'awo.tenant_id GUC read by current_tenant_id(). Must be called before '
    'any tenant-scoped query — never call set_config(''awo.tenant_id'', ...) directly.';

-- current_tenant_id() is the RLS gate used by every tenant-scoped table policy.
-- Only ever set via set_tenant_context() above — never call SET/set_config
-- directly, since that bypasses tenant existence/status validation.
--
-- Returns NULL (not an error) when no tenant is set — platform-wide queries
-- (e.g. reading platform_tenant from a platform-admin context) operate without
-- a tenant filter. Row-level security policies must use USING (tenant_id = current_tenant_id())
-- rather than asserting non-null, so platform-admin bypass works correctly.
CREATE OR REPLACE FUNCTION current_tenant_id() RETURNS uuid
    LANGUAGE sql STABLE SECURITY DEFINER
    SET search_path = public AS
$$
    SELECT NULLIF(current_setting('awo.tenant_id', true), '')::uuid;
$$;

COMMENT ON FUNCTION current_tenant_id() IS
    'Returns the tenant UUID set by set_tenant_context() for the current transaction. '
    'Used in all tenant-scoped RLS policies. Returns NULL for platform-admin connections.';

-- ── Auto updated_at ──────────────────────────────────────────────────────────

-- set_updated_at() is applied as a BEFORE UPDATE trigger on every entity table.
-- The trigger is named "trg_set_updated_at" on each table.
CREATE OR REPLACE FUNCTION set_updated_at()
    RETURNS trigger LANGUAGE plpgsql AS
$$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

COMMENT ON FUNCTION set_updated_at() IS
    'Trigger function: automatically sets updated_at = NOW() on every UPDATE. '
    'Applied to all AWO entity tables via trg_set_updated_at trigger.';

-- ── Naming series ─────────────────────────────────────────────────────────────

-- awo_naming_series stores per-tenant, per-series, per-year sequence counters.
-- The table is NOT tenant-scoped (no RLS) because the naming package writes
-- to it from a platform-level connection before the tenant context is set.
-- Access is controlled at the application layer by the naming package.
CREATE TABLE IF NOT EXISTS awo_naming_series (
    series_key  varchar(200) NOT NULL,
    tenant_id   uuid         NOT NULL,
    year        int          NOT NULL,
    current_seq bigint       NOT NULL DEFAULT 0,

    PRIMARY KEY (series_key, tenant_id, year)
);

COMMENT ON TABLE awo_naming_series IS
    'Sequence counters for NamingSeries auto-ID generation (e.g. INV-2026-000001). '
    'One row per (series_key, tenant_id, year). Not RLS-protected; access controlled '
    'exclusively by the framework naming package.';

-- next_naming_series() atomically increments the counter and returns the new value.
-- Uses INSERT ... ON CONFLICT to initialise the counter on first use.
CREATE OR REPLACE FUNCTION next_naming_series(
    p_series_key varchar(200),
    p_tenant_id  uuid,
    p_year       int
) RETURNS bigint
    LANGUAGE plpgsql AS
$$
DECLARE
    next_val bigint;
BEGIN
    INSERT INTO awo_naming_series (series_key, tenant_id, year, current_seq)
    VALUES (p_series_key, p_tenant_id, p_year, 1)
    ON CONFLICT (series_key, tenant_id, year)
    DO UPDATE SET current_seq = awo_naming_series.current_seq + 1
    RETURNING current_seq INTO next_val;
    RETURN next_val;
END;
$$;

COMMENT ON FUNCTION next_naming_series(varchar, uuid, int) IS
    'Atomically increments and returns the next sequence value for a naming series. '
    'Thread-safe via ON CONFLICT DO UPDATE (no gap possible under concurrent load).';
