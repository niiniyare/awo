package pgx_test

// Proves that PostgreSQL connection pooling cannot leak tenant context
// between requests. testdb.SetupTestDB deliberately configures the test
// pool with pool_max_conns=1, so every WithTx call in these tests is
// guaranteed to reuse the exact same physical connection the previous call
// used — there is no other connection it could possibly get. If tenant
// context ever leaked across connection reuse, these tests would see it.
//
// These tests use testdb.InstallTenantLifecycle rather than the plain
// testdb.ActivateTenant path: InstallTenantLifecycle installs the real
// production set_tenant_context (transaction-local — set_config's third
// argument is true), which is the property under test. The simpler
// pass-through set_tenant_context that SetupTestDB installs by default sets
// the GUC at session level, which would not exercise (and could falsely
// pass) the exact reset-on-commit/rollback behavior this file verifies.

import (
	"context"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/driver"
	"awo.so/awo/runtime/tenant"
	testdb "awo.so/awo/testutil/db"
)

// setupPoolTest returns a pool with pool_max_conns=1 and the real
// (transaction-local) set_tenant_context installed. It deliberately does
// NOT switch to AppRole — testdb.CreateTenant needs superuser/owner
// privileges to write platform_tenant and internally calls testdb.ResetRole,
// so any role switch done before creating tenants would be silently undone.
// Call switchToAppRole after all tenant setup is complete, immediately
// before exercising WithTx — this was itself the exact bug the first
// version of this test file had: it looked correct, ran on a genuinely
// single-connection pool, and still silently ran every "tenant-scoped"
// transaction as the PostgreSQL superuser, which unconditionally bypasses
// RLS regardless of FORCE ROW LEVEL SECURITY. That produced a false-positive
// failure indistinguishable from a real cross-tenant leak, which is exactly
// why the role must be asserted, not merely set-and-assumed, right at the
// point each test relies on it.
func setupPoolTest(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)
	testdb.ApplySQL(t, pool, testEntityDDL)
	return pool
}

// switchToAppRole switches pool's current session to the non-superuser
// AppRole and asserts it actually took effect — RLS tests must never
// silently run as the PostgreSQL superuser, which bypasses RLS entirely
// regardless of FORCE ROW LEVEL SECURITY.
func switchToAppRole(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), "SET ROLE "+testdb.AppRole)
	require.NoError(t, err)

	var current string
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT current_user").Scan(&current))
	require.Equal(t, testdb.AppRole, current,
		"the session must actually be running as AppRole for this test's RLS assertions to mean anything")
}

func TestConnectionPool_CommitDoesNotLeakTenantContext(t *testing.T) {
	pool := setupPoolTest(t)
	repo := newRepo(pool)

	tenantA := testdb.CreateTenant(t, pool, "ACTIVE")
	tenantB := testdb.CreateTenant(t, pool, "ACTIVE")
	switchToAppRole(t, pool) // after tenant setup — CreateTenant needs superuser and resets the role

	ctxA := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantA})
	require.NoError(t, repo.WithTx(ctxA, func(txCtx context.Context) error {
		_, err := repo.Create(txCtx, driver.CreateInput{Data: map[string]any{"name": "A-secret"}})
		return err
	}), "Tenant A's transaction must commit successfully")

	// Tenant B's WithTx call below MUST reuse the exact same physical
	// connection Tenant A's transaction just committed on — pool_max_conns=1
	// guarantees there is no other connection it could acquire instead.
	ctxB := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantB})
	var seenByB []map[string]any
	require.NoError(t, repo.WithTx(ctxB, func(txCtx context.Context) error {
		records, _, err := repo.Query(txCtx, nil, driver.WithSkipCount())
		for _, r := range records {
			seenByB = append(seenByB, r.Data)
		}
		return err
	}))

	assert.Empty(t, seenByB, "Tenant B must not see Tenant A's committed row through a reused connection — "+
		"this is only safe if set_tenant_context's GUC is transaction-local and actually resets on commit")
}

func TestConnectionPool_RollbackDoesNotLeakTenantContextOrData(t *testing.T) {
	pool := setupPoolTest(t)
	repo := newRepo(pool)

	tenantA := testdb.CreateTenant(t, pool, "ACTIVE")
	tenantB := testdb.CreateTenant(t, pool, "ACTIVE")
	switchToAppRole(t, pool)

	ctxA := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantA})
	rollbackErr := repo.WithTx(ctxA, func(txCtx context.Context) error {
		if _, err := repo.Create(txCtx, driver.CreateInput{Data: map[string]any{"name": "A-abandoned"}}); err != nil {
			return err
		}
		// Returning an error forces WithTx to roll back the transaction
		// (contrib/pgx/repo.go's WithTx calls pgxTx.Rollback on any non-nil
		// error from fn), simulating a failed request.
		return assert.AnError
	})
	require.Error(t, rollbackErr, "the deliberate error must propagate")

	ctxB := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantB})
	var seenByB []map[string]any
	require.NoError(t, repo.WithTx(ctxB, func(txCtx context.Context) error {
		records, _, err := repo.Query(txCtx, nil, driver.WithSkipCount())
		for _, r := range records {
			seenByB = append(seenByB, r.Data)
		}
		return err
	}))

	assert.Empty(t, seenByB, "a rolled-back row must not exist, and the reused connection's tenant "+
		"context from the rolled-back transaction must not leak into Tenant B's transaction either")
}

func TestConnectionPool_FailedTenantContextSetup_DoesNotPoisonConnection(t *testing.T) {
	pool := setupPoolTest(t)
	repo := newRepo(pool)

	suspended := testdb.CreateTenant(t, pool, "SUSPENDED")
	active := testdb.CreateTenant(t, pool, "ACTIVE")
	switchToAppRole(t, pool)

	// This WithTx call must fail — set_tenant_context rejects the suspended
	// tenant — and, critically, must not leave the single pooled connection
	// in a broken or role-confused state for the next caller.
	ctxSuspended := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: suspended})
	err := repo.WithTx(ctxSuspended, func(txCtx context.Context) error {
		_, err := repo.Create(txCtx, driver.CreateInput{Data: map[string]any{"name": "should-not-exist"}})
		return err
	})
	require.Error(t, err, "a suspended tenant's transaction must be rejected")

	// The same pool (same single connection) must still work correctly for
	// a subsequent, legitimate request.
	ctxActive := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: active})
	require.NoError(t, repo.WithTx(ctxActive, func(txCtx context.Context) error {
		_, err := repo.Create(txCtx, driver.CreateInput{Data: map[string]any{"name": "B-fine"}})
		return err
	}), "a failed tenant-context setup on the shared connection must not poison it for the next tenant")

	var seen []map[string]any
	require.NoError(t, repo.WithTx(ctxActive, func(txCtx context.Context) error {
		records, _, err := repo.Query(txCtx, nil, driver.WithSkipCount())
		for _, r := range records {
			seen = append(seen, r.Data)
		}
		return err
	}))
	require.Len(t, seen, 1, "exactly the one legitimate record must exist — the rejected attempt wrote nothing")
	assert.Equal(t, "B-fine", seen[0]["name"])
}

func TestConnectionPool_NestedWithTx_ReusesSameTransaction_NoSecondAcquire(t *testing.T) {
	pool := setupPoolTest(t)
	repo := newRepo(pool)

	tenantA := testdb.CreateTenant(t, pool, "ACTIVE")
	switchToAppRole(t, pool)
	ctxA := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantA})

	// A WithTx call issued from inside another WithTx call must detect the
	// already-open transaction (contrib/pgx/repo.go: "existing.InTx()") and
	// run directly on it rather than trying to acquire a second connection —
	// which would deadlock outright against a pool_max_conns=1 pool. This
	// is exactly the scenario that repo.go's InTx short-circuit exists for.
	err := repo.WithTx(ctxA, func(outerCtx context.Context) error {
		if _, err := repo.Create(outerCtx, driver.CreateInput{Data: map[string]any{"name": "outer"}}); err != nil {
			return err
		}
		return repo.WithTx(outerCtx, func(innerCtx context.Context) error {
			_, err := repo.Create(innerCtx, driver.CreateInput{Data: map[string]any{"name": "inner"}})
			return err
		})
	})
	require.NoError(t, err, "a nested WithTx call must not deadlock or fail against a single-connection pool")

	var seen []map[string]any
	require.NoError(t, repo.WithTx(ctxA, func(txCtx context.Context) error {
		records, _, err := repo.Query(txCtx, nil, driver.WithSkipCount())
		for _, r := range records {
			seen = append(seen, r.Data)
		}
		return err
	}))
	assert.Len(t, seen, 2, "both the outer and nested inserts must have committed as one transaction")
}

// TestConnectionPool_ConcurrentTenants_TrulyIsolated uses a real
// multi-connection pool (unlike the single-connection tests above) to prove
// isolation holds under genuine concurrency, not just sequential reuse —
// two tenants' transactions running on two different physical connections
// at the same instant must never observe each other's data.
func TestConnectionPool_ConcurrentTenants_TrulyIsolated(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)
	testdb.ApplySQL(t, pool, testEntityDDL)

	// A second, unbounded-concurrency pool against the same isolated test
	// schema/database — SetupTestDB's own pool is pinned to
	// pool_max_conns=1, which would serialize this test and defeat the
	// point of testing concurrent connections.
	concurrentPool := testdb.OpenConcurrentPool(t, pool)

	const tenantCount = 8
	var wg sync.WaitGroup
	results := make([]bool, tenantCount) // true = isolation held for this tenant
	for i := 0; i < tenantCount; i++ {
		idx := i
		tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
		repo := newRepo(concurrentPool)
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})
			ownRow := map[string]any{"name": "own"}
			err := repo.WithTx(ctx, func(txCtx context.Context) error {
				if _, err := repo.Create(txCtx, driver.CreateInput{Data: ownRow}); err != nil {
					return err
				}
				records, _, err := repo.Query(txCtx, nil, driver.WithSkipCount())
				if err != nil {
					return err
				}
				results[idx] = len(records) == 1 && records[0].Data["name"] == "own"
				return nil
			})
			if err != nil {
				results[idx] = false
			}
		}()
	}
	wg.Wait()

	for i, ok := range results {
		assert.True(t, ok, "tenant goroutine %d must see exactly its own single row, never another tenant's", i)
	}
}
