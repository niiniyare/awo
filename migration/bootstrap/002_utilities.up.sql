-- Bootstrap 002: Shared utility functions and tables used by all AWO modules.
--
-- Provides:
--   current_tenant_id()          — reads awo.tenant_id session variable (RLS gate)
--   set_updated_at()             — trigger function; auto-maintains updated_at column
--   awo_naming_series            — counter table for formatted sequential IDs
--   next_naming_series()         — atomic increment with upsert for naming series

-- ── Tenant context ────────────────────────────────────────────────────────────

-- current_tenant_id() is the RLS gate used by every tenant-scoped table policy.
-- The PostgreSQL driver (contrib/pgx) must set this via:
--   SET LOCAL "awo.tenant_id" = '<uuid>';
-- before executing any query on a tenant-scoped connection.
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
    'Returns the tenant UUID set by the pgx driver for the current transaction. '
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
