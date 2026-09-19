package pgx_test

// Platform-admin / SystemContext RLS boundary.
//
// There is no RLS bypass for platform admins anywhere in the codebase: the
// only privilege elevation "platform admin" carries is a Casbin (permission)
// bypass in api/authz.RequirePermission — the viewer's tenant_id is untouched
// and still flows through the normal RLS path. Confirmed by direct source
// review of every CREATE POLICY in the repository (all use plain
// tenant_id/org_id equality, no admin-flag branch) and of
// api/authz/authz.go (the bypass short-circuits Casbin only, still logs the
// viewer's own tenant_id, and still requires a session that went through
// normal tenant resolution).
//
// The one thing that looked like an intentional cross-tenant escape hatch —
// runtime/tenant.SystemContext(), a sentinel TenantContext with
// TenantID == uuid.Nil — is proven here to not work as a bypass either: the
// real set_tenant_context(uuid) (migration/bootstrap/002_utilities.up.sql),
// which contrib/pgx.Repository.WithTx calls for any TenantContext it finds in
// ctx, requires a matching platform_tenant row. uuid.Nil never has one, so
// the call fails closed with "tenant_not_found" instead of proceeding with
// no tenant filter. (Also: nothing in the codebase currently calls
// SystemContext at all — this test exists so that if something starts to,
// its assumed semantics are pinned down by a real test before that happens.)
import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testdb "awo.so/awo/testutil/db"
)

// TestSystemContext_NilTenantID_RejectedNotBypassed proves that driving a
// repository operation with runtime/tenant.SystemContext's sentinel
// TenantID (uuid.Nil) does not grant cross-tenant/platform-wide access: the
// real set_tenant_context() rejects it outright (P0001 tenant_not_found)
// because no platform_tenant row has id = uuid.Nil.
func TestSystemContext_NilTenantID_RejectedNotBypassed(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)

	err := testdb.TryActivateTenant(pool, uuid.Nil)
	require.Error(t, err, "set_tenant_context(uuid.Nil) must be rejected, not silently accepted as a platform-wide context")

	var pgErr *pgconn.PgError
	if assert.ErrorAs(t, err, &pgErr, "expected a real PostgreSQL error from set_tenant_context") {
		assert.Equal(t, "P0001", pgErr.Code, "expected tenant_not_found (P0001), got SQLSTATE %s: %s", pgErr.Code, pgErr.Message)
	}
}
