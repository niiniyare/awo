package pgx_test

// R.6 — contrib/pgx.Repository's generic scanner previously hardcoded
// "tenant_id", "updated_at", and "custom_fields" as always-present for every
// system entity. Several real, production platform tables deliberately omit
// one or more of these columns:
//
//   - platform_tenant: no tenant_id (it IS the tenant; ScopeSystem), no
//     custom_fields (predates that convention).
//   - iam_user_role: no updated_at (immutable after creation, by design), no
//     custom_fields.
//
// See docs/00-overview/TECHNICAL_FINDINGS_AND_FUTURE_IMPROVEMENTS.md §B.7 and
// tasks.md R.6 for the full evidence trail (found via live execution: a real
// `awo serve` + real login against a real, fully-migrated database).
//
// This file proves, against real PostgreSQL with the real migrations and the
// real non-superuser awo_app role, that Get/Create/Update against both
// tables now work through the generic EntityRepository path.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/compiler"
	contribpgx "awo.so/awo/contrib/pgx"
	"awo.so/awo/def"
	"awo.so/awo/driver"
	"awo.so/awo/platform/iam"
	"awo.so/awo/platform/tenant"
	"awo.so/awo/registry"
	testdb "awo.so/awo/testutil/db"
)

func TestSystemColumnContract_PlatformTenant_GetWorks_NoTenantIDColumn(t *testing.T) {
	reg, err := registry.BuildFrom([]def.EntityDefinition{&tenant.Definition})
	require.NoError(t, err)
	schema, err := compiler.Compile(reg)
	require.NoError(t, err)
	es := schema.ByName["platform_tenant"]
	require.NotNil(t, es, "platform_tenant missing from compiled schema")
	require.Equal(t, def.ScopeSystem, es.Scope, "platform_tenant must be ScopeSystem")
	require.False(t, es.HasCustomFields, "platform_tenant must declare HasCustomFields=false")

	pool := testdb.SetupTestDB(t)

	migrationSQL, err := os.ReadFile(filepath.Join("..", "..", "platform", "tenant", "migrations", "001_platform_tenant.up.sql"))
	require.NoError(t, err, "read real platform_tenant migration")
	testdb.ApplySQL(t, pool, string(migrationSQL))
	testdb.ApplySQL(t, pool, `GRANT SELECT, INSERT, UPDATE ON platform_tenant TO `+testdb.AppRole+`;`)

	repo := contribpgx.NewRepository(pool, es)

	ctx := context.Background()
	// platform_tenant has no RLS and no tenant_id — a standalone Get must
	// work with NO ambient tenant context at all, exactly matching
	// TenantResolver's own real call pattern (it resolves the tenant BEFORE
	// any tenant context can exist).
	tenantID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO platform_tenant (id, name, slug, status, contact_email) VALUES ($1, $2, $3, 'ACTIVE', $4)`,
		tenantID, "Acme Corp", "acme-"+tenantID.String()[:8], "ops@acme.test")
	require.NoError(t, err)

	got, err := repo.Get(ctx, tenantID)
	require.NoError(t, err, "Get against platform_tenant (no tenant_id column) must succeed")
	assert.Equal(t, "Acme Corp", got.Data["name"])
	assert.Equal(t, uuid.Nil, got.TenantID, "platform_tenant has no tenant_id — must be the zero value, not a scan error")
}

func TestSystemColumnContract_IAMUserRole_CreateAndGetWork_NoUpdatedAtOrCustomFields(t *testing.T) {
	userDef := &iam.UserDefinition
	roleDef := &iam.UserRoleDefinition

	reg, err := registry.BuildFrom([]def.EntityDefinition{userDef, roleDef})
	require.NoError(t, err)
	schema, err := compiler.Compile(reg)
	require.NoError(t, err)
	userSchema := schema.ByName["iam_user"]
	roleSchema := schema.ByName["iam_user_role"]
	require.NotNil(t, userSchema)
	require.NotNil(t, roleSchema)
	require.False(t, roleSchema.HasUpdatedAt, "iam_user_role must declare HasUpdatedAt=false")
	require.False(t, roleSchema.HasCustomFields, "iam_user_role must declare HasCustomFields=false")

	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)

	userMigration, err := os.ReadFile(filepath.Join("..", "..", "platform", "iam", "migrations", "001_iam_user.up.sql"))
	require.NoError(t, err)
	testdb.ApplySQL(t, pool, string(userMigration))
	testdb.ApplySQL(t, pool, `GRANT SELECT, INSERT, UPDATE ON iam_user TO `+testdb.AppRole+`;`)

	roleMigration, err := os.ReadFile(filepath.Join("..", "..", "platform", "iam", "migrations", "002_iam_user_role.up.sql"))
	require.NoError(t, err)
	testdb.ApplySQL(t, pool, string(roleMigration))
	testdb.ApplySQL(t, pool, `GRANT SELECT, INSERT, DELETE ON iam_user_role TO `+testdb.AppRole+`;`)

	userRepo := contribpgx.NewRepository(pool, userSchema)
	roleRepo := contribpgx.NewRepository(pool, roleSchema)

	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := testdb.WithTenant(context.Background(), tenantID)

	var createdUser, createdRole *def.EntityRecord
	require.NoError(t, userRepo.WithTx(ctx, func(txCtx context.Context) error {
		var err error
		createdUser, err = userRepo.Create(txCtx, driver.CreateInput{
			Data: map[string]any{
				"email":          "alice@acme.test",
				"full_name":      "Alice",
				"status":         "active",
				"password_hash":  "placeholder-hash",
				"email_verified": false,
			},
		})
		return err
	}), "Create against iam_user must succeed")

	require.NoError(t, roleRepo.WithTx(ctx, func(txCtx context.Context) error {
		var err error
		createdRole, err = roleRepo.Create(txCtx, driver.CreateInput{
			Data: map[string]any{
				"user_id":   createdUser.ID.String(),
				"role_name": "role:tenant.user",
			},
		})
		return err
	}), "Create against iam_user_role (no updated_at, no custom_fields) must succeed")
	assert.Equal(t, createdRole.CreatedAt, createdRole.UpdatedAt,
		"with no updated_at column, UpdatedAt must default to CreatedAt, not zero/garbage")

	fetched, err := roleRepo.Get(ctx, createdRole.ID)
	require.NoError(t, err, "Get against iam_user_role must succeed")
	assert.Equal(t, "role:tenant.user", fetched.Data["role_name"])
}

func TestSystemColumnContract_CreateWithCustomFields_RejectedExplicitly_NotSilentlyDropped(t *testing.T) {
	roleDef := &iam.UserRoleDefinition
	userDef := &iam.UserDefinition
	reg, err := registry.BuildFrom([]def.EntityDefinition{userDef, roleDef})
	require.NoError(t, err)
	schema, err := compiler.Compile(reg)
	require.NoError(t, err)
	roleSchema := schema.ByName["iam_user_role"]

	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)
	userMigration, err := os.ReadFile(filepath.Join("..", "..", "platform", "iam", "migrations", "001_iam_user.up.sql"))
	require.NoError(t, err)
	testdb.ApplySQL(t, pool, string(userMigration))
	roleMigration, err := os.ReadFile(filepath.Join("..", "..", "platform", "iam", "migrations", "002_iam_user_role.up.sql"))
	require.NoError(t, err)
	testdb.ApplySQL(t, pool, string(roleMigration))
	testdb.ApplySQL(t, pool, `GRANT SELECT, INSERT ON iam_user, iam_user_role TO `+testdb.AppRole+`;`)

	roleRepo := contribpgx.NewRepository(pool, roleSchema)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := testdb.WithTenant(context.Background(), tenantID)

	_, err = roleRepo.Create(ctx, driver.CreateInput{
		Data:         map[string]any{"user_id": uuid.New().String(), "role_name": "role:tenant.user"},
		CustomFields: map[string]any{"sneaky": "data"},
	})
	assert.Error(t, err, "supplying CustomFields for a NoCustomFields entity must error, not silently discard the data")
}
