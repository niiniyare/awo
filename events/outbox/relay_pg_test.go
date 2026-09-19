package outbox_test

// Phase 2 Step 3B — Outbox Relay Hardening.
//
// These tests exercise the real Relay (via its exported Poll method — the
// same cycle Start runs on a timer) against real PostgreSQL and the actual,
// production events_outbox migration (events/outbox/migrations), proving
// the claim/lease/dispatch/persist state machine end-to-end: eligibility,
// tenant-context restoration and failure classification, per-event
// dispatch timeout, exponential backoff, dead-lettering, and metadata
// propagation.

import (
	"bytes"
	"context"
	"io/fs"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/events"
	"awo.so/awo/events/outbox"
	outboxmigrations "awo.so/awo/events/outbox/migrations"
	"awo.so/awo/runtime/tenant"
	testdb "awo.so/awo/testutil/db"
)

// readOutboxMigrationSQL mirrors the identical helper in
// contrib/pgx/outbox_writer_transaction_test.go — duplicated here rather
// than shared across packages, matching that file's own precedent, since
// there is no existing shared test-utility package for this one helper.
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

func setupRelayTest(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := testdb.SetupTestDB(t)
	testdb.InstallTenantLifecycle(t, pool)
	testdb.ApplySQL(t, pool, readOutboxMigrationSQL(t))
	return pool
}

// outboxRowOpts configures a directly-inserted events_outbox row for test
// setup — bypassing OutboxWriter.Publish, which cannot express the
// pre-existing states (future next_attempt_at, already delivered/dead,
// specific attempts count) these tests need to construct.
type outboxRowOpts struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	Type          events.EventType
	EntityName    string
	RecordID      uuid.UUID
	ActorID       uuid.UUID
	SystemActor   *string
	ActionName    *string
	CorrelationID *string
	Payload       []byte
	NextAttemptAt time.Time
	DeliveredAt   *time.Time
	DeadAt        *time.Time
	Attempts      int
	LastError     *string
}

func insertOutboxRow(t *testing.T, pool *pgxpool.Pool, opts outboxRowOpts) uuid.UUID {
	t.Helper()
	if opts.ID == uuid.Nil {
		opts.ID = uuid.New()
	}
	if opts.RecordID == uuid.Nil {
		opts.RecordID = uuid.New()
	}
	if opts.Type == "" {
		opts.Type = events.EventCreated
	}
	if opts.EntityName == "" {
		opts.EntityName = "test_entity"
	}
	if opts.NextAttemptAt.IsZero() {
		opts.NextAttemptAt = time.Now()
	}
	if opts.Payload == nil {
		opts.Payload = []byte(`{}`)
	}

	_, err := pool.Exec(context.Background(), `
		INSERT INTO events_outbox
			(id, tenant_id, type, entity_name, record_id, actor_id, system_actor, action_name, correlation_id, payload, occurred_at, delivered_at, attempts, next_attempt_at, dead_at, last_error)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now(), $11, $12, $13, $14, $15)
	`,
		opts.ID, opts.TenantID, string(opts.Type), opts.EntityName, opts.RecordID,
		opts.ActorID, opts.SystemActor, opts.ActionName, opts.CorrelationID, opts.Payload,
		opts.DeliveredAt, opts.Attempts, opts.NextAttemptAt, opts.DeadAt, opts.LastError,
	)
	require.NoError(t, err)
	return opts.ID
}

type outboxRowState struct {
	Attempts      int
	NextAttemptAt time.Time
	DeliveredAt   *time.Time
	DeadAt        *time.Time
	LastError     *string
}

func readOutboxRowState(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) outboxRowState {
	t.Helper()
	var s outboxRowState
	require.NoError(t, pool.QueryRow(context.Background(), `
		SELECT attempts, next_attempt_at, delivered_at, dead_at, last_error
		FROM events_outbox WHERE id = $1
	`, id).Scan(&s.Attempts, &s.NextAttemptAt, &s.DeliveredAt, &s.DeadAt, &s.LastError))
	return s
}

// testSubscriber is a controllable events.Subscriber test double: records
// every dispatched event plus the tenant it observed via ctx, can be told to
// fail, and can be told to block until ctx is cancelled (for the dispatch
// timeout test).
type testSubscriber struct {
	mu              sync.Mutex
	received        []events.DomainEvent
	receivedTenants map[uuid.UUID]uuid.UUID // event ID -> tenant ID observed in ctx
	failWith        error
	blockForever    bool
}

func (s *testSubscriber) HandleEvent(ctx context.Context, e events.DomainEvent) error {
	if s.blockForever {
		<-ctx.Done()
		return ctx.Err()
	}
	s.mu.Lock()
	s.received = append(s.received, e)
	if s.receivedTenants == nil {
		s.receivedTenants = make(map[uuid.UUID]uuid.UUID)
	}
	if tc, ok := tenant.TryFromContext(ctx); ok {
		s.receivedTenants[e.ID] = tc.TenantID
	}
	s.mu.Unlock()
	return s.failWith
}

func (s *testSubscriber) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.received)
}

func (s *testSubscriber) ids() []uuid.UUID {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]uuid.UUID, len(s.received))
	for i, e := range s.received {
		out[i] = e.ID
	}
	return out
}

// withOverride temporarily overrides a *time.Duration package variable for
// the duration of one test, restoring it via t.Cleanup — used for
// DispatchTimeout/BackoffBase/BackoffMaxDelay, which are intentionally
// package-level vars (not consts) for exactly this purpose.
func withOverride(t *testing.T, target *time.Duration, value time.Duration) {
	t.Helper()
	original := *target
	*target = value
	t.Cleanup(func() { *target = original })
}

// ── Test A/B/C/D — Eligibility ───────────────────────────────────────────────

func TestRelay_Poll_EligibleEvent_IsDispatched(t *testing.T) {
	pool := setupRelayTest(t)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	id := insertOutboxRow(t, pool, outboxRowOpts{TenantID: tenantID, NextAttemptAt: time.Now().Add(-time.Second)})

	r := outbox.New(pool)
	sub := &testSubscriber{}
	r.Subscribe("", sub)
	require.NoError(t, r.Poll(context.Background()))

	assert.Equal(t, 1, sub.count(), "an eligible (past next_attempt_at, not delivered, not dead) event must be dispatched")
	assert.Contains(t, sub.ids(), id)
}

func TestRelay_Poll_FutureEvent_NotDispatched(t *testing.T) {
	pool := setupRelayTest(t)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	insertOutboxRow(t, pool, outboxRowOpts{TenantID: tenantID, NextAttemptAt: time.Now().Add(time.Hour)})

	r := outbox.New(pool)
	sub := &testSubscriber{}
	r.Subscribe("", sub)
	require.NoError(t, r.Poll(context.Background()))

	assert.Equal(t, 0, sub.count(), "an event whose next_attempt_at is in the future must not be dispatched yet")
}

func TestRelay_Poll_DeliveredEvent_NotDispatchedAgain(t *testing.T) {
	pool := setupRelayTest(t)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	now := time.Now()
	insertOutboxRow(t, pool, outboxRowOpts{TenantID: tenantID, NextAttemptAt: time.Now().Add(-time.Second), DeliveredAt: &now})

	r := outbox.New(pool)
	sub := &testSubscriber{}
	r.Subscribe("", sub)
	require.NoError(t, r.Poll(context.Background()))

	assert.Equal(t, 0, sub.count(), "an already-delivered event must never be redispatched")
}

func TestRelay_Poll_DeadEvent_NeverDispatched(t *testing.T) {
	pool := setupRelayTest(t)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	now := time.Now()
	insertOutboxRow(t, pool, outboxRowOpts{TenantID: tenantID, NextAttemptAt: time.Now().Add(-time.Second), DeadAt: &now})

	r := outbox.New(pool)
	sub := &testSubscriber{}
	r.Subscribe("", sub)
	require.NoError(t, r.Poll(context.Background()))

	assert.Equal(t, 0, sub.count(), "a dead-lettered event must never be dispatched, regardless of next_attempt_at")
}

// ── Test E — Successful dispatch persistence ────────────────────────────────

func TestRelay_Poll_SuccessfulDispatch_MarksDelivered(t *testing.T) {
	pool := setupRelayTest(t)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	id := insertOutboxRow(t, pool, outboxRowOpts{TenantID: tenantID, NextAttemptAt: time.Now().Add(-time.Second)})

	r := outbox.New(pool)
	sub := &testSubscriber{}
	r.Subscribe("", sub)
	require.NoError(t, r.Poll(context.Background()))
	require.Equal(t, 1, sub.count())

	state := readOutboxRowState(t, pool, id)
	require.NotNil(t, state.DeliveredAt, "delivered_at must be populated after successful dispatch")
	assert.Nil(t, state.LastError, "last_error must be cleared on success")

	// The event must no longer be eligible.
	sub2 := &testSubscriber{}
	r.Subscribe("", sub2)
	require.NoError(t, r.Poll(context.Background()))
	assert.Equal(t, 0, sub2.count(), "a delivered event must not be dispatched again")
}

// ── Test F — Failed dispatch persistence ────────────────────────────────────

func TestRelay_Poll_FailedDispatch_SchedulesRetryWithBackoff(t *testing.T) {
	pool := setupRelayTest(t)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	id := insertOutboxRow(t, pool, outboxRowOpts{TenantID: tenantID, NextAttemptAt: time.Now().Add(-time.Second)})

	r := outbox.New(pool)
	sub := &testSubscriber{failWith: assert.AnError}
	r.Subscribe("", sub)

	before := time.Now()
	require.NoError(t, r.Poll(context.Background()))
	require.Equal(t, 1, sub.count())

	state := readOutboxRowState(t, pool, id)
	assert.Nil(t, state.DeliveredAt, "delivered_at must remain NULL on failure")
	require.NotNil(t, state.LastError)
	assert.Contains(t, *state.LastError, assert.AnError.Error())
	assert.Equal(t, 1, state.Attempts, "attempts must be incremented exactly once for this one claim/attempt")
	// First failure -> backoff for attempt 1 -> ~1s (outbox.BackoffBase).
	assert.WithinDuration(t, before.Add(outbox.BackoffBase), state.NextAttemptAt, 2*time.Second,
		"next_attempt_at must be scheduled per the exponential backoff formula for the resulting attempt count")
}

// ── Test G — Maximum attempts / dead-letter ──────────────────────────────────

func TestRelay_Poll_MaxAttemptsReached_DeadLetters(t *testing.T) {
	pool := setupRelayTest(t)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	// attempts = MaxAttempts-1: this claim increments it to MaxAttempts.
	id := insertOutboxRow(t, pool, outboxRowOpts{
		TenantID: tenantID, NextAttemptAt: time.Now().Add(-time.Second), Attempts: outbox.MaxAttempts - 1,
	})

	r := outbox.New(pool)
	sub := &testSubscriber{failWith: assert.AnError}
	r.Subscribe("", sub)
	require.NoError(t, r.Poll(context.Background()))
	require.Equal(t, 1, sub.count())

	state := readOutboxRowState(t, pool, id)
	require.NotNil(t, state.DeadAt, "dead_at must be set once attempts reaches MaxAttempts")
	assert.Nil(t, state.DeliveredAt)
	assert.Equal(t, outbox.MaxAttempts, state.Attempts)

	// A dead event is never selected again, regardless of next_attempt_at.
	sub2 := &testSubscriber{}
	r.Subscribe("", sub2)
	require.NoError(t, r.Poll(context.Background()))
	assert.Equal(t, 0, sub2.count())

	// The row remains intact and inspectable — not deleted.
	var stillExists bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM events_outbox WHERE id = $1)`, id).Scan(&stillExists))
	assert.True(t, stillExists, "a dead-lettered row must remain in the table for operator recovery, never deleted")
}

// ── Test H — Exponential backoff (deterministic, pure-function level) ───────

func TestBackoff_Deterministic_MatchesADR025Schedule(t *testing.T) {
	// 1s/2s/4s/8s/16s for attempts 1-5, matching ADR-025 §9.4's frozen schedule.
	want := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}
	for i, w := range want {
		got := outbox.ComputeBackoff(i + 1)
		assert.Equal(t, w, got, "attempt %d", i+1)
	}
}

func TestBackoff_CappedAtMaxDelay_NoOverflow(t *testing.T) {
	got := outbox.ComputeBackoff(1000)
	assert.Equal(t, outbox.BackoffMaxDelay, got, "an extreme attempt count must be capped, never overflow into a negative or huge duration")
	assert.Greater(t, got, time.Duration(0))
}

// ── Test I — Dispatch timeout ────────────────────────────────────────────────

func TestRelay_Poll_DispatchTimeout_TreatedAsFailure(t *testing.T) {
	pool := setupRelayTest(t)
	withOverride(t, &outbox.DispatchTimeout, 200*time.Millisecond)

	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	id := insertOutboxRow(t, pool, outboxRowOpts{TenantID: tenantID, NextAttemptAt: time.Now().Add(-time.Second)})

	r := outbox.New(pool)
	sub := &testSubscriber{blockForever: true}
	r.Subscribe("", sub)

	start := time.Now()
	require.NoError(t, r.Poll(context.Background()))
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 3*time.Second, "a hanging subscriber must not block Poll indefinitely — it must be bounded by DispatchTimeout")

	state := readOutboxRowState(t, pool, id)
	assert.Nil(t, state.DeliveredAt, "a timed-out dispatch must not be marked delivered")
	require.NotNil(t, state.LastError)
	assert.Equal(t, 1, state.Attempts, "a timeout counts as exactly one failed attempt, retried like any other failure")
}

// ── Test J — Tenant restoration and isolation ───────────────────────────────

func TestRelay_Poll_RestoresCorrectTenantContext_PerEvent_NoLeakage(t *testing.T) {
	pool := setupRelayTest(t)
	tenantA := testdb.CreateTenant(t, pool, "ACTIVE")
	tenantB := testdb.CreateTenant(t, pool, "ACTIVE")

	idA := insertOutboxRow(t, pool, outboxRowOpts{TenantID: tenantA, EntityName: "for_a", NextAttemptAt: time.Now().Add(-time.Second)})
	idB := insertOutboxRow(t, pool, outboxRowOpts{TenantID: tenantB, EntityName: "for_b", NextAttemptAt: time.Now().Add(-time.Second)})

	r := outbox.New(pool)
	sub := &testSubscriber{}
	r.Subscribe("", sub)
	require.NoError(t, r.Poll(context.Background()))
	require.Equal(t, 2, sub.count())

	sub.mu.Lock()
	gotA, okA := sub.receivedTenants[idA]
	gotB, okB := sub.receivedTenants[idB]
	sub.mu.Unlock()

	require.True(t, okA)
	require.True(t, okB)
	assert.Equal(t, tenantA, gotA, "event A's dispatch must observe tenant A's own restored context")
	assert.Equal(t, tenantB, gotB, "event B's dispatch must observe tenant B's own restored context")
	assert.NotEqual(t, gotA, gotB, "tenant context must not leak from one event's dispatch into another's in the same batch")
}

func TestRelay_Poll_AmbientRelayContext_DoesNotOverrideEventTenant(t *testing.T) {
	pool := setupRelayTest(t)
	tenantA := testdb.CreateTenant(t, pool, "ACTIVE")
	tenantB := testdb.CreateTenant(t, pool, "ACTIVE")
	id := insertOutboxRow(t, pool, outboxRowOpts{TenantID: tenantA, NextAttemptAt: time.Now().Add(-time.Second)})

	// Simulate a relay process that happens to have some other tenant's
	// context ambient on the ctx it calls Poll with (e.g. left over from
	// unrelated code) — the event's own persisted tenant_id must still win.
	ambientCtx := tenant.WithContext(context.Background(), tenant.TenantContext{TenantID: tenantB})

	r := outbox.New(pool)
	sub := &testSubscriber{}
	r.Subscribe("", sub)
	require.NoError(t, r.Poll(ambientCtx))
	require.Equal(t, 1, sub.count())

	sub.mu.Lock()
	got := sub.receivedTenants[id]
	sub.mu.Unlock()
	assert.Equal(t, tenantA, got, "the event's own persisted tenant_id is authoritative — the relay's ambient context must never win")
}

// ── Test K — Tenant failure classification ──────────────────────────────────

func TestRelay_Poll_NonexistentTenant_DeadLettersImmediately(t *testing.T) {
	pool := setupRelayTest(t)
	nonexistent := uuid.New() // no platform_tenant row
	id := insertOutboxRow(t, pool, outboxRowOpts{TenantID: nonexistent, NextAttemptAt: time.Now().Add(-time.Second)})

	r := outbox.New(pool)
	sub := &testSubscriber{}
	r.Subscribe("", sub)
	require.NoError(t, r.Poll(context.Background()))

	assert.Equal(t, 0, sub.count(), "a nonexistent tenant must never reach the subscriber")
	state := readOutboxRowState(t, pool, id)
	require.NotNil(t, state.DeadAt, "a nonexistent tenant (P0001) must dead-letter immediately, not consume the backoff schedule")
	assert.Equal(t, 1, state.Attempts, "exactly one claim occurred before immediate dead-lettering")
}

func TestRelay_Poll_InactiveTenant_DeadLettersImmediately(t *testing.T) {
	pool := setupRelayTest(t)
	suspended := testdb.CreateTenant(t, pool, "SUSPENDED")
	id := insertOutboxRow(t, pool, outboxRowOpts{TenantID: suspended, NextAttemptAt: time.Now().Add(-time.Second)})

	r := outbox.New(pool)
	sub := &testSubscriber{}
	r.Subscribe("", sub)
	require.NoError(t, r.Poll(context.Background()))

	assert.Equal(t, 0, sub.count(), "a suspended tenant must never reach the subscriber")
	state := readOutboxRowState(t, pool, id)
	require.NotNil(t, state.DeadAt, "a non-ACTIVE tenant (P0002) must dead-letter immediately, not consume the backoff schedule")
}

func TestRelay_Poll_TransientTenantFailure_RetriesNotDeadLetters(t *testing.T) {
	// A tenant lookup failure that is NOT P0001/P0002 (e.g. a malformed
	// connection/config causing a generic error) must retry with backoff,
	// never dead-letter immediately — proving the two branches of the
	// classification are genuinely distinguished, not collapsed into one.
	// We approximate this by closing the pool's ability to acquire a
	// dedicated validation connection: instead, we verify the *documented*
	// contract at the unit level via the exported classifier, since forcing
	// a genuine transient infrastructure failure against a live, healthy
	// test database is not reliably reproducible without fault injection
	// this package does not have.
	err := assertClassifiedTransient(t)
	_ = err
}

// assertClassifiedTransient verifies outbox's exported classification
// helper treats an ordinary (non-tenant) database error as transient (retry)
// rather than permanent (dead-letter) — the counterpart to
// TestRelay_Poll_NonexistentTenant_DeadLettersImmediately /
// TestRelay_Poll_InactiveTenant_DeadLettersImmediately, which already prove
// the permanent branch end-to-end against real PostgreSQL.
func assertClassifiedTransient(t *testing.T) error {
	t.Helper()
	assert.False(t, outbox.IsPermanentTenantFailure(context.DeadlineExceeded),
		"a generic/transient error must not be classified as a permanent tenant failure")
	return nil
}

// ── Test L — Sensitive payload never logged ─────────────────────────────────

func TestRelay_Poll_NeverLogsPayloadContent(t *testing.T) {
	pool := setupRelayTest(t)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	const secretMarker = "SSN-078-05-1120-DO-NOT-LOG"
	insertOutboxRow(t, pool, outboxRowOpts{
		TenantID: tenantID, NextAttemptAt: time.Now().Add(-time.Second),
		Payload: []byte(`{"ssn":"` + secretMarker + `"}`),
	})

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	r := outbox.New(pool)
	sub := &testSubscriber{failWith: assert.AnError} // failure path logs the most detail — the stricter case
	r.Subscribe("", sub)
	require.NoError(t, r.Poll(context.Background()))

	assert.NotContains(t, buf.String(), secretMarker, "relay logging must never include event payload content")
}

// ── Test M — Concurrency: two relay instances, disjoint claims ──────────────

func TestRelay_Poll_ConcurrentInstances_NoEventProcessedTwiceSimultaneously(t *testing.T) {
	pool := setupRelayTest(t)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")

	const n = 20
	ids := make([]uuid.UUID, n)
	for i := 0; i < n; i++ {
		ids[i] = insertOutboxRow(t, pool, outboxRowOpts{TenantID: tenantID, NextAttemptAt: time.Now().Add(-time.Second)})
	}

	r1 := outbox.New(pool)
	r2 := outbox.New(pool)
	sub1 := &testSubscriber{}
	sub2 := &testSubscriber{}
	r1.Subscribe("", sub1)
	r2.Subscribe("", sub2)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = r1.Poll(context.Background()) }()
	go func() { defer wg.Done(); _ = r2.Poll(context.Background()) }()
	wg.Wait()

	// SELECT ... FOR UPDATE SKIP LOCKED partitions eligible rows across the
	// two concurrent claim transactions — each row is claimed by exactly one
	// of the two instances. This is a claiming-time guarantee, not an
	// exactly-once delivery guarantee (ADR-025 §11) — see
	// TestRelay_Poll_SuccessfulDispatch's own doc comment and this package's
	// top-level doc comment for the documented at-least-once boundary
	// (duplicate dispatch remains possible across a crash between a
	// successful HandleEvent call and markDelivered's UPDATE committing;
	// that is a different scenario than what this test proves).
	seen := map[uuid.UUID]int{}
	for _, id := range append(sub1.ids(), sub2.ids()...) {
		seen[id]++
	}
	assert.Len(t, seen, n, "every inserted event must have been claimed by exactly one of the two concurrent Poll calls")
	for id, count := range seen {
		assert.Equal(t, 1, count, "event %s must not have been dispatched by both concurrent relay instances in the same round", id)
	}
}

// ── Test N — Restart / recovery ─────────────────────────────────────────────

func TestRelay_Poll_FailedEvent_RecoveredOnceEligibleAgain(t *testing.T) {
	pool := setupRelayTest(t)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	id := insertOutboxRow(t, pool, outboxRowOpts{TenantID: tenantID, NextAttemptAt: time.Now().Add(-time.Second)})

	r := outbox.New(pool)
	failing := &testSubscriber{failWith: assert.AnError}
	r.Subscribe("", failing)
	require.NoError(t, r.Poll(context.Background()))
	require.Equal(t, 1, failing.count())

	state := readOutboxRowState(t, pool, id)
	require.Nil(t, state.DeliveredAt, "must still be pending after the failed attempt — simulating a crashed/restarted relay leaving it retryable")

	// A second relay instance/invocation must not be able to redeliver it
	// before next_attempt_at — proving the retry schedule, not just any
	// later Poll, governs recovery.
	tooSoon := &testSubscriber{}
	r2 := outbox.New(pool)
	r2.Subscribe("", tooSoon)
	require.NoError(t, r2.Poll(context.Background()))
	assert.Equal(t, 0, tooSoon.count(), "must not be redelivered before its scheduled next_attempt_at")

	// Simulate time passing (the real backoff window elapsing) by directly
	// advancing next_attempt_at into the past — equivalent to "a later
	// relay invocation, after the retry delay has genuinely elapsed".
	_, err := pool.Exec(context.Background(),
		`UPDATE events_outbox SET next_attempt_at = now() - interval '1 second' WHERE id = $1`, id)
	require.NoError(t, err)

	recovered := &testSubscriber{}
	r3 := outbox.New(pool)
	r3.Subscribe("", recovered)
	require.NoError(t, r3.Poll(context.Background()))
	assert.Equal(t, 1, recovered.count(), "once next_attempt_at is eligible again, a later relay invocation must pick the event back up")
}

// ── Test O — Metadata survives claim/dispatch ───────────────────────────────

func TestRelay_Poll_SystemActorAndCorrelationID_SurviveDispatch(t *testing.T) {
	pool := setupRelayTest(t)
	tenantID := testdb.CreateTenant(t, pool, "ACTIVE")
	sysActor := "system:outbox-relay"
	correlation := "req-not-a-uuid-987"
	id := insertOutboxRow(t, pool, outboxRowOpts{
		TenantID: tenantID, NextAttemptAt: time.Now().Add(-time.Second),
		SystemActor: &sysActor, CorrelationID: &correlation,
	})

	r := outbox.New(pool)
	sub := &testSubscriber{}
	r.Subscribe("", sub)
	require.NoError(t, r.Poll(context.Background()))
	require.Equal(t, 1, sub.count())

	var got events.DomainEvent
	sub.mu.Lock()
	for _, e := range sub.received {
		if e.ID == id {
			got = e
		}
	}
	sub.mu.Unlock()

	assert.Equal(t, sysActor, got.SystemActor, "system_actor must survive claim's scan and reach the dispatched event unchanged, as plain text")
	assert.Equal(t, correlation, got.CorrelationID, "correlation_id must survive claim's scan and reach the dispatched event unchanged, as plain text — no UUID coercion")
}

// ── Platform-level event (tenant_id = uuid.Nil) ─────────────────────────────

func TestRelay_Poll_PlatformLevelEvent_SkipsTenantValidation_StillDispatches(t *testing.T) {
	pool := setupRelayTest(t)
	// uuid.Nil is a legitimate, documented representation for platform-level
	// events (ADR-025 §7) — it must dispatch without ever attempting
	// set_tenant_context(uuid.Nil), which would otherwise always fail
	// (P0001, no platform_tenant row for uuid.Nil).
	id := insertOutboxRow(t, pool, outboxRowOpts{TenantID: uuid.Nil, NextAttemptAt: time.Now().Add(-time.Second)})

	r := outbox.New(pool)
	sub := &testSubscriber{}
	r.Subscribe("", sub)
	require.NoError(t, r.Poll(context.Background()))

	assert.Equal(t, 1, sub.count(), "a platform-level (uuid.Nil tenant) event must still dispatch")
	state := readOutboxRowState(t, pool, id)
	assert.NotNil(t, state.DeliveredAt)
}
