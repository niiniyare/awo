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
//	TEST_DATABASE_URL=postgres://user:pass@localhost:5432/awo?sslmode=disable
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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	contribpgx "awo.so/awo/contrib/pgx"
	"awo.so/awo/generator"
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
// Call this before any DML or SELECT on tenant-scoped tables. Fails the test
// immediately if set_tenant_context rejects tenantID — if InstallTenantLifecycle
// was called, that means tenantID must be a real, ACTIVE platform_tenant row
// (see CreateTenant); use TryActivateTenant instead to assert a rejection.
func ActivateTenant(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID) {
	t.Helper()
	if err := TryActivateTenant(pool, tenantID); err != nil {
		t.Fatalf("db.ActivateTenant: %v", err)
	}
}

// TryActivateTenant is ActivateTenant without the fatal assertion — it
// returns the set_tenant_context error (if any) instead of failing the test,
// for negative-path tests asserting that a PENDING/SUSPENDED/ARCHIVED or
// nonexistent tenant is rejected. On success, the role switch to AppRole
// still happens, matching ActivateTenant's behavior; on failure, no role
// switch occurs (the caller remains the superuser/owner role — call
// ResetRole is a no-op in that case since RESET ROLE with no prior SET ROLE
// is harmless).
func TryActivateTenant(pool *pgxpool.Pool, tenantID uuid.UUID) error {
	q := contribpgx.NewPoolQuerier(pool)
	if _, err := q.ExecSQL(context.Background(),
		"SELECT set_tenant_context($1)", tenantID.String()); err != nil {
		return fmt.Errorf("set_tenant_context: %w", err)
	}
	if _, err := q.ExecSQL(context.Background(),
		fmt.Sprintf("SET ROLE %s", AppRole)); err != nil {
		return fmt.Errorf("SET ROLE %s: %w", AppRole, err)
	}
	return nil
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

// OpenConcurrentPool opens a second pool against the same isolated test
// schema as pool (discovered via its current search_path), without the
// pool_max_conns=1 restriction SetupTestDB applies. Use this for tests that
// need genuine concurrent connections — SetupTestDB's own pool is
// deliberately pinned to a single connection so that sequential calls are
// guaranteed to reuse it (see SetupTestDB's doc comment), which is the
// opposite of what a concurrency test needs. The returned pool is closed
// automatically via t.Cleanup.
func OpenConcurrentPool(t *testing.T, pool *pgxpool.Pool) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping PostgreSQL integration test")
	}

	var searchPath string
	if err := pool.QueryRow(context.Background(), "SHOW search_path").Scan(&searchPath); err != nil {
		t.Fatalf("db.OpenConcurrentPool: discover search_path: %v", err)
	}

	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	cfg, err := pgxpool.ParseConfig(dsn + sep + "search_path=" + searchPath)
	if err != nil {
		t.Fatalf("db.OpenConcurrentPool: parse config: %v", err)
	}
	// Every physical connection the pool opens starts as the superuser/owner
	// role from the DSN — switch each one to AppRole as soon as it's
	// established so RLS is enforced no matter which connection a given
	// request happens to acquire.
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET ROLE "+AppRole)
		return err
	}
	concurrent, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("db.OpenConcurrentPool: %v", err)
	}
	t.Cleanup(concurrent.Close)
	return concurrent
}

// InstallTenantLifecycle creates a minimal platform_tenant(id, status) table
// in the test schema (if not already present) and replaces the schema's
// set_tenant_context with the real production implementation
// (generator.TenantContextSQL()) — the same SQL the framework's own
// migration generator and bootstrap migration emit, not a hand-maintained
// reimplementation that could silently drift from it and enforce something
// weaker than production actually does.
//
// Call this before CreateTenant/ActivateTenant in any test that needs to
// exercise tenant-lifecycle enforcement (PENDING/SUSPENDED/ARCHIVED
// rejection). Tests that only need tenant-ID row isolation and don't care
// about lifecycle status can continue using the simpler pass-through
// set_tenant_context(text) installed by SetupTestDB, and must NOT call this
// function (it replaces that pass-through with one that requires a real
// platform_tenant row to exist for any tenant ID used).
func InstallTenantLifecycle(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ResetRole(t, pool) // DDL requires superuser/owner privileges
	q := contribpgx.NewPoolQuerier(pool)
	ctx := context.Background()

	ddl := `
CREATE TABLE IF NOT EXISTS platform_tenant (
    id     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status text NOT NULL DEFAULT 'ACTIVE'
);
GRANT SELECT ON platform_tenant TO ` + AppRole + `;
`
	if _, err := q.ExecSQL(ctx, ddl); err != nil {
		t.Fatalf("db.InstallTenantLifecycle: create platform_tenant: %v", err)
	}

	if _, err := q.ExecSQL(ctx, generator.TenantContextSQL()); err != nil {
		t.Fatalf("db.InstallTenantLifecycle: install real set_tenant_context: %v", err)
	}
	if _, err := q.ExecSQL(ctx,
		fmt.Sprintf(`GRANT EXECUTE ON FUNCTION set_tenant_context(uuid) TO %s;`, AppRole)); err != nil {
		t.Fatalf("db.InstallTenantLifecycle: grant execute: %v", err)
	}
	if _, err := q.ExecSQL(ctx,
		fmt.Sprintf(`GRANT EXECUTE ON FUNCTION current_tenant_id() TO %s;`, AppRole)); err != nil {
		t.Fatalf("db.InstallTenantLifecycle: grant execute: %v", err)
	}
}

// CreateTenant inserts a platform_tenant row with the given status (e.g.
// "ACTIVE", "PENDING", "SUSPENDED", "ARCHIVED") and returns its ID. Requires
// InstallTenantLifecycle (or an equivalent migration providing
// platform_tenant) to have been applied first. Runs as the superuser/owner
// role since AppRole only has SELECT on platform_tenant (it is a
// framework-managed table, never written by application code via RLS).
//
// Pitfall: this calls ResetRole internally, so it silently undoes any
// earlier `SET ROLE` your test issued. Call all CreateTenant (and other
// superuser-requiring setup) before switching to AppRole for the RLS
// exercise you actually want to test, not the other way around — otherwise
// your test will silently run as the PostgreSQL superuser, which bypasses
// RLS entirely regardless of FORCE ROW LEVEL SECURITY, and any assertion
// that "tenant isolation held" will pass for the wrong reason.
func CreateTenant(t *testing.T, pool *pgxpool.Pool, status string) uuid.UUID {
	t.Helper()
	ResetRole(t, pool)
	id := uuid.New()
	q := contribpgx.NewPoolQuerier(pool)
	if _, err := q.ExecSQL(context.Background(),
		`INSERT INTO platform_tenant (id, status) VALUES ($1, $2)`, id, status); err != nil {
		t.Fatalf("db.CreateTenant: %v", err)
	}
	return id
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

// QueryRowSQL runs a single-row query as the original superuser/owner role
// (bypassing RLS, so it sees ground truth regardless of tenant context) and
// returns the resulting pgx.Row for the caller to Scan. Useful for asserting
// on raw table state directly, independent of what any tenant-scoped
// application code path would see.
func QueryRowSQL(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) pgx.Row {
	t.Helper()
	ResetRole(t, pool)
	return pool.QueryRow(context.Background(), sql, args...)
}

// QueryRowsSQL is QueryRowSQL for multi-row results. The caller must Close
// the returned pgx.Rows.
func QueryRowsSQL(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) (pgx.Rows, error) {
	t.Helper()
	ResetRole(t, pool)
	return pool.Query(context.Background(), sql, args...)
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
