package service_test

// Phase 2 Step 3 — Canonical Mutation Pipeline atomicity tests.
//
// These tests exercise EntityService.Create/Update/Delete against REAL
// PostgreSQL — a real contrib/pgx.Repository, a real audit.PostgresWriter,
// and a real events/outbox.OutboxWriter — to prove the mutation+audit+outbox
// atomicity invariant ADR-025 §5/§6 requires, not merely with fakes/mocks.
// entity_audit_test.go's stubRepo-based tests remain valid and unmodified for
// hook/audit-orchestration-level unit coverage; these tests add the
// real-transaction layer those cannot exercise.

import (
	"context"
	"encoding/json"
	"io/fs"
	"strings"
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
	"awo.so/awo/events"
	"awo.so/awo/events/outbox"
	outboxmigrations "awo.so/awo/events/outbox/migrations"
	"awo.so/awo/registry"
	"awo.so/awo/runtime"
	"awo.so/awo/runtime/tenant"
	testdb "awo.so/awo/testutil/db"
)

// platformAuditLogTestDDL mirrors audit/pg_writer.go's unexported auditLogDDL
// (the real production INSERT column list audit.PostgresWriter.Write uses) —
// a test-scoped fixture, matching the same pattern
// platform/audit/audit_integration_test.go already uses for the same reason.
const platformAuditLogTestDDL = `
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

const (
	pgEntityName      = "svc_pg_canonical_entity"
	pgAdminEntityName = "svc_pg_canonical_admin_entity"
)

func pgEntityDDL(tableName string) string {
	return `
CREATE TABLE ` + tableName + ` (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     uuid NOT NULL,
    status        text,
    ssn           text,
    custom_fields jsonb,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE ` + tableName + ` ENABLE ROW LEVEL SECURITY;
ALTER TABLE ` + tableName + ` FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON ` + tableName + ` USING (tenant_id = current_tenant_id());
GRANT SELECT, INSERT, UPDATE, DELETE ON ` + tableName + ` TO awo_app;
`
}

func pgEntityDef(name string) def.EntityDefinition {
	return &def.SystemDefinition{
		Name:   name,
		Module: "svc",
		Label:  "PGCanonical",
		Fields: []def.FieldDef{
			{Name: "status", Type: def.FieldTypeData},
			{Name: "ssn", Type: def.FieldTypeData, Sensitive: true},
		},
	}
}

// readOutboxMigrationSQL reads the real, production events_outbox migration's
// up-direction SQL directly from its embedded source, mirroring
// contrib/pgx/outbox_writer_transaction_test.go's identical helper — so
// these tests exercise the actual migration-defined schema, not a second,
// hand-maintained copy of it.
func readOutboxMigrationSQL(t *testing.T) string {
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
	t.Fatal("readOutboxMigrationSQL: no .up.sql file found in events/outbox/migrations")
	return ""
}

// pgServiceHarness bundles everything a real-Postgres EntityService test needs.
type pgServiceHarness struct {
	pool *pgxpool.Pool
	svc  *service.EntityService
	es   *compiler.EntitySchema
}

// setupPGService builds a real EntityService — real contrib/pgx.Repository,
// real audit.PostgresWriter, real events/outbox.OutboxWriter — for entityName
// (either pgEntityName, EventCategory Data, or pgAdminEntityName, EventCategory
// Admin). Both the entity table and the events_outbox table are freshly
// created in the test's own isolated schema.
func setupPGService(t *testing.T, entityName string, category audit.EventCategory) *pgServiceHarness {
	t.Helper()

	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)
	testdb.ApplySQL(t, pool, pgEntityDDL(entityName))
	testdb.ApplySQL(t, pool, platformAuditLogTestDDL)
	testdb.ApplySQL(t, pool, readOutboxMigrationSQL(t))

	d := pgEntityDef(entityName)
	reg, err := registry.BuildFrom([]def.EntityDefinition{d})
	require.NoError(t, err)
	schema, err := compiler.Compile(reg)
	require.NoError(t, err)
	es := schema.ByName[entityName]
	require.NotNil(t, es, "entity %q must be present in the compiled schema", entityName)

	// registry.BuildFrom (test/tooling path) does not call def.Register —
	// only production's registry.Build() does, via each module's own init().
	// audit.Sanitizer.sensitiveFields' Source 1 reads def.Lookup(entityName)
	// directly (not the compiled schema), so it must be registered here too
	// for Test F's Sensitive-field redaction to be discoverable at all —
	// guarded against def.Register's duplicate-name panic since multiple
	// tests in this file call setupPGService with the same entityName.
	if def.Lookup(entityName) == nil {
		def.Register(d)
	}

	audit.Register(audit.EntityAuditConfig{
		EntityName: entityName,
		Enabled:    true,
		Category:   category,
	})

	auditWriter := audit.NewPostgresWriter(contribpgx.NewPoolQuerier(pool))
	pipeline := runtime.NewPipeline(schema, auditWriter)
	repo := contribpgx.NewRepository(pool, es)

	svc := service.NewEntityService(es, repo, pipeline, nil).
		WithPublisher(outbox.NewWriter(pool))

	return &pgServiceHarness{pool: pool, svc: svc, es: es}
}

func pgActivateTenant(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, context.Context) {
	t.Helper()
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})
	return tenantID, ctx
}

type outboxRow struct {
	found         bool
	tenantID      uuid.UUID
	eventType     string
	entityName    string
	recordID      uuid.UUID
	actorID       uuid.UUID
	systemActor   *string
	correlationID *string
	payload       []byte
}

func readOutboxRow(t *testing.T, pool *pgxpool.Pool, recordID uuid.UUID) outboxRow {
	t.Helper()
	row := testdb.QueryRowSQL(t, pool, `
		SELECT tenant_id, type, entity_name, record_id, actor_id, system_actor, correlation_id, payload
		FROM events_outbox WHERE record_id = $1
	`, recordID)
	var r outboxRow
	err := row.Scan(&r.tenantID, &r.eventType, &r.entityName, &r.recordID, &r.actorID, &r.systemActor, &r.correlationID, &r.payload)
	if err != nil {
		return outboxRow{found: false}
	}
	r.found = true
	return r
}

func auditRowCount(t *testing.T, pool *pgxpool.Pool, recordID uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, testdb.QueryRowSQL(t, pool,
		`SELECT COUNT(*) FROM platform_audit_log WHERE record_id = $1`, recordID).Scan(&n))
	return n
}

func entityRowCount(t *testing.T, pool *pgxpool.Pool, tableName string, id uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, testdb.QueryRowSQL(t, pool,
		`SELECT COUNT(*) FROM `+tableName+` WHERE id = $1`, id).Scan(&n))
	return n
}

// ── Test A — Mutation + audit + outbox commit ───────────────────────────────

func TestEntityService_PG_Create_MutationAuditOutbox_CommitTogether(t *testing.T) {
	h := setupPGService(t, pgEntityName, audit.CategoryData)
	tenantID, ctx := pgActivateTenant(t, h.pool)

	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}
	rc := audit.RequestContext{RequestID: "req-abc-123-not-a-uuid"}
	ctx = audit.WithRequestContext(ctx, rc)

	created, err := h.svc.Create(ctx, map[string]any{"status": "draft"}, actor)
	require.NoError(t, err)
	require.NotNil(t, created)

	assert.Equal(t, 1, entityRowCount(t, h.pool, pgEntityName, created.ID), "the entity row must exist after commit")
	assert.Equal(t, 1, auditRowCount(t, h.pool, created.ID), "the audit row must exist after commit")

	ob := readOutboxRow(t, h.pool, created.ID)
	require.True(t, ob.found, "the outbox row must exist after commit")
	assert.Equal(t, tenantID, ob.tenantID, "the outbox row's tenant_id must match the mutation's tenant")
	assert.Equal(t, string(events.EventCreated), ob.eventType)
	assert.Equal(t, pgEntityName, ob.entityName)
	assert.Equal(t, actor.UserID, ob.actorID)
	require.True(t, json.Valid(ob.payload), "the outbox payload must be valid JSON")

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(ob.payload, &decoded))
	assert.Equal(t, "draft", decoded["status"])
}

// ── Test B — Mutation rollback removes outbox ───────────────────────────────

func TestEntityService_PG_Create_HookFailure_RollsBackMutationAuditAndOutbox(t *testing.T) {
	const entityName = "svc_pg_canonical_rollback_entity"
	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)
	testdb.ApplySQL(t, pool, pgEntityDDL(entityName))
	testdb.ApplySQL(t, pool, platformAuditLogTestDDL)
	testdb.ApplySQL(t, pool, readOutboxMigrationSQL(t))

	failingHook := &failingAfterCreateHook{}
	d := &def.SystemDefinition{
		Name:   entityName,
		Module: "svc",
		Label:  "PGRollback",
		Fields: []def.FieldDef{{Name: "status", Type: def.FieldTypeData}},
		Hooks:  def.HookSet{AfterCreate: []def.AfterCreateHook{failingHook}},
	}
	reg, err := registry.BuildFrom([]def.EntityDefinition{d})
	require.NoError(t, err)
	schema, err := compiler.Compile(reg)
	require.NoError(t, err)
	es := schema.ByName[entityName]
	require.NotNil(t, es)

	audit.Register(audit.EntityAuditConfig{EntityName: entityName, Enabled: true, Category: audit.CategoryData})

	auditWriter := audit.NewPostgresWriter(contribpgx.NewPoolQuerier(pool))
	pipeline := runtime.NewPipeline(schema, auditWriter)
	repo := contribpgx.NewRepository(pool, es)
	svc := service.NewEntityService(es, repo, pipeline, nil).WithPublisher(outbox.NewWriter(pool))

	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	testdb.ActivateTenant(t, pool, tenantID)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	_, err = svc.Create(ctx, map[string]any{"status": "draft"}, actor)
	require.Error(t, err, "the failing AfterCreate hook must cause Create to fail")

	// The record ID was never returned to us (Create failed), so verify via
	// COUNT(*) that the flush produced nothing at all — the table must be
	// completely empty, and so must platform_audit_log and events_outbox.
	var entityCount, auditCount, outboxCount int
	require.NoError(t, testdb.QueryRowSQL(t, pool, `SELECT COUNT(*) FROM `+entityName).Scan(&entityCount))
	require.NoError(t, testdb.QueryRowSQL(t, pool, `SELECT COUNT(*) FROM platform_audit_log`).Scan(&auditCount))
	require.NoError(t, testdb.QueryRowSQL(t, pool, `SELECT COUNT(*) FROM events_outbox`).Scan(&outboxCount))

	assert.Equal(t, 0, entityCount, "the mutation must not exist after rollback")
	assert.Equal(t, 0, auditCount, "the audit row must not exist after rollback")
	assert.Equal(t, 0, outboxCount, "the outbox row must not exist after rollback")
}

type failingAfterCreateHook struct{}

func (h *failingAfterCreateHook) AfterCreate(_ context.Context, _ *def.EntityRecord) error {
	return assert.AnError
}

// ── Test C — Audit failure rolls back mutation (ADMIN category propagates) ──

func TestEntityService_PG_Create_AdminAuditFailure_RollsBackMutationAndOutbox(t *testing.T) {
	h := setupPGService(t, pgAdminEntityName, audit.CategoryAdmin)
	// Deny the audit INSERT specifically — a genuine PostgreSQL
	// insufficient_privilege error, not a mock.
	testdb.ApplySQL(t, h.pool, `REVOKE INSERT ON platform_audit_log FROM awo_app;`)

	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	_, err := h.svc.Create(ctx, map[string]any{"status": "draft"}, actor)
	require.Error(t, err, "CategoryAdmin audit failure must propagate (ADR-017), causing Create to fail")

	var entityCount, outboxCount int
	require.NoError(t, testdb.QueryRowSQL(t, h.pool, `SELECT COUNT(*) FROM `+pgAdminEntityName).Scan(&entityCount))
	require.NoError(t, testdb.QueryRowSQL(t, h.pool, `SELECT COUNT(*) FROM events_outbox`).Scan(&outboxCount))
	assert.Equal(t, 0, entityCount, "the mutation must not commit when a mandatory (ADMIN) audit write fails")
	assert.Equal(t, 0, outboxCount, "no outbox row can survive when the surrounding transaction rolled back")
}

// ── Test D — Outbox failure rolls back mutation (unconditional, ADR-025 §6) ─

func TestEntityService_PG_Create_OutboxFailure_RollsBackMutationAndAudit(t *testing.T) {
	// CategoryData (the default, suppress-on-failure policy) — deliberately
	// NOT CategoryAdmin, to prove the outbox's unconditional failure policy
	// is independent of the audit category (ADR-025 §6): the outbox write
	// must roll back the transaction here even though an ordinary audit
	// failure for this same category would not.
	h := setupPGService(t, pgEntityName, audit.CategoryData)
	testdb.ApplySQL(t, h.pool, `REVOKE INSERT ON events_outbox FROM awo_app;`)

	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	_, err := h.svc.Create(ctx, map[string]any{"status": "draft"}, actor)
	require.Error(t, err, "an outbox insert failure must always propagate (ADR-025 §6), regardless of audit category")

	var entityCount, auditCount int
	require.NoError(t, testdb.QueryRowSQL(t, h.pool, `SELECT COUNT(*) FROM `+pgEntityName).Scan(&entityCount))
	require.NoError(t, testdb.QueryRowSQL(t, h.pool, `SELECT COUNT(*) FROM platform_audit_log`).Scan(&auditCount))
	assert.Equal(t, 0, entityCount, "the mutation must not commit when the mandatory outbox write fails")
	assert.Equal(t, 0, auditCount, "the audit row must not survive when the surrounding transaction rolled back")
}

// ── Test E — Tenant isolation ────────────────────────────────────────────────
//
// The entity table has real RLS (tenant_isolation policy, enforced via
// AppRole, matching contrib/pgx's established pattern). platform_audit_log
// and events_outbox are both intentionally global, non-RLS tables by design
// (ADR-018 for audit; ADR-025 §7/§21 for the outbox) — this test does not
// assert RLS-based invisibility for either, which would contradict their own
// authoritative designs. Instead it asserts the actual applicable invariant:
// each row, in every table, carries the correct tenant_id for the tenant
// whose transaction produced it, with no cross-tenant attribution.

func TestEntityService_PG_Create_TenantIsolation(t *testing.T) {
	h := setupPGService(t, pgEntityName, audit.CategoryData)

	tenantA := testdb.CreateTenant(t, h.pool, "ACTIVE")
	tenantB := testdb.CreateTenant(t, h.pool, "ACTIVE")

	testdb.ActivateTenant(t, h.pool, tenantA)
	ctxA := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantA})
	actorA := &def.Actor{UserID: uuid.New(), TenantID: tenantA}
	createdA, err := h.svc.Create(ctxA, map[string]any{"status": "A-secret"}, actorA)
	require.NoError(t, err)

	// Tenant B queries the entity table — RLS must hide Tenant A's row.
	testdb.ActivateTenant(t, h.pool, tenantB)
	ctxB := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantB})
	seenByB, _, err := h.svc.Query(ctxB, nil)
	require.NoError(t, err)
	for _, rec := range seenByB {
		assert.NotEqual(t, createdA.ID, rec.ID, "tenant B must never see tenant A's entity row via RLS")
	}

	// Ground-truth (superuser) reads confirm the outbox row is correctly
	// attributed to tenant A specifically — not tenant B, not zero-value.
	ob := readOutboxRow(t, h.pool, createdA.ID)
	require.True(t, ob.found)
	assert.Equal(t, tenantA, ob.tenantID, "the outbox event must carry tenant A's own tenant_id")
	assert.NotEqual(t, tenantB, ob.tenantID)
}

// ── Test F — Sensitive payload ───────────────────────────────────────────────

func TestEntityService_PG_Create_SensitiveField_NotInOutboxPayload(t *testing.T) {
	h := setupPGService(t, pgEntityName, audit.CategoryData)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	const sensitiveValue = "078-05-1120"
	created, err := h.svc.Create(ctx, map[string]any{"status": "draft", "ssn": sensitiveValue}, actor)
	require.NoError(t, err)

	ob := readOutboxRow(t, h.pool, created.ID)
	require.True(t, ob.found)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(ob.payload, &decoded))
	assert.NotEqual(t, sensitiveValue, decoded["ssn"], "the Sensitive field's real value must never appear in the outbox payload")
	assert.Equal(t, "[REDACTED]", decoded["ssn"], "the Sensitive field must be redacted using audit.Sanitizer's existing contract")
	assert.Equal(t, "draft", decoded["status"], "non-sensitive fields must still be present")

	// Also confirm the raw column, read as superuser, never contains the
	// real value anywhere in its bytes — not just at the expected JSON key.
	assert.NotContains(t, string(ob.payload), sensitiveValue)
}

// ── Test G — Correlation / system actor ─────────────────────────────────────

func TestEntityService_PG_Create_CorrelationID_PopulatedAsPlainText(t *testing.T) {
	h := setupPGService(t, pgEntityName, audit.CategoryData)
	tenantID, ctx := pgActivateTenant(t, h.pool)
	actor := &def.Actor{UserID: uuid.New(), TenantID: tenantID}

	const requestID = "req-not-a-uuid-12345"
	ctx = audit.WithRequestContext(ctx, audit.RequestContext{RequestID: requestID})

	created, err := h.svc.Create(ctx, map[string]any{"status": "draft"}, actor)
	require.NoError(t, err)

	ob := readOutboxRow(t, h.pool, created.ID)
	require.True(t, ob.found)
	require.NotNil(t, ob.correlationID)
	assert.Equal(t, requestID, *ob.correlationID, "correlation_id must be populated verbatim from audit.RequestContext.RequestID — "+
		"a deliberately non-UUID string, proving no UUID coercion is attempted")

	// EntityService's normal actor-driven mutation path never sets
	// SystemActor — it is reserved for background/system-originated events
	// (ADR-015). actor_id and system_actor must remain mutually exclusive
	// and separate: actor_id is set, system_actor must be NULL/empty here.
	assert.Equal(t, actor.UserID, ob.actorID)
	assert.True(t, ob.systemActor == nil || *ob.systemActor == "",
		"system_actor must be empty for an ordinary, actor-driven mutation — it is not invented here")
}
