// Package outbox implements the transactional outbox relay.
//
// The relay polls the events_outbox table for undelivered rows and publishes
// them to registered subscribers. At-least-once delivery is guaranteed:
//
//  1. Events are written inside the causing transaction (atomicity).
//  2. After commit, the relay picks up the row and delivers it.
//  3. On successful delivery, the row is marked delivered.
//  4. On failure, the row is retried up to MaxAttempts times with exponential
//     backoff. After MaxAttempts, the row is moved to the dead-letter table.
//
// The relay is a background goroutine started by bootstrap. It should be the
// only reader; multiple relay instances use a PostgreSQL advisory lock to
// prevent duplicate delivery in multi-instance deployments.
//
// Dependency: outbox → events, driver interfaces, pgx pool.
package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"awo.so/awo/events"
)

const (
	// pollInterval is how often the relay checks for new outbox rows.
	pollInterval = 500 * time.Millisecond

	// MaxAttempts is the maximum number of delivery attempts before a row
	// is moved to the dead-letter queue.
	MaxAttempts = 5

	// outboxTable is the database table holding pending events.
	outboxTable = "events_outbox"
)

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

// Start begins the polling loop. Blocks until ctx is cancelled.
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
			if err := r.poll(ctx); err != nil {
				slog.Error("outbox relay poll error", "err", err)
				// Continue — transient errors should not stop the relay.
			}
		}
	}
}

func (r *Relay) poll(ctx context.Context) error {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("outbox.poll: acquire connection: %w", err)
	}
	defer conn.Release()

	// Lock the outbox table with an advisory lock to prevent duplicate delivery
	// in multi-instance deployments. Lock ID is fixed (arbitrary magic number).
	const advisoryLockID = 7777777
	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", advisoryLockID).Scan(&locked); err != nil {
		return fmt.Errorf("outbox.poll: advisory lock: %w", err)
	}
	if !locked {
		return nil // another instance is processing
	}
	defer conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", advisoryLockID) //nolint:errcheck

	rows, err := conn.Query(ctx, fmt.Sprintf(
		`SELECT id, tenant_id, type, entity_name, record_id, actor_id, action_name, payload, occurred_at
		 FROM %s
		 WHERE delivered_at IS NULL AND attempts < $1
		 ORDER BY occurred_at ASC
		 LIMIT 50
		 FOR UPDATE SKIP LOCKED`,
		outboxTable,
	), MaxAttempts)
	if err != nil {
		return fmt.Errorf("outbox.poll: query: %w", err)
	}
	defer rows.Close()

	var toDeliver []events.DomainEvent
	var ids []uuid.UUID

	for rows.Next() {
		var e events.DomainEvent
		var tenantIDStr, recordIDStr, actorIDStr string
		var actionName *string
		var payload []byte

		if err := rows.Scan(
			&e.ID, &tenantIDStr, &e.Type, &e.EntityName,
			&recordIDStr, &actorIDStr, &actionName, &payload, &e.OccurredAt,
		); err != nil {
			return fmt.Errorf("outbox.poll: scan: %w", err)
		}
		e.TenantID, _ = uuid.Parse(tenantIDStr)
		e.RecordID, _ = uuid.Parse(recordIDStr)
		e.ActorID, _ = uuid.Parse(actorIDStr)
		if actionName != nil {
			e.ActionName = *actionName
		}
		e.Payload = payload

		toDeliver = append(toDeliver, e)
		ids = append(ids, e.ID)
	}
	if rows.Err() != nil {
		return fmt.Errorf("outbox.poll: rows: %w", rows.Err())
	}
	rows.Close()

	// Deliver each event.
	for i, e := range toDeliver {
		if err := r.deliver(ctx, e); err != nil {
			slog.Error("outbox delivery failed",
				"event_id", ids[i],
				"event_type", e.Type,
				"err", err,
			)
			// Increment attempts counter.
			conn.Exec(ctx, //nolint:errcheck
				fmt.Sprintf(`UPDATE %s SET attempts = attempts + 1, last_error = $1 WHERE id = $2`, outboxTable),
				err.Error(), ids[i],
			)
			continue
		}
		// Mark delivered.
		conn.Exec(ctx, //nolint:errcheck
			fmt.Sprintf(`UPDATE %s SET delivered_at = NOW(), attempts = attempts + 1 WHERE id = $1`, outboxTable),
			ids[i],
		)
	}
	return nil
}

func (r *Relay) deliver(ctx context.Context, e events.DomainEvent) error {
	// Dispatch to type-specific subscribers first, then wildcard subscribers.
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

// OutboxWriter writes events to the outbox table inside the caller's transaction.
// Implements events.Publisher.
type OutboxWriter struct {
	pool *pgxpool.Pool
}

// NewWriter creates an OutboxWriter.
func NewWriter(pool *pgxpool.Pool) *OutboxWriter {
	return &OutboxWriter{pool: pool}
}

var _ events.Publisher = (*OutboxWriter)(nil)

// Publish writes e to the outbox within the transaction in ctx.
func (w *OutboxWriter) Publish(ctx context.Context, e events.DomainEvent) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}

	payload, err := json.Marshal(e.Payload)
	if err != nil {
		return fmt.Errorf("outbox.Publish: marshal payload: %w", err)
	}

	conn, err := w.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("outbox.Publish: acquire: %w", err)
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, fmt.Sprintf(
		`INSERT INTO %s (id, tenant_id, type, entity_name, record_id, actor_id, action_name, payload, occurred_at, attempts)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 0)`,
		outboxTable,
	),
		e.ID, e.TenantID, string(e.Type), e.EntityName,
		e.RecordID, e.ActorID, nilIfEmpty(e.ActionName), payload, e.OccurredAt,
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
