// Package events defines the transactional outbox and event bus abstractions.
//
// All domain events are published inside the database transaction that caused
// them (write-ahead pattern). A background relay picks up committed rows and
// delivers them to subscribers. This guarantees at-least-once delivery even
// when the process crashes between commit and publish.
//
// Dependency: events → def only. Concrete implementations live in
// awo/contrib/pgx (outbox table) and awo/contrib/redis (pub/sub relay).
package events

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// EventType identifies the kind of domain event.
type EventType string

const (
	// Lifecycle events — emitted by the runtime pipeline.
	EventCreated EventType = "entity.created"
	EventUpdated EventType = "entity.updated"
	EventDeleted EventType = "entity.deleted"

	// Action events — emitted by action handlers.
	EventActionFired EventType = "entity.action"
)

// DomainEvent is the canonical event envelope written to the outbox table.
// All fields are populated before the outbox write; consumers receive the
// exact same struct after relay.
type DomainEvent struct {
	// ID is a UUIDv7 assigned at event creation time.
	ID uuid.UUID `json:"id"`

	// TenantID scopes the event to a tenant. Zero value for platform events.
	TenantID uuid.UUID `json:"tenant_id"`

	// Type is the event kind.
	Type EventType `json:"type"`

	// EntityName is the registered entity name (e.g. "finance_invoice").
	EntityName string `json:"entity_name"`

	// RecordID is the affected record's primary key.
	RecordID uuid.UUID `json:"record_id"`

	// ActorID is the user who caused the event. Zero value for system events.
	ActorID uuid.UUID `json:"actor_id,omitempty"`

	// ActionName is non-empty for EventActionFired events.
	ActionName string `json:"action_name,omitempty"`

	// Payload is the serialized record snapshot at event time.
	// Format: JSON-encoded map[string]any.
	Payload []byte `json:"payload,omitempty"`

	// OccurredAt is the wall-clock time at event creation (UTC).
	OccurredAt time.Time `json:"occurred_at"`
}

// Publisher writes domain events to the outbox table inside the current
// transaction. Implementations must be safe for concurrent use.
//
// The context must carry a transacted connection; Publisher.Publish will
// return an error if no active transaction is present.
type Publisher interface {
	// Publish writes e to the outbox within the transaction in ctx.
	Publish(ctx context.Context, e DomainEvent) error
}

// Subscriber receives relayed domain events.
type Subscriber interface {
	// HandleEvent processes a delivered event. Returning a non-nil error
	// causes the relay to retry (up to its configured retry policy).
	HandleEvent(ctx context.Context, e DomainEvent) error
}

// Bus routes committed outbox events to registered subscribers.
// Implementations are responsible for relay durability and retry logic.
type Bus interface {
	// Subscribe registers h to receive events of the given type.
	// If eventType is empty, h receives all events.
	Subscribe(eventType EventType, h Subscriber)

	// Start begins the relay loop. Blocks until ctx is cancelled.
	Start(ctx context.Context) error
}

// NoopPublisher discards all events. Useful in unit tests where no database
// is available and event delivery is not under test.
type NoopPublisher struct{}

func (NoopPublisher) Publish(_ context.Context, _ DomainEvent) error { return nil }
