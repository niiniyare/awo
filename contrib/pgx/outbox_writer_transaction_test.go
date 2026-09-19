package pgx_test

// Phase 2 Step 1 — Transaction-aware OutboxWriter.
//
// These tests prove events/outbox.OutboxWriter.Publish (a) actually joins the
// caller's existing database transaction via tx.QuerierFromContext (never a
// separate pool connection or a second transaction), and (b) preserves an
// already-JSON-encoded DomainEvent.Payload verbatim rather than re-marshaling
// it into a base64 string (PHASE2_ARCHITECTURE_PLAN.md §10/§21 Step 1,
// ADR-025 §7/§9.1).
//
// eventsOutboxTestDDL is a Step 1 test fixture only — it mirrors the columns
// events/outbox/relay.go's live poll()/Publish SQL already reference (id,
// tenant_id, type, entity_name, record_id, actor_id, action_name, payload,
// occurred_at, delivered_at, attempts, last_error), matching the same shape
// as the orphaned migrations/20260706000003_create_platform_support.up.sql
// schema. The real, ADR-025-reconciled migration (adding system_actor,
// correlation_id, next_attempt_at, dead_at, and the corrected partial index)
// is Step 2's responsibility and is deliberately not created here — Step 1
// does not touch those columns, so the current, already-live schema shape is
// sufficient to test it.
//
// Per ADR-025 §7/§21, events_outbox is a global table with no row-level
// security: tenant isolation for delivery is the relay's responsibility
// (restoring tenant context before dispatch, Step 3B), not a PostgreSQL
// policy on this table. TestOutboxWriter_Publish_TenantIdentityNotCrossContaminated
// below tests this table's actual invariant (correct tenant_id attribution
// per transaction) rather than an RLS policy this table does not and should
// not have at this step.
//
// Phase 2 Step 2 note: these tests apply the real, production
// events_outbox migration (events/outbox/migrations) directly via its
// exported SQLFS, rather than maintaining a second, hand-written copy of the
// schema — so this suite exercises the actual migration-defined table, not
// an approximation of it. Before Step 2 landed, this file defined its own
// eventsOutboxTestDDL constant as a placeholder (documented at the time as
// "not a substitute for Step 2"); that constant is removed now that the real
// migration exists.
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

	"awo.so/awo/driver"
	"awo.so/awo/events"
	"awo.so/awo/events/outbox"
	outboxmigrations "awo.so/awo/events/outbox/migrations"
	"awo.so/awo/runtime/tenant"
	testdb "awo.so/awo/testutil/db"
)

// readOutboxMigrationSQL reads the real, production events_outbox
// migration's up-direction SQL directly from its embedded source
// (events/outbox/migrations.SQLFS), so these tests exercise the actual
// migration-defined schema rather than a second, hand-maintained copy of it.
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

// setupOutboxWriterTest mirrors setupPoolTest (connection_pool_test.go) but
// additionally applies the real events_outbox migration.
func setupOutboxWriterTest(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)
	testdb.ApplySQL(t, pool, testEntityDDL)
	testdb.ApplySQL(t, pool, readOutboxMigrationSQL(t))
	return pool
}

func newOutboxEvent(tenantID uuid.UUID, payload []byte) events.DomainEvent {
	return events.DomainEvent{
		TenantID:   tenantID,
		Type:       events.EventCreated,
		EntityName: "test_entity",
		RecordID:   uuid.New(),
		Payload:    payload,
	}
}

func outboxRowCount(t *testing.T, pool *pgxpool.Pool, eventID uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, testdb.QueryRowSQL(t, pool,
		`SELECT COUNT(*) FROM events_outbox WHERE id = $1`, eventID).Scan(&n))
	return n
}

// ── Test A — Same transaction commit (with mid-transaction visibility proof) ──

// TestOutboxWriter_Publish_JoinsCallerTransaction_CommitMakesRowVisible
// proves OutboxWriter.Publish executes on the SAME transaction as the
// entity mutation, not a separate connection: the row must not be visible
// from an independent connection while the transaction is still open, and
// must become visible only once that transaction actually commits.
func TestOutboxWriter_Publish_JoinsCallerTransaction_CommitMakesRowVisible(t *testing.T) {
	pool := setupOutboxWriterTest(t)
	repo := newRepo(pool)
	writer := outbox.NewWriter(pool)
	concurrent := testdb.OpenConcurrentPool(t, pool)

	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	switchToAppRole(t, pool)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})

	eventID := uuid.New()
	probeReady := make(chan struct{})
	releaseTx := make(chan struct{})
	txResult := make(chan error, 1)

	go func() {
		txResult <- repo.WithTx(ctx, func(txCtx context.Context) error {
			// The "mutation" leg — proves outbox + mutation share one TX, not
			// just that the outbox write alone is transactional.
			if _, err := repo.Create(txCtx, driver.CreateInput{Data: map[string]any{"name": "with-outbox-event"}}); err != nil {
				return err
			}
			e := newOutboxEvent(tenantID, []byte(`{"amount":1}`))
			e.ID = eventID
			if err := writer.Publish(txCtx, e); err != nil {
				return err
			}
			close(probeReady)
			<-releaseTx // hold the transaction open until the test says so
			return nil
		})
	}()

	<-probeReady
	var midCount int
	require.NoError(t, concurrent.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM events_outbox WHERE id = $1`, eventID).Scan(&midCount))
	assert.Equal(t, 0, midCount,
		"the outbox row must not be visible from an independent connection while the mutation's transaction is still open — "+
			"visibility here would mean Publish used a separate connection/transaction, not the caller's")

	close(releaseTx)
	require.NoError(t, <-txResult, "the transaction must commit successfully")

	var afterCount int
	require.NoError(t, concurrent.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM events_outbox WHERE id = $1`, eventID).Scan(&afterCount))
	assert.Equal(t, 1, afterCount, "the outbox row must be visible once the transaction commits")
}

// ── Test B — Same transaction rollback ──────────────────────────────────────

func TestOutboxWriter_Publish_RollsBackWithTheMutation(t *testing.T) {
	pool := setupOutboxWriterTest(t)
	repo := newRepo(pool)
	writer := outbox.NewWriter(pool)

	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	switchToAppRole(t, pool)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})

	eventID := uuid.New()
	txErr := repo.WithTx(ctx, func(txCtx context.Context) error {
		if _, err := repo.Create(txCtx, driver.CreateInput{Data: map[string]any{"name": "should-roll-back"}}); err != nil {
			return err
		}
		e := newOutboxEvent(tenantID, []byte(`{"amount":1}`))
		e.ID = eventID
		if err := writer.Publish(txCtx, e); err != nil {
			return err
		}
		return assert.AnError // force rollback, simulating a failed request
	})
	require.Error(t, txErr, "the deliberate error must propagate")

	assert.Equal(t, 0, outboxRowCount(t, pool, eventID),
		"the outbox row must not exist after the surrounding transaction rolls back")

	var seen []map[string]any
	require.NoError(t, repo.WithTx(ctx, func(txCtx context.Context) error {
		records, _, err := repo.Query(txCtx, nil, driver.WithSkipCount())
		for _, r := range records {
			seen = append(seen, r.Data)
		}
		return err
	}))
	assert.Empty(t, seen, "the mutation must have rolled back together with the outbox row — same transaction, same fate")
}

// ── Test C — Payload preservation (flat object, no double-encoding) ─────────

func TestOutboxWriter_Publish_PreservesPayloadAsJSONObject_NotBase64String(t *testing.T) {
	pool := setupOutboxWriterTest(t)
	repo := newRepo(pool)
	writer := outbox.NewWriter(pool)

	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	switchToAppRole(t, pool)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})

	original := map[string]any{"entity": "invoice", "record_id": "123", "amount": 100}
	originalJSON, err := json.Marshal(original)
	require.NoError(t, err)

	eventID := uuid.New()
	require.NoError(t, repo.WithTx(ctx, func(txCtx context.Context) error {
		e := newOutboxEvent(tenantID, originalJSON)
		e.ID = eventID
		return writer.Publish(txCtx, e)
	}))

	// jsonb_typeof is the decisive check: a base64-double-encoded payload
	// (this defect's exact symptom) would report 'string', never 'object'.
	var typeofPayload string
	require.NoError(t, testdb.QueryRowSQL(t, pool,
		`SELECT jsonb_typeof(payload) FROM events_outbox WHERE id = $1`, eventID).Scan(&typeofPayload))
	require.Equal(t, "object", typeofPayload,
		"payload must be stored as a JSON object, not a JSON string — a 'string' result here is exactly "+
			"the base64-double-encoding defect this test exists to catch")

	var amount float64
	require.NoError(t, testdb.QueryRowSQL(t, pool,
		`SELECT (payload->>'amount')::float FROM events_outbox WHERE id = $1`, eventID).Scan(&amount))
	assert.Equal(t, float64(100), amount, "fields must be queryable and correctly valued via a JSON path expression")

	var entity string
	require.NoError(t, testdb.QueryRowSQL(t, pool,
		`SELECT payload->>'entity' FROM events_outbox WHERE id = $1`, eventID).Scan(&entity))
	assert.Equal(t, "invoice", entity)
}

// ── Test D — Payload round-trip (nested structure, structural comparison) ───

func TestOutboxWriter_Publish_NestedPayload_RoundTripsStructurally(t *testing.T) {
	pool := setupOutboxWriterTest(t)
	repo := newRepo(pool)
	writer := outbox.NewWriter(pool)

	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	switchToAppRole(t, pool)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})

	original := map[string]any{
		"entity": "invoice",
		"lines": []any{
			map[string]any{"sku": "A1", "qty": 2},
			map[string]any{"sku": "B2", "qty": 5},
		},
		"metadata": map[string]any{"source": "import", "flags": []any{"urgent", "reviewed"}},
	}
	originalJSON, err := json.Marshal(original)
	require.NoError(t, err)

	eventID := uuid.New()
	require.NoError(t, repo.WithTx(ctx, func(txCtx context.Context) error {
		e := newOutboxEvent(tenantID, originalJSON)
		e.ID = eventID
		return writer.Publish(txCtx, e)
	}))

	var stored []byte
	require.NoError(t, testdb.QueryRowSQL(t, pool,
		`SELECT payload FROM events_outbox WHERE id = $1`, eventID).Scan(&stored))

	// Structural comparison, not byte-string equality — PostgreSQL's jsonb
	// storage may reorder object keys or normalize number formatting, so
	// decode both sides and compare as Go values.
	var gotDecoded, wantDecoded any
	require.NoError(t, json.Unmarshal(stored, &gotDecoded))
	require.NoError(t, json.Unmarshal(originalJSON, &wantDecoded))
	assert.Equal(t, wantDecoded, gotDecoded,
		"the persisted payload must be structurally equivalent to the original JSON, "+
			"not a base64-reencoded or otherwise transformed representation")
}

// ── Test E — Tenant identity integrity ──────────────────────────────────────
//
// events_outbox has no RLS by design (ADR-025 §7/§21) — this test proves the
// actual invariant that applies at this table: publishing events for two
// different tenants, each inside its own transaction, never cross-attributes
// which tenant_id ends up on which row. Cross-tenant *query* isolation for
// delivery is the relay's job (Step 3B), not a policy on this table.

func TestOutboxWriter_Publish_TenantIdentityNotCrossContaminated(t *testing.T) {
	pool := setupOutboxWriterTest(t)
	repo := newRepo(pool)
	writer := outbox.NewWriter(pool)

	tenantA := testdb.CreateTenant(t, pool, "ACTIVE")
	tenantB := testdb.CreateTenant(t, pool, "ACTIVE")
	switchToAppRole(t, pool)

	eventA := uuid.New()
	ctxA := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantA})
	require.NoError(t, repo.WithTx(ctxA, func(txCtx context.Context) error {
		e := newOutboxEvent(tenantA, []byte(`{"who":"A"}`))
		e.ID = eventA
		return writer.Publish(txCtx, e)
	}))

	eventB := uuid.New()
	ctxB := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantB})
	require.NoError(t, repo.WithTx(ctxB, func(txCtx context.Context) error {
		e := newOutboxEvent(tenantB, []byte(`{"who":"B"}`))
		e.ID = eventB
		return writer.Publish(txCtx, e)
	}))

	var gotTenantA, gotTenantB uuid.UUID
	require.NoError(t, testdb.QueryRowSQL(t, pool,
		`SELECT tenant_id FROM events_outbox WHERE id = $1`, eventA).Scan(&gotTenantA))
	require.NoError(t, testdb.QueryRowSQL(t, pool,
		`SELECT tenant_id FROM events_outbox WHERE id = $1`, eventB).Scan(&gotTenantB))

	assert.Equal(t, tenantA, gotTenantA, "event A's row must carry tenant A's own id, not tenant B's")
	assert.Equal(t, tenantB, gotTenantB, "event B's row must carry tenant B's own id, not tenant A's")
	assert.NotEqual(t, gotTenantA, gotTenantB)
}

// ── Test F — Transaction / input failure handling ───────────────────────────

func TestOutboxWriter_Publish_NoActiveTransaction_ReturnsError(t *testing.T) {
	pool := setupOutboxWriterTest(t)
	writer := outbox.NewWriter(pool)

	e := newOutboxEvent(uuid.New(), []byte(`{"amount":1}`))
	err := writer.Publish(context.Background(), e)
	require.Error(t, err,
		"Publish must return an error when ctx carries no active transaction, "+
			"per events.Publisher's documented contract — not silently succeed via a bare pool connection")
}

func TestOutboxWriter_Publish_MalformedPayload_RejectedNotSilentlyStored(t *testing.T) {
	pool := setupOutboxWriterTest(t)
	repo := newRepo(pool)
	writer := outbox.NewWriter(pool)

	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	switchToAppRole(t, pool)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})

	eventID := uuid.New()
	err := repo.WithTx(ctx, func(txCtx context.Context) error {
		e := newOutboxEvent(tenantID, []byte(`{not valid json`))
		e.ID = eventID
		return writer.Publish(txCtx, e)
	})
	require.Error(t, err,
		"a non-nil, syntactically invalid JSON payload must be rejected, not silently stored as an "+
			"arbitrary string and not silently dropped")

	assert.Equal(t, 0, outboxRowCount(t, pool, eventID),
		"the surrounding transaction must have rolled back — no partial row from the rejected payload")
}

func TestOutboxWriter_Publish_NilPayload_StoresNullNotError(t *testing.T) {
	pool := setupOutboxWriterTest(t)
	repo := newRepo(pool)
	writer := outbox.NewWriter(pool)

	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	switchToAppRole(t, pool)
	ctx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantID})

	eventID := uuid.New()
	require.NoError(t, repo.WithTx(ctx, func(txCtx context.Context) error {
		e := newOutboxEvent(tenantID, nil)
		e.ID = eventID
		return writer.Publish(txCtx, e)
	}))

	var isNull bool
	require.NoError(t, testdb.QueryRowSQL(t, pool,
		`SELECT payload IS NULL FROM events_outbox WHERE id = $1`, eventID).Scan(&isNull))
	assert.True(t, isNull, "a nil Payload must store SQL NULL, not an error and not an empty-string JSON value")
}
