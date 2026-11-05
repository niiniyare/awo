package pgx_test

// Part 7 of the "shared SDUI record-management" mission: validate that
// LinkLabelResolver cannot be used to bypass tenant isolation. The resolver
// queries a linked entity's own table (e.g. iam_user) to find a display
// label — that is still a read of real, RLS-protected data, and an earlier
// version of LinkResolver.ResolveLinkLabels queried the bare *pgxpool.Pool
// directly, with a comment merely ASSERTING that RLS was "already active on
// the connection" for the request. That was never actually verified.
//
// This test proves, against the real non-superuser awo_app role (not the
// admin superuser the live demo happens to run as, which bypasses RLS
// entirely and would never have caught this), that:
//  1. resolving a same-tenant user's label works correctly, and
//  2. resolving a different tenant's user id — even though its exact value
//     is known to the caller — returns nothing. No label, no row, no leak.
//
// Both together are the actual "tenant scope cannot be bypassed" guarantee
// Part 7 asks for: it is not merely that cross-tenant data is absent, but
// that same-tenant resolution still genuinely works under RLS (ruling out
// "fails closed for everyone, including legitimately scoped callers," which
// would hide the bug as "no leak" while silently breaking the feature).

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contribpgx "awo.so/awo/contrib/pgx"
	testdb "awo.so/awo/testutil/db"
)

func TestLinkResolver_PG_TenantIsolation_CannotResolveAcrossTenants(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)

	userSQL, err := os.ReadFile(filepath.Join("..", "..", "platform", "iam", "migrations", "001_iam_user.up.sql"))
	require.NoError(t, err)
	testdb.ApplySQL(t, pool, string(userSQL))
	testdb.ApplySQL(t, pool, `GRANT SELECT, INSERT, UPDATE ON iam_user TO `+testdb.AppRole+`;`)

	tenantA := testdb.CreateTenant(t, pool, "ACTIVE")
	tenantB := testdb.CreateTenant(t, pool, "ACTIVE")

	userA := uuid.New()
	userB := uuid.New()
	testdb.ApplySQL(t, pool, `INSERT INTO iam_user (id, tenant_id, email, full_name) VALUES ('`+
		userA.String()+`', '`+tenantA.String()+`', 'a@test.local', 'Tenant A User');`)
	testdb.ApplySQL(t, pool, `INSERT INTO iam_user (id, tenant_id, email, full_name) VALUES ('`+
		userB.String()+`', '`+tenantB.String()+`', 'b@test.local', 'Tenant B User');`)

	// Real non-superuser role from here on — ActivateTenant both sets the RLS
	// GUC for any immediate raw-SQL assertion AND switches the session role
	// to awo_app (SetupTestDB's pool_max_conns=1 pins the whole test to that
	// one connection, so this role switch is in effect for every query
	// below, including LinkResolver's own internal transaction).
	testdb.ActivateTenant(t, pool, tenantA)
	ctx := testdb.WithTenant(context.Background(), tenantA)

	resolver := contribpgx.NewLinkResolver(pool)

	t.Run("same-tenant id resolves to its real label", func(t *testing.T) {
		labels, err := resolver.ResolveLinkLabels(ctx, "iam_user", "full_name", []uuid.UUID{userA})
		require.NoError(t, err)
		assert.Equal(t, "Tenant A User", labels[userA])
	})

	t.Run("a different tenant's id resolves to nothing — no label, no leak", func(t *testing.T) {
		labels, err := resolver.ResolveLinkLabels(ctx, "iam_user", "full_name", []uuid.UUID{userB})
		require.NoError(t, err)
		_, found := labels[userB]
		assert.False(t, found, "resolving another tenant's user id must not return a label — RLS must block it, not just the raw row")
	})

	t.Run("a mixed batch resolves only the caller's own tenant's ids", func(t *testing.T) {
		labels, err := resolver.ResolveLinkLabels(ctx, "iam_user", "full_name", []uuid.UUID{userA, userB})
		require.NoError(t, err)
		assert.Equal(t, "Tenant A User", labels[userA])
		_, found := labels[userB]
		assert.False(t, found, "a batch call must not leak a cross-tenant id just because it rode along with a legitimate one")
	})
}

// TestLinkResolver_PG_NoTenantContext_FailsClosed matches
// Repository's own existing, already-tested "no ambient tenant context ->
// nothing visible, never everything visible" contract: a resolver call with
// no tenant.TenantContext in ctx at all (e.g. a bug elsewhere stripped it)
// must not silently fall back to an unscoped, see-everything query.
func TestLinkResolver_PG_NoTenantContext_FailsClosed(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)

	userSQL, err := os.ReadFile(filepath.Join("..", "..", "platform", "iam", "migrations", "001_iam_user.up.sql"))
	require.NoError(t, err)
	testdb.ApplySQL(t, pool, string(userSQL))
	testdb.ApplySQL(t, pool, `GRANT SELECT, INSERT, UPDATE ON iam_user TO `+testdb.AppRole+`;`)

	tenantA := testdb.CreateTenant(t, pool, "ACTIVE")
	userA := uuid.New()
	testdb.ApplySQL(t, pool, `INSERT INTO iam_user (id, tenant_id, email, full_name) VALUES ('`+
		userA.String()+`', '`+tenantA.String()+`', 'a@test.local', 'Tenant A User');`)

	testdb.ActivateTenant(t, pool, tenantA) // role switch only; ctx below carries no tenant

	resolver := contribpgx.NewLinkResolver(pool)
	labels, err := resolver.ResolveLinkLabels(context.Background(), "iam_user", "full_name", []uuid.UUID{userA})
	require.NoError(t, err)
	_, found := labels[userA]
	assert.False(t, found, "with no tenant context in ctx at all, nothing must resolve — not even a row that would otherwise be in scope")
}
