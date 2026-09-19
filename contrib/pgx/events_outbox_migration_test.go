package pgx_test

// Phase 2 Step 2 — events_outbox Migration Reconciliation.
//
// These tests exercise the real, production events_outbox migration
// (events/outbox/migrations, applied via readOutboxMigrationSQL in
// outbox_writer_transaction_test.go) directly against real PostgreSQL,
// proving the schema ADR-025 §7 requires actually exists, has the correct
// column types, retains the correct indexes/constraints for the retry/
// dead-letter lifecycle model, and remains the intentionally global,
// non-RLS table ADR-025 §7/§21 specifies.

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testdb "awo.so/awo/testutil/db"
)

type outboxColumn struct {
	dataType string
	nullable string
}

func outboxColumnTypes(t *testing.T, pool *pgxpool.Pool) map[string]outboxColumn {
	t.Helper()
	rows, err := testdb.QueryRowsSQL(t, pool, `
		SELECT column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_name = 'events_outbox' AND table_schema = current_schema()
	`)
	require.NoError(t, err)
	defer rows.Close()

	out := make(map[string]outboxColumn)
	for rows.Next() {
		var name, dtype, nullable string
		require.NoError(t, rows.Scan(&name, &dtype, &nullable))
		out[name] = outboxColumn{dataType: dtype, nullable: nullable}
	}
	require.NoError(t, rows.Err())
	return out
}

// ── Test A — Fresh schema ────────────────────────────────────────────────────

func TestEventsOutboxMigration_FreshSchema_CreatesTable(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, readOutboxMigrationSQL(t))

	assert.True(t, testdb.TableExists(t, pool, "events_outbox"),
		"the real events_outbox migration must create the table on a fresh database/schema")
}

// ── Test B — Required columns and PostgreSQL types ──────────────────────────

func TestEventsOutboxMigration_RequiredColumnsAndTypes(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, readOutboxMigrationSQL(t))

	cols := outboxColumnTypes(t, pool)

	want := map[string]string{
		"id":              "uuid",
		"tenant_id":       "uuid",
		"type":            "text",
		"entity_name":     "text",
		"record_id":       "uuid",
		"actor_id":        "uuid",
		"system_actor":    "text",
		"action_name":     "text",
		"correlation_id":  "text",
		"payload":         "jsonb",
		"occurred_at":     "timestamp with time zone",
		"delivered_at":    "timestamp with time zone",
		"attempts":        "integer",
		"next_attempt_at": "timestamp with time zone",
		"last_error":      "text",
		"dead_at":         "timestamp with time zone",
	}
	for col, wantType := range want {
		got, ok := cols[col]
		require.True(t, ok, "column %q must exist on the production events_outbox schema", col)
		assert.Equal(t, wantType, got.dataType, "column %q: unexpected PostgreSQL type", col)
	}

	// ADR-025 explicitly rejects renaming `type` to `event_type` — the
	// column name must match the live relay.go SELECT/INSERT statements
	// exactly, which this repository's Step 1 code already uses.
	_, hasEventType := cols["event_type"]
	assert.False(t, hasEventType, "the column must be named `type`, not `event_type`")

	// ADR-025 §12 explicitly rejects a deterministic idempotency-key column.
	_, hasIdemKey := cols["idempotency_key"]
	assert.False(t, hasIdemKey, "no deterministic idempotency-key column may exist (ADR-025 §12)")
	_, hasSystemActorID := cols["system_actor_id"]
	assert.False(t, hasSystemActorID, "the actor column is `system_actor` (text), never `system_actor_id`")

	assert.Equal(t, "NO", cols["tenant_id"].nullable, "tenant_id must be NOT NULL (ADR-025 §7 — uuid.Nil, not SQL NULL, represents platform-level events)")
	assert.Equal(t, "NO", cols["record_id"].nullable, "record_id must be NOT NULL (ADR-025 §7)")
	assert.Equal(t, "NO", cols["type"].nullable, "type must be NOT NULL")
	assert.Equal(t, "NO", cols["entity_name"].nullable, "entity_name must be NOT NULL")
	assert.Equal(t, "YES", cols["correlation_id"].nullable, "correlation_id must be nullable — an opaque, optional tracing identifier")
	assert.Equal(t, "YES", cols["system_actor"].nullable, "system_actor must be nullable — NULL for human/service-account actors")
	assert.Equal(t, "YES", cols["dead_at"].nullable, "dead_at must be nullable — NULL until moved to terminal failure")
	assert.Equal(t, "NO", cols["next_attempt_at"].nullable, "next_attempt_at must be NOT NULL (has a DEFAULT now())")
}

// ── Test C — Required constraints/indexes ───────────────────────────────────

func TestEventsOutboxMigration_PendingIndex_MatchesRetryDeadLetterModel(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, readOutboxMigrationSQL(t))

	var indexdef string
	require.NoError(t, testdb.QueryRowSQL(t, pool, `
		SELECT indexdef FROM pg_indexes
		WHERE indexname = 'events_outbox_pending' AND schemaname = current_schema()
	`).Scan(&indexdef))

	assert.Contains(t, indexdef, "next_attempt_at",
		"the pending index must be usable for next_attempt_at-scheduled retry lookup (ADR-025 §9.4)")
	assert.Contains(t, indexdef, "delivered_at IS NULL",
		"the pending predicate must exclude already-delivered rows")
	assert.Contains(t, indexdef, "dead_at IS NULL",
		"the pending predicate must exclude dead-lettered rows — an index still predicated on the retired "+
			"attempts < 5 cutoff would silently reintroduce the poison-event gap ADR-025 §9.4/§22 closes")
	assert.NotContains(t, indexdef, "attempts",
		"the index predicate must not reference the retired flat attempts < 5 cutoff from the orphaned migration")

	var pkCount int
	require.NoError(t, testdb.QueryRowSQL(t, pool, `
		SELECT COUNT(*) FROM pg_constraint con
		JOIN pg_class t ON t.oid = con.conrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE t.relname = 'events_outbox' AND n.nspname = current_schema() AND con.contype = 'p'
	`).Scan(&pkCount))
	assert.Equal(t, 1, pkCount, "events_outbox must have exactly one primary key constraint (on id)")
}

// ── Test D — Global table semantics (intentionally no RLS) ──────────────────

func TestEventsOutboxMigration_GlobalTable_IntentionallyNoRLS(t *testing.T) {
	pool := testdb.SetupTestDB(t)
	testdb.ApplySQL(t, pool, readOutboxMigrationSQL(t))

	var rlsEnabled, rlsForced bool
	require.NoError(t, testdb.QueryRowSQL(t, pool, `
		SELECT c.relrowsecurity, c.relforcerowsecurity
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relname = 'events_outbox' AND n.nspname = current_schema()
	`).Scan(&rlsEnabled, &rlsForced))

	// This is a deliberate architectural decision (ADR-025 §7/§21), not a
	// security gap: tenant_id on this table is event data restored by the
	// relay before dispatch (Step 3B), not a database-enforced access
	// boundary. Tenant isolation for mutation data continues to be enforced
	// by RLS elsewhere (contrib/pgx, per-entity tables) — this test does not
	// assert or imply that boundary is weakened; it asserts only that THIS
	// table's own, intentional design is actually what is deployed.
	assert.False(t, rlsEnabled,
		"events_outbox is intentionally a GLOBAL table with no row-level security (ADR-025 §7/§21) — "+
			"asserting true here would mean the deployed schema contradicts the authoritative ADR")
	assert.False(t, rlsForced)

	var policyCount int
	require.NoError(t, testdb.QueryRowSQL(t, pool, `
		SELECT COUNT(*) FROM pg_policies WHERE tablename = 'events_outbox' AND schemaname = current_schema()
	`).Scan(&policyCount))
	assert.Equal(t, 0, policyCount, "no RLS policy should exist on events_outbox")
}
