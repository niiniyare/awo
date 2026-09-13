// Package db provides PostgreSQL integration test helpers.
//
// Uses existing framework packages (runtime/tenant, contrib/pgx) for all
// operations. Test isolation: a unique schema per test run, dropped on cleanup.
//
// # RLS enforcement
//
// PostgreSQL superusers bypass RLS even with FORCE ROW LEVEL SECURITY.
// SetupTestDB creates a non-superuser role [AppRole] ("awo_app") and switches
// to it so that RLS policies are enforced in tests. ActivateTenant calls
// set_tenant_context AND SET ROLE awo_app. Test DDL must GRANT necessary
// privileges on tables to AppRole.
//
// # Required environment variable
//
//	TEST_DATABASE_URL=postgres://user:pass@localhost:5432/awo_test?sslmode=disable
//
// If unset, any test calling SetupTestDB is automatically skipped.
//
// # Usage
//
//	func TestSomething(t *testing.T) {
//	    pool := db.SetupTestDB(t)
//	    db.ApplySQL(t, pool, myTableDDL)  // must GRANT ... TO awo_app
//	    tenantA := db.RawTenantID()
//	    db.ActivateTenant(t, pool, tenantA)  // sets tenant + role
//	    // insert + query; RLS enforced
//	}
package db

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	contribpgx "awo.so/awo/contrib/pgx"
	"awo.so/awo/runtime/tenant"
)

// AppRole is the non-superuser PostgreSQL role used in integration tests.
// RLS is enforced for this role. Test DDL must GRANT table privileges to it.
const AppRole = "awo_app"

// SetupTestDB creates an isolated test schema, installs framework RLS helpers,
// creates the AppRole (if absent), and returns a single-connection pool.
// The schema is dropped when the test ends via t.Cleanup.
func SetupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping PostgreSQL integration test")
	}

	ctx := context.Background()

	// Root pool: create isolated schema and ensure AppRole exists.
	rootPool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("db.SetupTestDB: connect: %v", err)
	}
	if err := rootPool.Ping(ctx); err != nil {
		rootPool.Close()
		t.Fatalf("db.SetupTestDB: ping: %v", err)
	}

	// Ensure AppRole exists (non-superuser, no login needed for SET ROLE).
	roleSQL := fmt.Sprintf(`
DO $$ BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '%s') THEN
    CREATE ROLE %s NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
  END IF;
END $$;
-- Allow the current user to SET ROLE to AppRole.
GRANT %s TO CURRENT_USER;
`, AppRole, AppRole, AppRole)
	if _, err := rootPool.Exec(ctx, roleSQL); err != nil {
		rootPool.Close()
		t.Fatalf("db.SetupTestDB: ensure role %q: %v", AppRole, err)
	}

	schemaName := "test_" + strings.ReplaceAll(uuid.New().String(), "-", "")
	if _, err := rootPool.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA %q AUTHORIZATION CURRENT_USER`, schemaName)); err != nil {
		rootPool.Close()
		t.Fatalf("db.SetupTestDB: create schema: %v", err)
	}
	// Grant schema usage to AppRole so it can see objects in it.
	if _, err := rootPool.Exec(ctx, fmt.Sprintf(`GRANT USAGE ON SCHEMA %q TO %s`, schemaName, AppRole)); err != nil {
		rootPool.Close()
		t.Fatalf("db.SetupTestDB: grant schema usage: %v", err)
	}
	rootPool.Close()

	// Re-connect with the isolated schema as search_path.
	// pool_max_conns=1: single connection so session SET ROLE and set_config
	// persist across consecutive ExecSQL/QueryRow calls in the same test.
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	pool, err := pgxpool.New(ctx, dsn+sep+"search_path="+schemaName+",public&pool_max_conns=1")
	if err != nil {
		t.Fatalf("db.SetupTestDB: schema pool: %v", err)
	}

	// Install framework RLS helper functions into the isolated schema.
	// is_local=false → session-level (persists across statements without transactions).
	q := contribpgx.NewPoolQuerier(pool)
	infraSQL := fmt.Sprintf(`
CREATE OR REPLACE FUNCTION %s.current_tenant_id() RETURNS uuid AS $$
  SELECT NULLIF(current_setting('awo.tenant_id', true), '')::uuid;
$$ LANGUAGE sql STABLE;

CREATE OR REPLACE FUNCTION %s.set_tenant_context(tenant_id text) RETURNS void AS $$
BEGIN
  PERFORM set_config('awo.tenant_id', tenant_id, false);
END;
$$ LANGUAGE plpgsql;

GRANT EXECUTE ON FUNCTION %s.current_tenant_id() TO %s;
GRANT EXECUTE ON FUNCTION %s.set_tenant_context(text) TO %s;
`, schemaName, schemaName, schemaName, AppRole, schemaName, AppRole)
	if _, err := q.ExecSQL(ctx, infraSQL); err != nil {
		pool.Close()
		t.Fatalf("db.SetupTestDB: install RLS helpers: %v", err)
	}

	t.Cleanup(func() {
		// Reset role before dropping schema (role switch may block DROP).
		_, _ = pool.Exec(context.Background(), "RESET ROLE")
		pool.Close()

		cleanupPool, cerr := pgxpool.New(context.Background(), dsn)
		if cerr != nil {
			t.Logf("db.SetupTestDB cleanup: connect: %v", cerr)
			return
		}
		defer cleanupPool.Close()
		cq := contribpgx.NewPoolQuerier(cleanupPool)
		if _, err := cq.ExecSQL(context.Background(),
			fmt.Sprintf(`DROP SCHEMA %q CASCADE`, schemaName)); err != nil {
			t.Logf("db.SetupTestDB cleanup: drop schema %q: %v", schemaName, err)
		}
	})

	return pool
}

// WithTenant attaches tenantID to ctx via the framework's tenant.WithContext.
func WithTenant(ctx context.Context, tenantID uuid.UUID) context.Context {
	return tenant.WithContext(ctx, tenant.TenantContext{TenantID: tenantID})
}

// RawTenantID returns a new random UUID for use as a test tenant ID.
func RawTenantID() uuid.UUID {
	return uuid.New()
}

// ActivateTenant sets the RLS tenant context AND switches the session role to
// AppRole ("awo_app"). Both must be called together: RLS checks current_tenant_id()
// AND requires a non-superuser role.
//
// Call this before any DML or SELECT on tenant-scoped tables.
func ActivateTenant(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID) {
	t.Helper()
	q := contribpgx.NewPoolQuerier(pool)
	// set_tenant_context first (session-level).
	if _, err := q.ExecSQL(context.Background(),
		"SELECT set_tenant_context($1)", tenantID.String()); err != nil {
		t.Fatalf("db.ActivateTenant: set_tenant_context: %v", err)
	}
	// Switch to non-superuser role so RLS is enforced.
	if _, err := q.ExecSQL(context.Background(),
		fmt.Sprintf("SET ROLE %s", AppRole)); err != nil {
		t.Fatalf("db.ActivateTenant: SET ROLE %s: %v", AppRole, err)
	}
}

// ResetRole switches the session back to the original superuser/admin role.
// Use this when subsequent operations need superuser privileges (e.g., DDL).
func ResetRole(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	q := contribpgx.NewPoolQuerier(pool)
	if _, err := q.ExecSQL(context.Background(), "RESET ROLE"); err != nil {
		t.Fatalf("db.ResetRole: %v", err)
	}
}

// ApplySQL executes raw SQL (e.g., a generated migration) as the original
// superuser role. Automatically resets role before executing so DDL succeeds.
func ApplySQL(t *testing.T, pool *pgxpool.Pool, sql string) {
	t.Helper()
	ResetRole(t, pool) // DDL requires superuser/owner privileges
	q := contribpgx.NewPoolQuerier(pool)
	if _, err := q.ExecSQL(context.Background(), sql); err != nil {
		t.Fatalf("db.ApplySQL:\n%v\nSQL:\n%s", err, sql)
	}
}

// TableExists reports whether tableName exists in the current search_path.
func TableExists(t *testing.T, pool *pgxpool.Pool, tableName string) bool {
	t.Helper()
	var exists bool
	err := pool.QueryRow(context.Background(),
		`SELECT EXISTS(
			SELECT 1 FROM information_schema.tables
			WHERE table_name = $1
			  AND table_schema = current_schema()
		)`, tableName).Scan(&exists)
	if err != nil {
		t.Fatalf("db.TableExists(%q): %v", tableName, err)
	}
	return exists
}

// RowCount returns the number of rows visible under the current RLS context.
func RowCount(t *testing.T, pool *pgxpool.Pool, tableName string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		fmt.Sprintf(`SELECT COUNT(*) FROM %q`, tableName)).Scan(&n); err != nil {
		t.Fatalf("db.RowCount(%q): %v", tableName, err)
	}
	return n
}
