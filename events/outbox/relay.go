// Package outbox implements the transactional outbox relay.
//
// The relay polls the events_outbox table for eligible pending rows and
// delivers them to registered subscribers. At-least-once delivery is
// guaranteed:
//
//  1. Events are written inside the causing transaction (atomicity, see
//     OutboxWriter.Publish).
//  2. After commit, the relay claims the row and dispatches it.
//  3. On successful dispatch, the row is marked delivered.
//  4. On failure, the row is retried with exponential backoff
//     (next_attempt_at) up to MaxAttempts times. After MaxAttempts, the row
//     is moved to the terminal dead-letter state (dead_at) — it is never
//     deleted, and remains queryable for operator recovery.
//
// This package never claims exactly-once delivery and never introduces
// per-entity ordering guarantees (ADR-025 §9.5/§11) — a Subscriber may be
// invoked more than once for the same event (see claim's lease comment and
// deliver's doc comment for the exact crash window that produces this), and
// must be idempotent on its own terms.
//
// The relay is a background goroutine started by bootstrap. Multiple relay
// instances (or concurrent poll cycles) claim disjoint batches of rows via
// `SELECT ... FOR UPDATE SKIP LOCKED` plus a durable, schema-only lease
// (pushing next_attempt_at into the near future for the claimed rows, in the
// same short transaction that claims them) — no advisory lock or long-held
// transaction is used, and no external subscriber call ever runs while a
// database transaction or lock is held (see claim and processOne).
//
// Dependency: outbox → events, driver interfaces, pgx pool, tx (for
// transaction-bound writes — see OutboxWriter.Publish), runtime/tenant (for
// dispatch-time tenant-context restoration), internal/dberr + runtime (for
// tenant-failure classification, reusing Phase 1's existing mechanism).
package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"awo.so/awo/events"
	"awo.so/awo/internal/dberr"
	"awo.so/awo/runtime"
	"awo.so/awo/runtime/tenant"
	"awo.so/awo/tx"
)

const (
	// pollInterval is how often Start's loop checks for new outbox rows.
	pollInterval = 500 * time.Millisecond

	// MaxAttempts is the maximum number of dispatch attempts before a row
	// is moved to the terminal dead-letter state (dead_at set).
	MaxAttempts = 5

	// outboxTable is the database table holding pending events.
	outboxTable = "events_outbox"

	// claimBatchSize is the maximum number of rows one claim() call
	// reserves at a time.
	claimBatchSize = 50

	// maxLastErrorLen bounds how much of a dispatch/classification error's
	// text is persisted to last_error. Relay-originated errors never
	// contain payload data by construction; this is defense-in-depth
	// against a third-party Subscriber whose own error text happens to wrap
	// unexpectedly large content.
	maxLastErrorLen = 2000
)

// Process-wide tunables. These are package-level variables, not constants,
// specifically so tests can shrink them for fast, deterministic execution
// (e.g. a short DispatchTimeout to test the timeout path without waiting
// seconds). In production, set them once at startup before calling
// Start — they are not safe to mutate concurrently with a running relay.
var (
	// DispatchTimeout bounds a single event's external dispatch call
	// (Subscriber.HandleEvent). A hung or slow subscriber must not be able
	// to block the relay indefinitely or delay every other claimed event in
	// the same batch (ADR-025 §9.3). A timeout is treated as an ordinary
	// dispatch failure — it follows the same retry/dead-letter path as any
	// other error, never as a special case.
	DispatchTimeout = 10 * time.Second

	// BackoffBase and BackoffMaxDelay define the exponential retry schedule
	// (ADR-025 §9.4): delay = BackoffBase * 2^(attempt-1), capped at
	// BackoffMaxDelay. With the defaults below this produces 1s/2s/4s/8s/16s
	// for attempts 1-5 — attempt 5's own backoff is never actually scheduled
	// in practice, since a 5th failure (attempts reaching MaxAttempts)
	// dead-letters instead of scheduling a 6th attempt.
	BackoffBase     = 1 * time.Second
	BackoffMaxDelay = 5 * time.Minute
)

// leaseDuration returns how long a claimed row's next_attempt_at is pushed
// into the future for the duration of dispatch — a durable, crash-safe lease
// using only the existing next_attempt_at column (no new schema needed). It
// must safely exceed DispatchTimeout so a legitimately in-flight dispatch is
// never re-claimed by another poll cycle (this instance or another relay
// instance) before it has had a chance to finish and persist its own result.
func leaseDuration() time.Duration {
	return DispatchTimeout + 20*time.Second
}

// ComputeBackoff returns the exponential backoff delay for the given 1-based
// attempt number: BackoffBase * 2^(attempt-1), capped at BackoffMaxDelay and
// guarded against shift overflow for pathological attempt numbers. Exported
// as a pure, deterministic function so tests (and operators) can verify the
// retry schedule without needing a live database.
func ComputeBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	shift := attempt - 1
	const maxShift = 20 // 2^20 seconds (~12 days) already vastly exceeds any sane BackoffMaxDelay
	if shift > maxShift {
		shift = maxShift
	}
	delay := BackoffBase * time.Duration(int64(1)<<uint(shift))
	if delay <= 0 || delay > BackoffMaxDelay {
		delay = BackoffMaxDelay
	}
	return delay
}

// tenantOutcome classifies why restoring an event's tenant context failed,
// determining whether the event should be retried or dead-lettered
// (ADR-025 §16 row 15). This reuses Phase 1's existing dberr classification
// (tasks.md 1.1/1.1a) rather than inventing a new error taxonomy.
type tenantOutcome int

const (
	// tenantOutcomeTransient covers infrastructure errors (connection
	// failure, pool exhaustion, etc.) indistinguishable from any other
	// transient dispatch failure — retried with the same backoff schedule.
	tenantOutcomeTransient tenantOutcome = iota

	// tenantOutcomePermanent covers a tenant that will never become
	// dispatchable by retrying: it does not exist, or exists but is not
	// ACTIVE. Retrying wastes the backoff schedule on a condition that
	// cannot spontaneously resolve — dead-lettered immediately instead.
	tenantOutcomePermanent
)

// classifyTenantError translates a set_tenant_context() failure into a
// retry/dead-letter decision using internal/dberr — the exact mechanism
// platform/iam's Login path already uses for CodeTenantNotFound (P0001) and
// CodeTenantNotActive (P0002).
func classifyTenantError(err error) tenantOutcome {
	if isPermanentTenantFailure(err) {
		return tenantOutcomePermanent
	}
	return tenantOutcomeTransient
}

// isPermanentTenantFailure reports whether err (as returned by
// set_tenant_context()) indicates a tenant that will never become
// dispatchable by retrying — as opposed to a transient infrastructure
// failure. Uses internal/dberr's existing CodeTenantNotFound/
// CodeTenantNotActive translation (tasks.md 1.1/1.1a), not a new taxonomy.
func isPermanentTenantFailure(err error) bool {
	parsed := dberr.Parse(err, "outbox.relay: restore tenant context")
	var be *runtime.BusinessError
	if errors.As(parsed, &be) {
		switch be.Code {
		case "tenant.not_found", "tenant.not_active":
			return true
		}
	}
	return false
}

// IsPermanentTenantFailure exports isPermanentTenantFailure's classification
// for tests that need to verify the transient/permanent boundary without
// reproducing a live P0001/P0002 database error.
func IsPermanentTenantFailure(err error) bool {
	return isPermanentTenantFailure(err)
}

// Relay polls the outbox table and delivers events to subscribers.
type Relay struct {
	pool        *pgxpool.Pool
	subscribers map[events.EventType][]events.Subscriber
}

// New creates a Relay.
func New(pool *pgxpool.Pool) *Relay {
	return &Relay{
		pool:        pool,
		subscribers: make(map[events.EventType][]events.Subscriber),
	}
}

// Subscribe registers a subscriber. Empty eventType receives all events.
func (r *Relay) Subscribe(eventType events.EventType, h events.Subscriber) {
	r.subscribers[eventType] = append(r.subscribers[eventType], h)
}

// Start begins the polling loop. Blocks until ctx is cancelled. Each tick
// runs exactly one Poll cycle.
func (r *Relay) Start(ctx context.Context) error {
	slog.Info("outbox relay started", "poll_interval", pollInterval)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("outbox relay stopped")
			return nil
		case <-ticker.C:
			if err := r.Poll(ctx); err != nil {
				slog.Error("outbox relay poll error", "err", err)
				// Continue — transient errors should not stop the relay.
			}
		}
	}
}

// Poll runs exactly one claim-and-dispatch cycle: it reserves up to
// claimBatchSize eligible rows and attempts to dispatch each one, persisting
// the outcome of each attempt individually. It is the same cycle Start runs
// on a timer, exported so tests and operational tooling can trigger a
// deterministic sweep without waiting on pollInterval.
func (r *Relay) Poll(ctx context.Context) error {
	claimed, err := r.claim(ctx)
	if err != nil {
		return err
	}
	for _, ce := range claimed {
		r.processOne(ctx, ce)
	}
	return nil
}

// claimedEvent is one row reserved by claim(), carrying the post-increment
// attempts count claim() observed (used for backoff/dead-letter decisions)
// and whether its persisted tenant_id failed to parse.
type claimedEvent struct {
	events.DomainEvent
	attempts          int
	tenantIDMalformed bool
}

// claim reserves up to claimBatchSize eligible rows in one short
// transaction: `SELECT ... FOR UPDATE SKIP LOCKED` to claim without
// blocking on rows another concurrent claimer already holds, immediately
// followed, in the same transaction, by an UPDATE that increments attempts
// and pushes next_attempt_at into the future (the lease) for exactly the
// claimed rows — then commits. No external call happens inside this
// transaction, and no lock is held once claim returns: dispatch (processOne)
// always runs after this transaction has already committed.
//
// Eligibility matches ADR-025 §9.4/§16 exactly: delivered_at IS NULL AND
// dead_at IS NULL AND next_attempt_at <= now() — not the old flat
// "attempts < N" cutoff, which this replaces entirely.
//
// attempts increments here, at claim time — i.e. "a dispatch attempt was
// initiated" — not after a completed attempt. If the process crashes after
// this commits but before processOne persists an outcome, the lease expires
// and a later Poll call reclaims the row, incrementing attempts again for
// what is, from the row's own perspective, a second initiated attempt. This
// is an accepted, documented characteristic of at-least-once delivery with a
// lease-based claim (ADR-025 §11) — not a bug, and not something this
// package attempts to make exactly-once.
func (r *Relay) claim(ctx context.Context) ([]claimedEvent, error) {
	dbTx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("outbox.claim: begin: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = dbTx.Rollback(ctx)
		}
	}()

	rows, err := dbTx.Query(ctx, fmt.Sprintf(
		`SELECT id, tenant_id, type, entity_name, record_id, actor_id, system_actor, action_name, correlation_id, payload, occurred_at
		 FROM %s
		 WHERE delivered_at IS NULL AND dead_at IS NULL AND next_attempt_at <= now()
		 ORDER BY next_attempt_at ASC
		 LIMIT %d
		 FOR UPDATE SKIP LOCKED`,
		outboxTable, claimBatchSize,
	))
	if err != nil {
		return nil, fmt.Errorf("outbox.claim: query: %w", err)
	}

	var claimed []claimedEvent
	for rows.Next() {
		var e events.DomainEvent
		var tenantIDStr, recordIDStr, actorIDStr string
		var systemActor, actionName, correlationID *string
		var payload []byte

		if err := rows.Scan(
			&e.ID, &tenantIDStr, &e.Type, &e.EntityName,
			&recordIDStr, &actorIDStr, &systemActor, &actionName, &correlationID, &payload, &e.OccurredAt,
		); err != nil {
			rows.Close()
			return nil, fmt.Errorf("outbox.claim: scan: %w", err)
		}

		ce := claimedEvent{DomainEvent: e}
		// tenant_id is a native `uuid NOT NULL` column, so pgx always
		// yields a canonical, parseable string for it — a parse failure
		// here is structurally not expected to occur via any genuine data
		// path (ADR-025 §16 row 15's fourth branch), but is still checked
		// explicitly, not silently discarded, so a future bug or direct
		// database tampering cannot masquerade as a legitimate uuid.Nil
		// (platform-level) event.
		if parsed, perr := uuid.Parse(tenantIDStr); perr != nil {
			ce.tenantIDMalformed = true
		} else {
			ce.TenantID = parsed
		}
		ce.RecordID, _ = uuid.Parse(recordIDStr)
		ce.ActorID, _ = uuid.Parse(actorIDStr)
		if systemActor != nil {
			ce.SystemActor = *systemActor
		}
		if actionName != nil {
			ce.ActionName = *actionName
		}
		if correlationID != nil {
			ce.CorrelationID = *correlationID
		}
		ce.Payload = payload

		claimed = append(claimed, ce)
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		return nil, fmt.Errorf("outbox.claim: rows: %w", rowsErr)
	}

	if len(claimed) == 0 {
		if err := dbTx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("outbox.claim: commit (empty): %w", err)
		}
		committed = true
		return nil, nil
	}

	ids := make([]uuid.UUID, len(claimed))
	for i, ce := range claimed {
		ids[i] = ce.ID
	}

	leaseRows, err := dbTx.Query(ctx, fmt.Sprintf(
		`UPDATE %s
		 SET attempts = attempts + 1,
		     next_attempt_at = now() + make_interval(secs => $1)
		 WHERE id = ANY($2)
		 RETURNING id, attempts`,
		outboxTable,
	), leaseDuration().Seconds(), ids)
	if err != nil {
		return nil, fmt.Errorf("outbox.claim: lease update: %w", err)
	}
	attemptsByID := make(map[uuid.UUID]int, len(claimed))
	for leaseRows.Next() {
		var id uuid.UUID
		var attempts int
		if err := leaseRows.Scan(&id, &attempts); err != nil {
			leaseRows.Close()
			return nil, fmt.Errorf("outbox.claim: lease scan: %w", err)
		}
		attemptsByID[id] = attempts
	}
	leaseRowsErr := leaseRows.Err()
	leaseRows.Close()
	if leaseRowsErr != nil {
		return nil, fmt.Errorf("outbox.claim: lease rows: %w", leaseRowsErr)
	}

	if err := dbTx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("outbox.claim: commit: %w", err)
	}
	committed = true

	for i := range claimed {
		claimed[i].attempts = attemptsByID[claimed[i].ID]
	}
	return claimed, nil
}

// processOne dispatches one claimed event and persists the outcome. It never
// holds a database transaction or lock while calling a subscriber: claim's
// transaction has already committed by the time processOne runs, and each
// persistence call below (markDelivered/scheduleRetry/deadLetter) is its own
// short, independent statement issued after dispatch completes or times out.
//
// ctx is the relay's own ambient context (unbounded by the per-event
// timeout) — used for every persistence call, including on the timeout/
// failure path, so that recording an outcome is never itself defeated by the
// same timeout that caused the failure being recorded.
func (r *Relay) processOne(ctx context.Context, ce claimedEvent) {
	fields := []any{
		"event_id", ce.ID,
		"event_type", string(ce.Type),
		"entity_name", ce.EntityName,
		"tenant_id", ce.TenantID,
		"attempt", ce.attempts,
	}

	if ce.tenantIDMalformed {
		err := fmt.Errorf("event %s has a tenant_id that failed to parse", ce.ID)
		// Logged at Error with an explicit marker distinct from an ordinary
		// dead-letter: this indicates data corruption or an attempted
		// tenant-boundary bypass, not a routine operational failure
		// (ADR-025 §16 row 15) — intended to page an operator, not just be
		// visible in a dashboard.
		slog.Error("outbox: malformed tenant identity — operator attention required",
			append(fields, "page_operator", true, "err", err)...)
		r.deadLetter(ctx, ce.ID, err)
		return
	}

	var dispatchCtx context.Context
	if ce.TenantID == uuid.Nil {
		// A legitimate platform-level event (ADR-025 §7) — no per-tenant
		// validation applies. Use the same SystemContext convention
		// contrib/pgx already establishes for platform-level operations.
		dispatchCtx = tenant.SystemContext(ctx)
	} else {
		restored, err := r.restoreTenantContext(ctx, ce.TenantID)
		if err != nil {
			switch classifyTenantError(err) {
			case tenantOutcomePermanent:
				slog.Error("outbox: tenant permanently unavailable — dead-lettering, no backoff consumed",
					append(fields, "err", err)...)
				r.deadLetter(ctx, ce.ID, err)
			default:
				slog.Warn("outbox: transient failure restoring tenant context — retrying with backoff",
					append(fields, "err", err)...)
				r.scheduleRetryOrDeadLetter(ctx, ce, err, fields)
			}
			return
		}
		dispatchCtx = restored
	}

	dispatchCtx, cancel := context.WithTimeout(dispatchCtx, DispatchTimeout)
	defer cancel()

	if err := r.deliver(dispatchCtx, ce.DomainEvent); err != nil {
		slog.Warn("outbox: dispatch failed", append(fields, "err", err)...)
		r.scheduleRetryOrDeadLetter(ctx, ce, err, fields)
		return
	}

	r.markDelivered(ctx, ce.ID)
}

// scheduleRetryOrDeadLetter applies the shared attempts-vs-MaxAttempts
// decision used by both the tenant-transient-failure path and the ordinary
// dispatch-failure path (including a timeout, which is just an ordinary
// failure — see deliver's ctx.WithTimeout above).
func (r *Relay) scheduleRetryOrDeadLetter(ctx context.Context, ce claimedEvent, cause error, fields []any) {
	if ce.attempts >= MaxAttempts {
		slog.Error("outbox: maximum attempts reached — dead-lettering",
			append(fields, "max_attempts", MaxAttempts, "err", cause)...)
		r.deadLetter(ctx, ce.ID, cause)
		return
	}
	r.scheduleRetry(ctx, ce.ID, ce.attempts, cause)
}

// restoreTenantContext validates tenantID via the real, production
// set_tenant_context() function — the same one contrib/pgx.Repository.WithTx
// calls for every ordinary mutation — on a short-lived connection dedicated
// to this check, and returns a context carrying tenant.TenantContext for the
// dispatch call on success. This does not share a physical connection with
// whatever a Subscriber does on its own: a Subscriber that itself calls
// contrib/pgx.Repository.WithTx independently (and correctly) re-establishes
// tenant context on its own acquired connection, exactly as it already does
// for ordinary HTTP-triggered mutations. The relay must not rely on, and
// never does rely on, whatever tenant context happens to be ambient in the
// relay process — the event's own persisted tenant_id is authoritative.
func (r *Relay) restoreTenantContext(ctx context.Context, tenantID uuid.UUID) (context.Context, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return ctx, fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT set_tenant_context($1)", tenantID.String()); err != nil {
		return ctx, err
	}
	return tenant.WithContext(ctx, tenant.TenantContext{TenantID: tenantID}), nil
}

// deliver dispatches e to type-specific subscribers first, then wildcard
// subscribers. Duplicate dispatch is possible and tolerated (at-least-once,
// never exactly-once, ADR-025 §11): if the process crashes after a
// Subscriber's HandleEvent returns success here but before markDelivered's
// UPDATE commits, the row's next_attempt_at lease will eventually expire and
// a later Poll call will dispatch it again. Every Subscriber implementation
// must be idempotent on its own terms (ADR-025 §4.G).
func (r *Relay) deliver(ctx context.Context, e events.DomainEvent) error {
	for _, h := range r.subscribers[e.Type] {
		if err := h.HandleEvent(ctx, e); err != nil {
			return err
		}
	}
	for _, h := range r.subscribers[""] {
		if err := h.HandleEvent(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

// markDelivered persists successful dispatch. delivered_at is set only after
// deliver has already returned success — never before, and never on a
// timeout or error.
func (r *Relay) markDelivered(ctx context.Context, id uuid.UUID) {
	if _, err := r.pool.Exec(ctx, fmt.Sprintf(
		`UPDATE %s SET delivered_at = now(), last_error = NULL WHERE id = $1`, outboxTable,
	), id); err != nil {
		slog.Error("outbox: failed to persist delivered state — the event may be redispatched on the next cycle "+
			"(at-least-once, not a correctness bug)", "event_id", id, "err", err)
	}
}

// scheduleRetry persists a failed dispatch attempt that has not yet reached
// MaxAttempts: attempts was already incremented by claim(); this call only
// schedules the next attempt (next_attempt_at) and records last_error.
// delivered_at and dead_at are left untouched.
func (r *Relay) scheduleRetry(ctx context.Context, id uuid.UUID, attemptsAtClaim int, cause error) {
	delay := ComputeBackoff(attemptsAtClaim)
	if _, err := r.pool.Exec(ctx, fmt.Sprintf(
		`UPDATE %s SET next_attempt_at = now() + make_interval(secs => $1), last_error = $2 WHERE id = $3`,
		outboxTable,
	), delay.Seconds(), truncateErrorMessage(cause), id); err != nil {
		slog.Error("outbox: failed to persist retry schedule", "event_id", id, "err", err)
	}
}

// deadLetter persists the terminal failure state. The row is never deleted
// and remains excluded from future polling via events_outbox_pending's own
// `WHERE ... AND dead_at IS NULL` predicate (Step 2's migration) — it stays
// queryable for operator recovery (`SELECT * FROM events_outbox WHERE
// dead_at IS NOT NULL`).
func (r *Relay) deadLetter(ctx context.Context, id uuid.UUID, cause error) {
	if _, err := r.pool.Exec(ctx, fmt.Sprintf(
		`UPDATE %s SET dead_at = now(), last_error = $1 WHERE id = $2`, outboxTable,
	), truncateErrorMessage(cause), id); err != nil {
		slog.Error("outbox: failed to persist dead-letter state", "event_id", id, "err", err)
	}
}

// truncateErrorMessage bounds how much of a dispatch/classification error's
// text is persisted to last_error (see maxLastErrorLen). Never logs or
// stores e.Payload — only cause.Error()'s own text.
func truncateErrorMessage(cause error) string {
	s := cause.Error()
	if len(s) > maxLastErrorLen {
		return s[:maxLastErrorLen] + "... (truncated)"
	}
	return s
}

// OutboxWriter writes events to the outbox table inside the caller's transaction.
// Implements events.Publisher.
//
// pool is retained for construction-time compatibility with New/NewWriter's
// existing shared-pool wiring; Publish itself no longer uses it directly
// (see Publish's doc comment) — only Relay.claim needs pool access.
type OutboxWriter struct {
	pool *pgxpool.Pool
}

// NewWriter creates an OutboxWriter.
func NewWriter(pool *pgxpool.Pool) *OutboxWriter {
	return &OutboxWriter{pool: pool}
}

var _ events.Publisher = (*OutboxWriter)(nil)

// Publish writes e to the outbox within the transaction in ctx.
//
// The caller's context must carry an active transaction-bound connection —
// the same one contrib/pgx.Repository.WithTx embeds into ctx for every
// mutation. Publish resolves it via tx.QuerierFromContext and executes the
// INSERT through it, exactly as audit/pg_writer.go and
// audit/transactional_writer.go already do for the audit write — never by
// acquiring a separate connection from pool. If ctx carries no active
// transaction, Publish returns an error rather than silently succeeding via
// an auto-commit connection, per this method's events.Publisher contract.
//
// e.Payload must already be valid JSON (or nil/empty — a valid, omitted
// state per the field's own json:"payload,omitempty" contract). Publish
// stores the bytes verbatim; it does not re-encode them. A non-nil value
// that is not syntactically valid JSON is rejected rather than silently
// stored as an arbitrary string.
func (w *OutboxWriter) Publish(ctx context.Context, e events.DomainEvent) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}

	q, ok := tx.QuerierFromContext(ctx)
	if !ok {
		return fmt.Errorf("outbox.Publish: no active transaction in context")
	}

	var payloadArg any
	if len(e.Payload) > 0 {
		if !json.Valid(e.Payload) {
			return fmt.Errorf("outbox.Publish: payload is not valid JSON")
		}
		payloadArg = e.Payload
	}

	_, err := q.ExecSQL(ctx, fmt.Sprintf(
		`INSERT INTO %s (id, tenant_id, type, entity_name, record_id, actor_id, system_actor, action_name, correlation_id, payload, occurred_at, attempts)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 0)`,
		outboxTable,
	),
		e.ID, e.TenantID, string(e.Type), e.EntityName,
		e.RecordID, e.ActorID, nilIfEmpty(e.SystemActor), nilIfEmpty(e.ActionName), nilIfEmpty(e.CorrelationID), payloadArg, e.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("outbox.Publish: insert: %w", err)
	}
	return nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
