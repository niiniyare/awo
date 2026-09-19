package pgx_test

// Repository.WithTx panic-safety.
//
// WithTx's transaction lifecycle is: BeginTx (acquires a physical connection
// from the pool) -> optional set_tenant_context -> fn(txCtx) -> Commit or
// Rollback. Only Commit/Rollback release the connection back to the pool
// (pgxpool's Tx wraps Release internally). Before the fix, if fn panicked —
// for example via Repository.Create's intentional
// "tenant.FromContext(ctx) // panics if no TenantContext" — neither Commit
// nor Rollback would ever run, and the connection would stay checked out
// forever: invisible to the pool, and (against the pool_max_conns=1 test
// pools used throughout this package) fatal to every subsequent test or
// request sharing that pool. The fix wraps the transaction body in a defer
// that rolls back unless a commit already happened, and deliberately does
// NOT call recover() — so cleanup always runs, but the panic itself still
// propagates to the caller exactly as an unhandled panic normally would.
//
// These tests use a real PostgreSQL pool_max_conns=1 pool (the same
// discipline as connection_pool_test.go) specifically so that "the
// connection was released" can be proven by literally reusing it for a
// second, unrelated tenant afterward — there is no other connection for
// that second call to have used if it succeeds.
import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/driver"
	"awo.so/awo/runtime/tenant"
	testdb "awo.so/awo/testutil/db"
)

// runAndRecover calls fn and reports whether it panicked, without letting
// the panic escape the test goroutine before the assertion runs.
func runAndRecover(fn func()) (panicked bool, value any) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true
			value = r
		}
	}()
	fn()
	return false, nil
}

func TestWithTx_PanicBeforeAnyMutation_RollsBackAndReleasesConnection(t *testing.T) {
	pool := setupPoolTest(t)
	repo := newRepo(pool)

	tenantA := testdb.CreateTenant(t, pool, "ACTIVE")
	tenantB := testdb.CreateTenant(t, pool, "ACTIVE")
	switchToAppRole(t, pool)

	ctxA := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantA})
	panicked, value := runAndRecover(func() {
		_ = repo.WithTx(ctxA, func(txCtx context.Context) error {
			panic("simulated panic before any DB call")
		})
	})
	require.True(t, panicked, "the panic must propagate out of WithTx, not be swallowed")
	assert.Equal(t, "simulated panic before any DB call", value)

	// The single pooled connection must still be usable for a different
	// tenant — this only succeeds if Rollback actually ran and released it.
	ctxB := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantB})
	require.NoError(t, repo.WithTx(ctxB, func(txCtx context.Context) error {
		_, err := repo.Create(txCtx, driver.CreateInput{Data: map[string]any{"name": "B-after-panic"}})
		return err
	}), "the connection must be reusable by a completely different tenant after a panic in the previous caller")
}

func TestWithTx_PanicAfterMutation_RollsBackTheMutationToo(t *testing.T) {
	pool := setupPoolTest(t)
	repo := newRepo(pool)

	tenantA := testdb.CreateTenant(t, pool, "ACTIVE")
	tenantB := testdb.CreateTenant(t, pool, "ACTIVE")
	switchToAppRole(t, pool)

	ctxA := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantA})
	panicked, _ := runAndRecover(func() {
		_ = repo.WithTx(ctxA, func(txCtx context.Context) error {
			_, err := repo.Create(txCtx, driver.CreateInput{Data: map[string]any{"name": "should-be-rolled-back"}})
			if err != nil {
				t.Fatalf("setup Create failed: %v", err)
			}
			panic("simulated panic after a mutation")
		})
	})
	require.True(t, panicked)

	// Re-activate as Tenant A on the (now-released) connection and confirm
	// the mutation from the panicked transaction was never committed.
	ctxA2 := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantA})
	var seen []map[string]any
	require.NoError(t, repo.WithTx(ctxA2, func(txCtx context.Context) error {
		records, _, err := repo.Query(txCtx, nil, driver.WithSkipCount())
		for _, r := range records {
			seen = append(seen, r.Data)
		}
		return err
	}))
	assert.Empty(t, seen, "the row created immediately before the panic must not have been committed")

	ctxB := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantB})
	require.NoError(t, repo.WithTx(ctxB, func(txCtx context.Context) error {
		_, err := repo.Create(txCtx, driver.CreateInput{Data: map[string]any{"name": "B-fine"}})
		return err
	}), "the connection must still be usable by another tenant afterward")
}

// TestWithTx_PanicInNestedCall_CaughtByOutermostRollback proves that a panic
// inside a nested WithTx call (the existing.InTx() short-circuit path, which
// does no transaction management of its own) is still caught by the
// outermost WithTx's defer — the one that actually owns the connection.
func TestWithTx_PanicInNestedCall_CaughtByOutermostRollback(t *testing.T) {
	pool := setupPoolTest(t)
	repo := newRepo(pool)

	tenantA := testdb.CreateTenant(t, pool, "ACTIVE")
	tenantB := testdb.CreateTenant(t, pool, "ACTIVE")
	switchToAppRole(t, pool)

	ctxA := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantA})
	panicked, _ := runAndRecover(func() {
		_ = repo.WithTx(ctxA, func(outerCtx context.Context) error {
			_, err := repo.Create(outerCtx, driver.CreateInput{Data: map[string]any{"name": "outer"}})
			if err != nil {
				t.Fatalf("setup Create failed: %v", err)
			}
			return repo.WithTx(outerCtx, func(innerCtx context.Context) error {
				panic("simulated panic inside the nested call")
			})
		})
	})
	require.True(t, panicked, "the panic must still propagate through the nested call and out of the outer WithTx")

	ctxB := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantB})
	var seen []map[string]any
	require.NoError(t, repo.WithTx(ctxB, func(txCtx context.Context) error {
		records, _, err := repo.Query(txCtx, nil, driver.WithSkipCount())
		for _, r := range records {
			seen = append(seen, r.Data)
		}
		return err
	}), "the connection must be reusable afterward")
	assert.Empty(t, seen, "Tenant B must see none of Tenant A's data — including the outer 'outer' row, "+
		"which must have been rolled back along with everything else in that transaction")
}

func TestWithTx_NoPanic_StillCommitsNormally(t *testing.T) {
	// Sanity check that adding the defer/committed-flag machinery did not
	// change ordinary (non-panicking) commit behavior.
	pool := setupPoolTest(t)
	repo := newRepo(pool)

	tenantA := testdb.CreateTenant(t, pool, "ACTIVE")
	switchToAppRole(t, pool)

	ctxA := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantA})
	require.NoError(t, repo.WithTx(ctxA, func(txCtx context.Context) error {
		_, err := repo.Create(txCtx, driver.CreateInput{Data: map[string]any{"name": "committed-normally"}})
		return err
	}))

	var seen []map[string]any
	require.NoError(t, repo.WithTx(ctxA, func(txCtx context.Context) error {
		records, _, err := repo.Query(txCtx, nil, driver.WithSkipCount())
		for _, r := range records {
			seen = append(seen, r.Data)
		}
		return err
	}))
	require.Len(t, seen, 1)
	assert.Equal(t, "committed-normally", seen[0]["name"])
}
