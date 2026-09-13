package db_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contribpgx "awo.so/awo/contrib/pgx"
	"awo.so/awo/runtime/tenant"
	"awo.so/awo/testutil/db"
)

// minimalTableSQL is DDL for a minimal tenant-scoped table used by RLS tests.
// Mirrors the pattern that the migration generator produces for SystemDefinition
// with ScopeTenant. GRANT gives AppRole the privileges needed for RLS testing.
const minimalTableSQL = `
CREATE TABLE test_items (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid NOT NULL,
    name        text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE test_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE test_items FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON test_items
    USING (tenant_id = current_tenant_id());

-- AppRole needs DML privileges so ActivateTenant (which switches to AppRole)
-- can insert AND select with RLS enforced.
GRANT SELECT, INSERT, UPDATE, DELETE ON test_items TO awo_app;
`

// TestRLSIsolation verifies that Tenant A cannot read Tenant B's rows,
// and vice versa, purely through RLS — no application-level PolicyFunc.
func TestRLSIsolation(t *testing.T) {
	pool := db.SetupTestDB(t)

	// Apply a minimal tenant-scoped table.
	db.ApplySQL(t, pool, minimalTableSQL)

	tenantA := db.RawTenantID()
	tenantB := db.RawTenantID()

	// Insert one row per tenant using the pool querier with correct tenant context.
	insertRow := func(t *testing.T, tenantID uuid.UUID, name string) {
		t.Helper()
		db.ActivateTenant(t, pool, tenantID)
		q := contribpgx.NewPoolQuerier(pool)
		_, err := q.ExecSQL(context.Background(),
			`INSERT INTO test_items (tenant_id, name) VALUES ($1, $2)`,
			tenantID, name)
		require.NoError(t, err)
	}

	insertRow(t, tenantA, "item-A")
	insertRow(t, tenantB, "item-B")

	// As Tenant A: must see exactly 1 row (own row only).
	t.Run("TenantA sees own rows only", func(t *testing.T) {
		db.ActivateTenant(t, pool, tenantA)
		n := db.RowCount(t, pool, "test_items")
		assert.Equal(t, 1, n, "Tenant A should see exactly 1 row")
	})

	// As Tenant B: must see exactly 1 row (own row only).
	t.Run("TenantB sees own rows only", func(t *testing.T) {
		db.ActivateTenant(t, pool, tenantB)
		n := db.RowCount(t, pool, "test_items")
		assert.Equal(t, 1, n, "Tenant B should see exactly 1 row")
	})

	// Without any tenant context: must see 0 rows (NULL uuid matches nothing).
	t.Run("no tenant context sees zero rows", func(t *testing.T) {
		// Clear tenant context by setting empty string — current_tenant_id() → NULL.
		q := contribpgx.NewPoolQuerier(pool)
		_, err := q.ExecSQL(context.Background(), "SELECT set_tenant_context('')")
		require.NoError(t, err)
		n := db.RowCount(t, pool, "test_items")
		assert.Equal(t, 0, n, "no tenant context should see 0 rows")
	})
}

// TestRLSDefenseInDepth verifies that RLS alone is sufficient —
// even without the application-layer context, the DB policy blocks cross-tenant reads.
// This mirrors the architectural invariant: PolicyFunc is optional, RLS is mandatory.
func TestRLSDefenseInDepth(t *testing.T) {
	pool := db.SetupTestDB(t)
	db.ApplySQL(t, pool, minimalTableSQL)

	tenantA := db.RawTenantID()
	tenantB := db.RawTenantID()

	// Insert rows for both tenants.
	for _, tid := range []uuid.UUID{tenantA, tenantB} {
		db.ActivateTenant(t, pool, tid)
		q := contribpgx.NewPoolQuerier(pool)
		_, err := q.ExecSQL(context.Background(),
			`INSERT INTO test_items (tenant_id, name) VALUES ($1, $2)`,
			tid, "item-"+tid.String())
		require.NoError(t, err)
	}

	// Activate tenant A, query WITHOUT any application context key.
	// The RLS policy is the only gate.
	db.ActivateTenant(t, pool, tenantA)
	n := db.RowCount(t, pool, "test_items")
	assert.Equal(t, 1, n, "RLS alone should restrict to Tenant A's row")

	// Activate tenant B and verify the same.
	db.ActivateTenant(t, pool, tenantB)
	n = db.RowCount(t, pool, "test_items")
	assert.Equal(t, 1, n, "RLS alone should restrict to Tenant B's row")
}

// TestTableExistsHelper verifies the test helper itself is correct.
func TestTableExistsHelper(t *testing.T) {
	pool := db.SetupTestDB(t)
	assert.False(t, db.TableExists(t, pool, "test_items"), "table should not exist yet")
	db.ApplySQL(t, pool, minimalTableSQL)
	assert.True(t, db.TableExists(t, pool, "test_items"), "table should exist after ApplySQL")
}

// TestWithTenantContext verifies db.WithTenant produces a context using
// the framework's tenant.WithContext under the hood (not a private key).
func TestWithTenantContext(t *testing.T) {
	tenantID := db.RawTenantID()
	ctx := db.WithTenant(context.Background(), tenantID)

	// Verify round-trip through the framework's tenant package.
	// If db.WithTenant used a different context key this would panic.
	tc := tenant.FromContext(ctx)
	assert.Equal(t, tenantID, tc.TenantID)
}
