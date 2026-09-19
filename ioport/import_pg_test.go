package ioport_test

// Phase 2 Step 5 — end-to-end integration: ioport.Import wired to a REAL
// *api/service.EntityService (real contrib/pgx.Repository, real
// audit.PostgresWriter, real events/outbox.OutboxWriter), proving the two
// packages actually fit together through the ioport.EntityBatchMutator
// interface, not just that each compiles against it in isolation. The
// heavier atomicity/tenant/security matrix lives in
// api/service/entity_batch_pg_test.go (CreateBatch's own real-PostgreSQL
// tests); this file only proves ioport's CSV/JSON parsing and flush control
// flow drive that real pipeline correctly end to end.

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"awo.so/awo/api/service"
	"awo.so/awo/audit"
	"awo.so/awo/compiler"
	contribpgx "awo.so/awo/contrib/pgx"
	"awo.so/awo/def"
	"awo.so/awo/events/outbox"
	outboxmigrations "awo.so/awo/events/outbox/migrations"
	"awo.so/awo/registry"
	"awo.so/awo/runtime"
	"awo.so/awo/runtime/tenant"
	testdb "awo.so/awo/testutil/db"

	"io/fs"

	. "awo.so/awo/ioport"
)

const ioPGEntityName = "io_pg_import_entity"

func ioPGEntityDDL() string {
	return `
CREATE TABLE ` + ioPGEntityName + ` (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     uuid NOT NULL,
    sku           text,
    price         text,
    custom_fields jsonb,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE ` + ioPGEntityName + ` ENABLE ROW LEVEL SECURITY;
ALTER TABLE ` + ioPGEntityName + ` FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON ` + ioPGEntityName + ` USING (tenant_id = current_tenant_id());
GRANT SELECT, INSERT, UPDATE, DELETE ON ` + ioPGEntityName + ` TO awo_app;
`
}

const ioPlatformAuditLogTestDDL = `
CREATE TABLE IF NOT EXISTS platform_audit_log (
    id               UUID        NOT NULL DEFAULT gen_random_uuid(),
    tenant_id        UUID        NOT NULL,
    request_id       TEXT,
    entity_name      TEXT        NOT NULL,
    record_id        UUID,
    operation        TEXT        NOT NULL,
    actor_id         UUID,
    service_account_id UUID,
    system_actor     TEXT,
    ip_address       TEXT,
    session_id       TEXT,
    before_data      JSONB,
    after_data       JSONB,
    changed_fields   TEXT[],
    event_category   TEXT        NOT NULL,
    severity         TEXT        NOT NULL DEFAULT 'INFO',
    risk_score       INT         NOT NULL DEFAULT 0,
    compliance_flags JSONB,
    context          JSONB,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT platform_audit_log_pkey PRIMARY KEY (id)
);
GRANT SELECT, INSERT ON platform_audit_log TO awo_app;
`

func readIOPGOutboxMigrationSQL(t *testing.T) string {
	t.Helper()
	fsys := outboxmigrations.SQLFS()
	entries, err := fs.ReadDir(fsys, ".")
	require.NoError(t, err)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			b, err := fs.ReadFile(fsys, e.Name())
			require.NoError(t, err)
			return string(b)
		}
	}
	t.Fatal("readIOPGOutboxMigrationSQL: no .up.sql file found")
	return ""
}

// setupIOPGService builds a real EntityService for ioPGEntityName, wired
// exactly the way api/router.Register wires one in production, so
// ioport.Import(ctx, schema, svc, ...) exercises the genuine canonical
// pipeline end to end.
func setupIOPGService(t *testing.T) (*service.EntityService, *compiler.EntitySchema, uuid.UUID, context.Context) {
	t.Helper()

	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)
	testdb.ApplySQL(t, pool, ioPGEntityDDL())
	testdb.ApplySQL(t, pool, ioPlatformAuditLogTestDDL)
	testdb.ApplySQL(t, pool, readIOPGOutboxMigrationSQL(t))

	d := &def.SystemDefinition{
		Name:   ioPGEntityName,
		Module: "io",
		Label:  "IOPGImport",
		Fields: []def.FieldDef{
			{Name: "sku", Type: def.FieldTypeData, Required: true},
			{Name: "price", Type: def.FieldTypeData},
		},
	}
	reg, err := registry.BuildFrom([]def.EntityDefinition{d})
	require.NoError(t, err)
	schema, err := compiler.Compile(reg)
	require.NoError(t, err)
	es := schema.ByName[ioPGEntityName]
	require.NotNil(t, es)

	audit.Register(audit.EntityAuditConfig{EntityName: ioPGEntityName, Enabled: true, Category: audit.CategoryData})
	auditWriter := audit.NewPostgresWriter(contribpgx.NewPoolQuerier(pool))
	pipeline := runtime.NewPipeline(schema, auditWriter)
	repo := contribpgx.NewRepository(pool, es)
	svc := service.NewEntityService(es, repo, pipeline, nil).WithPublisher(outbox.NewWriter(pool))

	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})

	return svc, es, tenantID, ctx
}

func TestImport_PG_CSV_EndToEnd_GoesThroughCanonicalPipeline(t *testing.T) {
	svc, es, _, ctx := setupIOPGService(t)
	actor := &def.Actor{UserID: uuid.New()}

	csv := "sku,price\nSKU-001,9.99\nSKU-002,19.99\nSKU-003,29.99\n"
	result, err := Import(ctx, es, svc, strings.NewReader(csv), ImportOptions{
		Format: FormatCSV,
		Actor:  actor,
	})
	require.NoError(t, err)
	require.Equal(t, 3, result.Created)
	require.Empty(t, result.Errors)
}

func TestImport_PG_CSV_RequiredFieldMissing_SkipErrorsFalse_RejectsWholeImport(t *testing.T) {
	svc, es, _, ctx := setupIOPGService(t)
	actor := &def.Actor{UserID: uuid.New()}

	// Row 2 is missing the required "sku" field.
	csv := "sku,price\nSKU-001,9.99\n,19.99\n"
	result, err := Import(ctx, es, svc, strings.NewReader(csv), ImportOptions{
		Format:     FormatCSV,
		Actor:      actor,
		SkipErrors: false,
	})
	require.Error(t, err, "a required-field violation must reject the import, not silently persist an invalid row")
	require.Equal(t, 0, result.Created, "no row may be reported created — not even the valid one before it")
}

func TestImport_PG_CSV_RequiredFieldMissing_SkipErrorsTrue_PersistsOnlyValidRows(t *testing.T) {
	svc, es, _, ctx := setupIOPGService(t)
	actor := &def.Actor{UserID: uuid.New()}

	csv := "sku,price\nSKU-001,9.99\n,19.99\nSKU-003,29.99\n"
	result, err := Import(ctx, es, svc, strings.NewReader(csv), ImportOptions{
		Format:     FormatCSV,
		Actor:      actor,
		SkipErrors: true,
	})
	require.NoError(t, err)
	require.Equal(t, 2, result.Created, "the 2 valid rows must persist despite 1 invalid row in the same flush")
	require.Len(t, result.Errors, 1)
}
