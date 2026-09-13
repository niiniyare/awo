-- Migration: bootstrap — create RLS helper function.
-- This migration must run first (lowest sequence number).

-- set_tenant_context sets the transaction-local RLS variable.
-- Called by the app before any DML on tenant-scoped tables.
-- TRUE makes it transaction-local — resets automatically on COMMIT/ROLLBACK.
CREATE OR REPLACE FUNCTION set_tenant_context(p_tenant_id UUID)
RETURNS VOID
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
BEGIN
    PERFORM set_config('app.current_tenant_id', p_tenant_id::text, TRUE);
END;
$$;

-- current_tenant_id reads the transaction-local tenant ID.
-- Used in RLS policy USING expressions.
CREATE OR REPLACE FUNCTION current_tenant_id()
RETURNS UUID
LANGUAGE plpgsql
STABLE
AS $$
BEGIN
    RETURN current_setting('app.current_tenant_id', TRUE)::UUID;
EXCEPTION
    WHEN OTHERS THEN
        RETURN NULL;
END;
$$;
