package pgx_test

// rls_defense_test.go verifies that malformed, adversarial, or pathological
// filter inputs cannot bypass PostgreSQL Row-Level Security policies.
//
// Tests in this file use a real PostgreSQL connection (TEST_DATABASE_URL env
// var). Each test creates its own isolated schema via testutil/db.SetupTestDB,
// which is dropped when the test ends.
//
// These tests complement the basic RLS isolation tests in testutil/db/rls_test.go
// by covering attack vectors at the Repository query layer:
//   - Cross-tenant predicate (Tenant B tries to read Tenant A's ID explicitly)
//   - OR predicate covering both tenant IDs
//   - Empty filter (relies on RLS alone)
//   - Contradictory predicate (always-false filter still obeys RLS)
//   - Count obeys RLS

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/driver"
	"awo.so/awo/filter"
	testdb "awo.so/awo/testutil/db"
)

// TestRLSDefense_CrossTenantIDPredicate verifies that Tenant B cannot read a
// specific record belonging to Tenant A even when supplying the exact UUID in
// the query filter. RLS filters out the row regardless of the app-layer filter.
func TestRLSDefense_CrossTenantIDPredicate(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, testEntityDDL)

	tenantA := testdb.RawTenantID()
	tenantB := testdb.RawTenantID()
	repo := newRepo(pool)

	// Tenant A creates a record.
	testdb.ActivateTenant(t, pool, tenantA)
	ctxA := testdb.WithTenant(context.Background(), tenantA)
	rec, err := repo.Create(ctxA, driver.CreateInput{Data: map[string]any{"name": "secret"}})
	require.NoError(t, err)

	// Tenant B attempts to query with Tenant A's record ID directly.
	testdb.ActivateTenant(t, pool, tenantB)
	ctxB := testdb.WithTenant(context.Background(), tenantB)

	rows, _, err := repo.Query(ctxB, filter.Eq("id", rec.ID), driver.WithSkipCount())
	require.NoError(t, err)
	assert.Empty(t, rows, "RLS must block Tenant B from reading Tenant A's record by ID")
}

// TestRLSDefense_GetCrossTenant verifies that repo.Get by a cross-tenant ID
// returns a not-found error, not the record.
func TestRLSDefense_GetCrossTenant(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, testEntityDDL)

	tenantA := testdb.RawTenantID()
	tenantB := testdb.RawTenantID()
	repo := newRepo(pool)

	testdb.ActivateTenant(t, pool, tenantA)
	ctxA := testdb.WithTenant(context.Background(), tenantA)
	rec, err := repo.Create(ctxA, driver.CreateInput{Data: map[string]any{"name": "secret"}})
	require.NoError(t, err)

	testdb.ActivateTenant(t, pool, tenantB)
	ctxB := testdb.WithTenant(context.Background(), tenantB)

	_, err = repo.Get(ctxB, rec.ID)
	require.Error(t, err, "cross-tenant Get must return an error")
}

// TestRLSDefense_ORPredicateCrossTenant verifies that an OR predicate covering
// both tenant IDs cannot surface Tenant A's rows when activated as Tenant B.
func TestRLSDefense_ORPredicateCrossTenant(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, testEntityDDL)

	tenantA := testdb.RawTenantID()
	tenantB := testdb.RawTenantID()
	repo := newRepo(pool)

	testdb.ActivateTenant(t, pool, tenantA)
	ctxA := testdb.WithTenant(context.Background(), tenantA)
	_, err := repo.Create(ctxA, driver.CreateInput{Data: map[string]any{"name": "A-secret"}})
	require.NoError(t, err)

	testdb.ActivateTenant(t, pool, tenantB)
	ctxB := testdb.WithTenant(context.Background(), tenantB)
	_, err = repo.Create(ctxB, driver.CreateInput{Data: map[string]any{"name": "B-row"}})
	require.NoError(t, err)

	// Adversarial OR filter naming both tenant IDs.
	f := filter.Or(
		filter.Eq("tenant_id", tenantA),
		filter.Eq("tenant_id", tenantB),
	)
	rows, _, err := repo.Query(ctxB, f, driver.WithSkipCount())
	require.NoError(t, err)
	// RLS restricts to current_tenant_id() = tenantB — Tenant A's row invisible.
	for _, row := range rows {
		assert.Equal(t, tenantB, row.TenantID,
			"OR predicate must not leak Tenant A rows to Tenant B session")
	}
}

// TestRLSDefense_EmptyFilter verifies that a nil filter relies entirely on RLS
// and returns only the current tenant's rows.
func TestRLSDefense_EmptyFilter(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, testEntityDDL)

	tenantA := testdb.RawTenantID()
	tenantB := testdb.RawTenantID()
	repo := newRepo(pool)

	// Tenant A: 3 rows.
	testdb.ActivateTenant(t, pool, tenantA)
	ctxA := testdb.WithTenant(context.Background(), tenantA)
	for i := range 3 {
		_, err := repo.Create(ctxA, driver.CreateInput{
			Data: map[string]any{"name": "a", "code": string(rune('A' + i))},
		})
		require.NoError(t, err)
	}

	// Tenant B: 1 row.
	testdb.ActivateTenant(t, pool, tenantB)
	ctxB := testdb.WithTenant(context.Background(), tenantB)
	_, err := repo.Create(ctxB, driver.CreateInput{Data: map[string]any{"name": "b"}})
	require.NoError(t, err)

	// Tenant B queries with no filter — RLS must restrict to 1 row.
	rows, _, err := repo.Query(ctxB, nil, driver.WithSkipCount())
	require.NoError(t, err)
	assert.Len(t, rows, 1, "empty filter must still be tenant-scoped via RLS")

	// Tenant A queries with no filter — must see exactly 3.
	testdb.ActivateTenant(t, pool, tenantA)
	rows, _, err = repo.Query(ctxA, nil, driver.WithSkipCount())
	require.NoError(t, err)
	assert.Len(t, rows, 3, "empty filter for Tenant A must return Tenant A's 3 rows")
}

// TestRLSDefense_ContradictoryPredicate verifies that an always-false predicate
// returns 0 rows without error and does not leak data from other tenants.
func TestRLSDefense_ContradictoryPredicate(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, testEntityDDL)

	tenantA := testdb.RawTenantID()
	repo := newRepo(pool)

	testdb.ActivateTenant(t, pool, tenantA)
	ctx := testdb.WithTenant(context.Background(), tenantA)

	_, err := repo.Create(ctx, driver.CreateInput{Data: map[string]any{"name": "real"}})
	require.NoError(t, err)

	// Contradictory filter: id = uuid.Nil (no such row exists).
	rows, _, err := repo.Query(ctx, filter.Eq("id", uuid.Nil), driver.WithSkipCount())
	require.NoError(t, err)
	assert.Empty(t, rows, "contradictory predicate must return 0 rows without error")
}

// TestRLSDefense_Count_TenantScoped verifies Count also obeys RLS and does
// not return cross-tenant row counts.
func TestRLSDefense_Count_TenantScoped(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, testEntityDDL)

	tenantA := testdb.RawTenantID()
	tenantB := testdb.RawTenantID()
	repo := newRepo(pool)

	testdb.ActivateTenant(t, pool, tenantA)
	ctxA := testdb.WithTenant(context.Background(), tenantA)
	for range 5 {
		_, err := repo.Create(ctxA, driver.CreateInput{Data: map[string]any{"name": "a"}})
		require.NoError(t, err)
	}

	testdb.ActivateTenant(t, pool, tenantB)
	ctxB := testdb.WithTenant(context.Background(), tenantB)

	n, err := repo.Count(ctxB, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(0), n, "Count for Tenant B must be 0 when all rows belong to Tenant A")
}
