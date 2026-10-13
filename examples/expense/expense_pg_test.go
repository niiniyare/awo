package expense_test

// Showcase vertical-slice test for business_expense (examples/expense).
//
// Proves, against real PostgreSQL with the real non-superuser awo_app role
// and the real, transaction-scoped production set_tenant_context()
// (testdb.InstallTenantLifecycle — the same convention established by Steps
// 7a/7b/8.1/Phase-3-Step-1), that the showcase entity's declared Go code
// alone produces a working end-to-end vertical slice: tenant-isolated
// persistence through the real contrib/pgx.Repository + the real
// EntityService.Create/Update pipeline, and the StatusTransitionGuard hook
// gating the business lifecycle.
//
// Does not re-verify generic EntityService machinery already proven
// elsewhere against real PostgreSQL (audit-row actor attribution — Step 7b;
// standalone-read tenant context — Step 7a; mutation/audit/outbox
// atomicity — api/service/entity_pg_atomicity_test.go). Scope here is what
// is specific to this entity: its own migration, its own FK, and its own
// hook.
//
// iam_user is stood in with a minimal id+tenant_id table (same convention
// platform/organization's own test file uses for platform_organization's
// unrelated FK targets) — only the FK relationship matters here, not IAM's
// own behavior.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/api/service"
	"awo.so/awo/audit"
	"awo.so/awo/compiler"
	contribpgx "awo.so/awo/contrib/pgx"
	"awo.so/awo/def"
	"awo.so/awo/examples/expense"
	"awo.so/awo/registry"
	"awo.so/awo/runtime"
	"awo.so/awo/runtime/tenant"
	testdb "awo.so/awo/testutil/db"
)

const setUpdatedAtSQL = `
CREATE OR REPLACE FUNCTION set_updated_at()
    RETURNS trigger LANGUAGE plpgsql AS
$$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;
`

const iamUserStubDDL = `
CREATE TABLE IF NOT EXISTS iam_user (
    id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL
);
`

func setupExpenseService(t *testing.T) (svc *service.EntityService, pool *pgxpool.Pool) {
	t.Helper()

	iamUserDef := &def.SystemDefinition{Name: "user", Module: "iam"}
	reg, err := registry.BuildFrom([]def.EntityDefinition{iamUserDef, &expense.Definition})
	require.NoError(t, err)
	schema, err := compiler.Compile(reg)
	require.NoError(t, err)
	es := schema.ByName["business_expense"]
	require.NotNil(t, es, "business_expense missing from compiled schema")

	p := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, p)
	testdb.ApplySQL(t, p, setUpdatedAtSQL)
	testdb.ApplySQL(t, p, iamUserStubDDL)

	migrationSQL, err := os.ReadFile(filepath.Join("migrations", "001_create_business_expense.up.sql"))
	require.NoError(t, err, "read real business_expense migration")
	testdb.ApplySQL(t, p, string(migrationSQL))
	testdb.ApplySQL(t, p, `GRANT SELECT, INSERT, UPDATE, DELETE ON business_expense TO `+testdb.AppRole+`;`)

	repo := contribpgx.NewRepository(p, es)
	pipeline := runtime.NewPipeline(schema, audit.NoopAuditWriter{})
	svc = service.NewEntityService(es, repo, pipeline)
	return svc, p
}

func newExpenseCtx(tenantID uuid.UUID) context.Context {
	return tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})
}

// newActor inserts a backing row into the iam_user stub (business_expense's
// submitted_by is a real FK) and returns an Actor referencing it.
func newActor(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID) *def.Actor {
	t.Helper()
	userID := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO iam_user (id, tenant_id) VALUES ($1, $2)`, userID, tenantID)
	require.NoError(t, err)
	return &def.Actor{UserID: userID, TenantID: tenantID}
}

func TestExpense_Create_DefaultsToDraft_TenantScoped(t *testing.T) {
	svc, pool := setupExpenseService(t)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	actor := newActor(t, pool, tenantID)
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := newExpenseCtx(tenantID)

	created, err := svc.Create(ctx, map[string]any{
		"description":  "Flight to client site",
		"amount":       "450.00",
		"category":     "travel",
		"submitted_by": actor.UserID.String(),
	}, actor)
	require.NoError(t, err)
	assert.Equal(t, "draft", created.Data["status"])

	fetched, err := svc.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "draft", fetched.Data["status"])
}

func TestExpense_TenantIsolation_OtherTenantCannotSeeIt(t *testing.T) {
	svc, pool := setupExpenseService(t)
	tenantA := testdb.CreateTenant(t, pool, "ACTIVE")
	tenantB := testdb.CreateTenant(t, pool, "ACTIVE")
	actorA := newActor(t, pool, tenantA)

	testdb.ActivateTenant(t, pool, tenantA)
	ctxA := newExpenseCtx(tenantA)
	created, err := svc.Create(ctxA, map[string]any{
		"description":  "Tenant A's expense",
		"amount":       "10.00",
		"category":     "other",
		"submitted_by": actorA.UserID.String(),
	}, actorA)
	require.NoError(t, err)

	testdb.ActivateTenant(t, pool, tenantB)
	ctxB := newExpenseCtx(tenantB)
	_, err = svc.Get(ctxB, created.ID)
	assert.True(t, runtime.IsNotFound(err), "tenant B must not see tenant A's expense")
}

func TestExpense_StatusTransitionGuard_ValidTransition_Succeeds(t *testing.T) {
	svc, pool := setupExpenseService(t)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	actor := newActor(t, pool, tenantID)
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := newExpenseCtx(tenantID)

	created, err := svc.Create(ctx, map[string]any{
		"description":  "Team lunch",
		"amount":       "60.00",
		"category":     "meals",
		"submitted_by": actor.UserID.String(),
	}, actor)
	require.NoError(t, err)
	require.Equal(t, "draft", created.Data["status"])

	submitted, err := svc.Update(ctx, created.ID, map[string]any{"status": "submitted"}, actor)
	require.NoError(t, err, "draft -> submitted must be allowed")
	assert.Equal(t, "submitted", submitted.Data["status"])

	approved, err := svc.Update(ctx, created.ID, map[string]any{"status": "approved"}, actor)
	require.NoError(t, err, "submitted -> approved must be allowed")
	assert.Equal(t, "approved", approved.Data["status"])
}

func TestExpense_StatusTransitionGuard_InvalidTransition_Rejected(t *testing.T) {
	svc, pool := setupExpenseService(t)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	actor := newActor(t, pool, tenantID)
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := newExpenseCtx(tenantID)

	created, err := svc.Create(ctx, map[string]any{
		"description":  "Suspicious jump straight to approved",
		"amount":       "999.00",
		"category":     "other",
		"submitted_by": actor.UserID.String(),
	}, actor)
	require.NoError(t, err)
	require.Equal(t, "draft", created.Data["status"])

	_, err = svc.Update(ctx, created.ID, map[string]any{"status": "approved"}, actor)
	assert.Error(t, err, "draft -> approved must be rejected (must go through submitted)")

	var be *runtime.BusinessError
	assert.ErrorAs(t, err, &be)
	if be != nil {
		assert.Equal(t, "expense.invalid_status_transition", be.Code)
	}

	// The row must remain in its prior state — the guard ran BeforeUpdate,
	// before any transaction opened, so nothing was persisted.
	fetched, getErr := svc.Get(ctx, created.ID)
	require.NoError(t, getErr)
	assert.Equal(t, "draft", fetched.Data["status"])
}

func TestExpense_StatusTransitionGuard_UnrelatedFieldUpdate_Unaffected(t *testing.T) {
	svc, pool := setupExpenseService(t)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	actor := newActor(t, pool, tenantID)
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := newExpenseCtx(tenantID)

	created, err := svc.Create(ctx, map[string]any{
		"description":  "Office supplies",
		"amount":       "25.00",
		"category":     "supplies",
		"submitted_by": actor.UserID.String(),
	}, actor)
	require.NoError(t, err)

	// Updating an unrelated field (not status) must not trigger the guard at
	// all — proves the guard checks the status transition specifically, not
	// just "update exists" (the exact failure mode this test would catch if
	// StatusTransitionGuard repeated platform/organization.MoveGuard's
	// parameter-order bug — see the technical findings register).
	updated, err := svc.Update(ctx, created.ID, map[string]any{"notes": "receipt attached"}, actor)
	require.NoError(t, err, "updating an unrelated field must not be blocked by the status guard")
	assert.Equal(t, "receipt attached", updated.Data["notes"])
	assert.Equal(t, "draft", updated.Data["status"])
}
