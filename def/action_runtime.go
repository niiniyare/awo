package def

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// ActionRuntime is the execution environment injected into every
// [ActionHandlerFunc]. It provides access to all framework services within
// the scope of a single action invocation.
//
// The runtime is constructed per-request, tenant-scoped, and
// permission-checked before the handler is called. It must not be retained
// beyond the handler invocation.
//
// All methods are safe for concurrent use.
type ActionRuntime interface {
	// Repo returns a tenant-scoped, permission-aware repository for the named
	// entity. entityName must be a fully-qualified name (e.g. "finance_invoice").
	// The returned repo applies RLS automatically via set_tenant_context.
	Repo(entityName string) ActionEntityRepo

	// Tx executes fn inside a database transaction. The context passed to fn
	// carries the open transaction. If fn returns an error, the transaction
	// rolls back. Any workflow starts issued inside fn are deferred to after
	// commit so they do not trigger on rollback.
	Tx(ctx context.Context, fn func(ctx context.Context) error) error

	// Publish emits a domain event via the event outbox. Returns immediately;
	// delivery is guaranteed by the outbox pattern.
	Publish(ctx context.Context, event ActionEvent) error

	// StartWorkflow enqueues a Temporal workflow. If called inside Tx, the
	// start is deferred to after the transaction commits.
	StartWorkflow(ctx context.Context, spec ActionWorkflowSpec) (workflowID string, err error)

	// Notify sends a notification through the configured channels.
	Notify(ctx context.Context, n ActionNotification) error

	// InvalidateCache clears all cached pages and feature-flag evaluations for
	// the given entity. Call after mutations that change record state visible
	// to SDUI schemas.
	InvalidateCache(ctx context.Context, entityName string) error

	// Cache returns the tenant-namespaced cache accessor.
	Cache() ActionCache

	// Clock returns the current wall-clock time. Use this instead of
	// time.Now() to allow deterministic testing.
	Clock() time.Time

	// Logger returns a structured logger pre-seeded with tenant_id, user_id,
	// entity_name, action_name, and request_id.
	Logger() *slog.Logger

	// TenantID returns the UUID of the tenant this action executes within.
	TenantID() uuid.UUID

	// Actor returns the authenticated principal who invoked the action.
	// Never nil for guarded actions.
	Actor() *Actor
}

// ActionEntityRepo provides entity persistence for a single entity type within
// an action invocation. All methods apply RLS automatically.
type ActionEntityRepo interface {
	// EntityName returns the fully-qualified entity name this repo is bound to.
	EntityName() string

	// Get retrieves a single record by primary key.
	// Returns *runtime.NotFoundError if absent.
	Get(ctx context.Context, id uuid.UUID) (*EntityRecord, error)

	// Query returns records matching f. Options tune limit, offset, and order.
	Query(ctx context.Context, f ActionFilter, opts ...ActionQueryOpt) ([]*EntityRecord, error)

	// Count returns the number of records matching f.
	Count(ctx context.Context, f ActionFilter) (int64, error)

	// Exists returns true when at least one record matches f.
	Exists(ctx context.Context, f ActionFilter) (bool, error)

	// Create inserts a new record and runs the full lifecycle pipeline.
	Create(ctx context.Context, data map[string]any) (*EntityRecord, error)

	// Update applies patch to the existing record and runs the update pipeline.
	Update(ctx context.Context, id uuid.UUID, patch map[string]any) (*EntityRecord, error)

	// Delete removes a record and runs the delete pipeline.
	Delete(ctx context.Context, id uuid.UUID) error
}

// ActionFilter is the predicate passed to ActionEntityRepo query methods.
// The concrete type is *filter.Filter (awo.so/awo/filter). Using any
// here keeps the def package free of filter package imports, matching the
// same pattern as def.Filter used by PolicyFunc.
//
// Do not implement this with types outside awo.so/awo/filter.
type ActionFilter any

// ActionQueryOpt configures a Query call.
type ActionQueryOpt func(*ActionQueryConfig)

// ActionQueryConfig holds the resolved query options.
type ActionQueryConfig struct {
	Limit  int
	Offset int
	Order  string
}

// WithActionLimit sets the maximum number of records returned.
func WithActionLimit(n int) ActionQueryOpt {
	return func(c *ActionQueryConfig) { c.Limit = n }
}

// WithActionOffset sets the record offset for pagination.
func WithActionOffset(n int) ActionQueryOpt {
	return func(c *ActionQueryConfig) { c.Offset = n }
}

// WithActionOrder sets the sort expression (e.g. "created_at DESC").
func WithActionOrder(expr string) ActionQueryOpt {
	return func(c *ActionQueryConfig) { c.Order = expr }
}

// ActionEvent is a domain event published via [ActionRuntime.Publish].
type ActionEvent struct {
	// Topic is the routing key (e.g. "finance.invoice.submitted").
	Topic string

	// Payload is the event body. Must be JSON-serialisable.
	Payload any

	// TenantID is set automatically by the runtime when left zero.
	TenantID uuid.UUID
}

// ActionWorkflowSpec describes a Temporal workflow start request.
type ActionWorkflowSpec struct {
	// WorkflowFn is the registered Temporal workflow function name.
	WorkflowFn string

	// TaskQueue is the Temporal task queue to route the workflow to.
	TaskQueue string

	// WorkflowID is an optional stable ID. When empty, the runtime generates
	// one using the convention: {tenant}.{entity}.{record_id}.{action}.
	WorkflowID string

	// Input is the workflow input value. Must be JSON-serialisable.
	Input any
}

// ActionNotification is sent via [ActionRuntime.Notify].
type ActionNotification struct {
	// UserIDs is the list of target user UUIDs.
	UserIDs []uuid.UUID

	// Subject is the notification title / email subject.
	Subject string

	// Body is the notification body (plain text or HTML, channel-dependent).
	Body string

	// Channel is the delivery channel ("email", "sms", "in_app").
	// Defaults to "in_app" when empty.
	Channel string

	// TenantID is set automatically by the runtime when left zero.
	TenantID uuid.UUID
}

// ActionCache provides cache read/write within an action, namespaced by tenant.
type ActionCache interface {
	// Get deserialises the value at key into dst. Returns cache.ErrMiss on miss.
	Get(ctx context.Context, key string, dst any) error

	// Set stores value at key with the given TTL.
	Set(ctx context.Context, key string, value any, ttl time.Duration) error

	// Delete removes key. No-op on miss.
	Delete(ctx context.Context, key string) error

	// DeletePrefix removes all keys sharing the given prefix.
	DeletePrefix(ctx context.Context, prefix string) error
}
