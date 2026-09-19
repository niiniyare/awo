package pgx_test

// Verifies the two-layer defense a background job gets if it ever forgets to
// attach a tenant.TenantContext before touching a tenant-scoped repository —
// the scenario every currently-unwired async subsystem (outbox subscribers,
// Temporal activities, the cron scheduler) would hit on day one of being
// wired up, since none of them attach one today (confirmed by direct source
// review: none has any production caller yet).
//
// Layer 1 (Go, write path): Repository.Create/BulkCreate call
// tenant.FromContext (not TryFromContext) and panic immediately if no
// TenantContext is present — fail-fast, not a silent wrong-tenant write.
//
// Layer 2 (SQL, read path): Repository.WithTx only calls set_tenant_context
// when a TenantContext is present in ctx (tenant.TryFromContext) — with none,
// it silently skips that call rather than erroring. But the connection is
// then left with current_tenant_id() = NULL for that transaction, and every
// tenant-scoped RLS policy is "tenant_id = current_tenant_id()", which SQL
// never evaluates true for NULL — so reads on tenant-scoped tables return
// zero rows instead of every tenant's rows. The Go-level skip is fail-open by
// itself, but the net behavior is fail-closed because of how the RLS policy
// is written. This test exists to pin that combined behavior down with a
// real assertion instead of leaving it as an inference from reading the code.
import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/driver"
	"awo.so/awo/runtime/tenant"
	testdb "awo.so/awo/testutil/db"
)

func TestRepository_Create_NoTenantContext_PanicsRatherThanWritingUnscoped(t *testing.T) {
	pool := setupPoolTest(t)
	repo := newRepo(pool)

	active := testdb.CreateTenant(t, pool, "ACTIVE")
	switchToAppRole(t, pool)

	// Seed one legitimate row so a would-be leak has something to leak.
	ctxActive := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: active})
	require.NoError(t, repo.WithTx(ctxActive, func(txCtx context.Context) error {
		_, err := repo.Create(txCtx, driver.CreateInput{Data: map[string]any{"name": "real-tenant-row"}})
		return err
	}))

	// A background job that forgot to attach a TenantContext at all —
	// exactly what every currently-unwired async subsystem does today.
	//
	// Called directly, deliberately NOT through WithTx: Create's very first
	// line is tenant.FromContext(ctx), before any connection/transaction is
	// acquired, so this panics before touching the pool at all. Triggering
	// the panic from inside a live WithTx callback instead would abandon that
	// transaction's connection without releasing it back to the pool — fatal
	// for repeat runs against this package's pool_max_conns=1 test pools.
	bareCtx := context.Background()
	assert.Panics(t, func() {
		_, _ = repo.Create(bareCtx, driver.CreateInput{Data: map[string]any{"name": "should-never-be-written"}})
	}, "Repository.Create must panic (tenant.FromContext) rather than silently create an unscoped or wrong-tenant row")
}

func TestRepository_Query_NoTenantContext_SeesNothing_NotEveryTenant(t *testing.T) {
	pool := setupPoolTest(t)
	repo := newRepo(pool)

	tenantA := testdb.CreateTenant(t, pool, "ACTIVE")
	tenantB := testdb.CreateTenant(t, pool, "ACTIVE")
	switchToAppRole(t, pool)

	ctxA := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantA})
	require.NoError(t, repo.WithTx(ctxA, func(txCtx context.Context) error {
		_, err := repo.Create(txCtx, driver.CreateInput{Data: map[string]any{"name": "A-row"}})
		return err
	}))
	ctxB := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantB})
	require.NoError(t, repo.WithTx(ctxB, func(txCtx context.Context) error {
		_, err := repo.Create(txCtx, driver.CreateInput{Data: map[string]any{"name": "B-row"}})
		return err
	}))

	// A background job that forgot to attach a TenantContext at all. WithTx
	// silently skips set_tenant_context (tenant.TryFromContext returns
	// false), but current_tenant_id() then reads NULL for this transaction,
	// and "tenant_id = NULL" is never true — so this must see zero rows, not
	// both tenants' rows and not a panic on the read path (Query never calls
	// tenant.FromContext; it relies entirely on the RLS policy).
	bareCtx := context.Background()
	var seen []map[string]any
	require.NoError(t, repo.WithTx(bareCtx, func(txCtx context.Context) error {
		records, _, err := repo.Query(txCtx, nil, driver.WithSkipCount())
		for _, r := range records {
			seen = append(seen, r.Data)
		}
		return err
	}))
	assert.Empty(t, seen, "a background job with no tenant context must see zero rows on a tenant-scoped table — "+
		"never every tenant's data, and this must hold even though the Go-level WithTx skip is fail-open by itself")
}
