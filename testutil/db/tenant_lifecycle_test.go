package db_test

// Regression coverage for tenant ACTIVE-status enforcement at the RLS layer.
//
// These tests exercise set_tenant_context() directly — no HTTP layer, no
// TenantResolver middleware, no AuthService — to prove the database itself
// is the enforcement boundary, exactly as docs/04-multitenancy/RLS_SPEC.md
// and TENANT_LIFECYCLE.md specify. Any caller that reaches the database
// (a background job, a Temporal activity, the CLI, a future internal RPC,
// or a bug that skips HTTP middleware) inherits this guarantee for free.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testdb "awo.so/awo/testutil/db"
)

func TestTenantLifecycle_ACTIVE_Allowed(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)

	id := testdb.CreateTenant(t, pool, "ACTIVE")

	err := testdb.TryActivateTenant(pool, id)
	assert.NoError(t, err, "an ACTIVE tenant must be accepted by set_tenant_context")
}

func TestTenantLifecycle_PENDING_Denied(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)

	id := testdb.CreateTenant(t, pool, "PENDING")

	err := testdb.TryActivateTenant(pool, id)
	require.Error(t, err, "a PENDING tenant must be rejected by set_tenant_context")
	assert.Contains(t, err.Error(), "tenant_not_active")
}

func TestTenantLifecycle_SUSPENDED_Denied(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)

	id := testdb.CreateTenant(t, pool, "SUSPENDED")

	err := testdb.TryActivateTenant(pool, id)
	require.Error(t, err, "a SUSPENDED tenant must be rejected by set_tenant_context — "+
		"a suspended tenant must not be able to establish RLS context via any "+
		"code path, not just HTTP login")
	assert.Contains(t, err.Error(), "tenant_not_active")
}

func TestTenantLifecycle_ARCHIVED_Denied(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)

	id := testdb.CreateTenant(t, pool, "ARCHIVED")

	err := testdb.TryActivateTenant(pool, id)
	require.Error(t, err, "an ARCHIVED tenant must be rejected by set_tenant_context")
	assert.Contains(t, err.Error(), "tenant_not_active")
}

func TestTenantLifecycle_NonexistentTenant_Denied(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)

	// A random UUID with no backing platform_tenant row at all — this is
	// exactly what a forged or stale X-Tenant-ID header, or a bug passing an
	// uninitialized uuid.UUID zero value's sibling, would produce.
	err := testdb.TryActivateTenant(pool, testdb.RawTenantID())
	require.Error(t, err, "a nonexistent tenant ID must be rejected, not silently accepted")
	assert.Contains(t, err.Error(), "tenant_not_found")
}

// TestTenantLifecycle_DirectRepositoryAccess_NoHTTPMiddleware proves the
// enforcement holds for a caller that never goes through TenantResolver or
// any HTTP middleware at all — simulating a background job, a Temporal
// activity, or a CLI command that calls set_tenant_context directly via a
// raw connection, exactly like contrib/pgx.Repository.WithTx does
// internally. Without this, the database layer would provide zero
// defense-in-depth for any caller that isn't an HTTP request.
func TestTenantLifecycle_DirectRepositoryAccess_NoHTTPMiddleware(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)

	suspended := testdb.CreateTenant(t, pool, "SUSPENDED")
	active := testdb.CreateTenant(t, pool, "ACTIVE")

	// No TenantResolver, no RequireAuth, no AuthService — just the raw
	// mechanism every WithTx call uses internally.
	require.Error(t, testdb.TryActivateTenant(pool, suspended),
		"direct DB access for a suspended tenant must be rejected with no HTTP layer involved")
	require.NoError(t, testdb.TryActivateTenant(pool, active),
		"direct DB access for an active tenant must succeed")
}
