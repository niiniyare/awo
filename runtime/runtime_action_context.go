package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"awo.so/awo/audit"
	"awo.so/awo/def"
	"awo.so/awo/events"
)

// ActionContext is the concrete, canonical implementation of
// [def.ActionRuntime] (Phase 2 Step 4 — PHASE2_ARCHITECTURE_PLAN.md §21,
// ADR-025 §4/§10). It is constructed per-action-invocation by the framework
// (see [NewActionContext], wired by api/service.ActionContextFactory) and
// passed to [def.ActionDef.HandlerFunc] as [def.ActionContext.Runtime].
//
// ActionContext is NOT safe for use after the handler returns.
// It must not be stored in goroutines that outlive the request.
type ActionContext struct {
	ctx        context.Context
	tenantID   uuid.UUID
	actor      *def.Actor
	entityName string
	recordID   uuid.UUID

	// publish is the event outbox publisher. May be events.NoopPublisher{} in
	// unit tests. Both Publish and StartWorkflow delegate to this — there is
	// no separate workflow.WorkflowExecutor dependency here: a workflow start
	// is durable intent (an outbox event), never a direct Temporal call from
	// this transaction-bound path (ADR-025 §4.D, §10).
	publish events.Publisher

	// txFn is the function that wraps a callback in a database transaction.
	// Provided by the driver (contrib/pgx), via api/service.EntityService.WithTx.
	// May be a no-op in tests.
	txFn func(ctx context.Context, fn func(ctx context.Context) error) error

	// repoFn resolves an ActionEntityRepo for the given entity name.
	// Provided by api/service.ActionContextFactory at construction time —
	// backed by the same api/service.EntityService (and therefore the same
	// canonical mutation pipeline: validation, hooks, audit, durable outbox
	// event) that ordinary HTTP CRUD uses. Actions do not get a second,
	// lesser mutation path.
	repoFn func(entityName string) def.ActionEntityRepo

	// cache is the tenant-namespaced cache. Defaults to NoopActionCache{}.
	cache def.ActionCache

	// notifyFn sends notifications. Optional; no-op when nil.
	notifyFn func(ctx context.Context, n def.ActionNotification) error

	// invalidateFn clears cached pages for an entity. Optional; no-op when nil.
	invalidateFn func(ctx context.Context, entityName string) error

	// logger is a structured logger pre-seeded with action context fields.
	logger *slog.Logger
}

// ActionContextConfig holds all dependencies needed to construct an
// [ActionContext]. Callers that do not have a particular dependency should
// supply the appropriate no-op value rather than nil.
type ActionContextConfig struct {
	// Ctx is the request context.
	Ctx context.Context

	// TenantID is the tenant scope for this action.
	TenantID uuid.UUID

	// Actor is the authenticated principal. Must not be nil for guarded actions.
	Actor *def.Actor

	// EntityName is the qualified name of the entity this action was invoked
	// on (e.g. "finance_invoice") — the entity the action's own route is
	// declared against, per ActionDef's routing convention
	// (POST /api/v1/entities/{entity-name}/{id}/{action-name}). Required so
	// Publish/StartWorkflow can populate events.DomainEvent's NOT NULL
	// entity_name column; a zero value there would make every outbox write
	// from this ActionContext fail at the database level.
	EntityName string

	// RecordID is the primary key of the record this action targets — the
	// ":id" path segment. Required for the same reason as EntityName (the
	// outbox schema's record_id column is NOT NULL).
	RecordID uuid.UUID

	// Publish writes domain events to the transactional outbox.
	// Required. Pass events.NoopPublisher{} to disable event publishing.
	Publish events.Publisher

	// TxFn wraps fn in a database transaction. The context passed to fn
	// carries the open transaction handle understood by the driver.
	// Required. Pass a no-op wrapper in unit tests.
	TxFn func(ctx context.Context, fn func(ctx context.Context) error) error

	// RepoFn resolves a per-entity repository by qualified entity name.
	// Required. Pass a stub that returns a no-op repo in unit tests.
	RepoFn func(entityName string) def.ActionEntityRepo

	// Cache provides tenant-namespaced cache access. Defaults to
	// NoopActionCache{} when nil.
	Cache def.ActionCache

	// NotifyFn sends notifications. May be nil (no-op).
	NotifyFn func(ctx context.Context, n def.ActionNotification) error

	// InvalidateFn clears cached SDUI schemas for an entity. May be nil (no-op).
	InvalidateFn func(ctx context.Context, entityName string) error

	// Logger is the structured logger. Defaults to slog.Default() when nil.
	Logger *slog.Logger
}

// NewActionContext constructs an ActionContext from cfg.
// Panics if Publish, TxFn, or RepoFn are nil — these are required
// dependencies; callers must supply no-op values rather than nil.
func NewActionContext(cfg ActionContextConfig) *ActionContext {
	if cfg.Publish == nil {
		panic("runtime: NewActionContext: Publish must not be nil; pass events.NoopPublisher{}")
	}
	if cfg.TxFn == nil {
		panic("runtime: NewActionContext: TxFn must not be nil")
	}
	if cfg.RepoFn == nil {
		panic("runtime: NewActionContext: RepoFn must not be nil")
	}

	cache := cfg.Cache
	if cache == nil {
		cache = NoopActionCache{}
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With(
		"tenant_id", cfg.TenantID,
		"entity", cfg.EntityName,
		"record_id", cfg.RecordID,
	)
	if cfg.Actor != nil {
		logger = logger.With("user_id", cfg.Actor.UserID)
	}

	return &ActionContext{
		ctx:          cfg.Ctx,
		tenantID:     cfg.TenantID,
		actor:        cfg.Actor,
		entityName:   cfg.EntityName,
		recordID:     cfg.RecordID,
		publish:      cfg.Publish,
		txFn:         cfg.TxFn,
		repoFn:       cfg.RepoFn,
		cache:        cache,
		notifyFn:     cfg.NotifyFn,
		invalidateFn: cfg.InvalidateFn,
		logger:       logger,
	}
}

// Ensure ActionContext implements def.ActionRuntime at compile time.
var _ def.ActionRuntime = (*ActionContext)(nil)

// ── def.ActionRuntime implementation ─────────────────────────────────────────

// Repo returns a tenant-scoped repository for the named entity.
// entityName must be fully-qualified (e.g. "finance_invoice").
func (a *ActionContext) Repo(entityName string) def.ActionEntityRepo {
	return a.repoFn(entityName)
}

// Tx executes fn inside a database transaction. If fn returns an error, the
// transaction rolls back. Workflow starts and Publish calls issued inside fn
// are durable outbox writes on the same transaction — they commit or roll
// back atomically with everything else fn does, never as a separate step.
func (a *ActionContext) Tx(ctx context.Context, fn func(ctx context.Context) error) error {
	return a.txFn(ctx, fn)
}

// Publish emits a domain event via the transactional outbox. The event is
// written in the same transaction as the mutation that caused it — ctx must
// carry an active transaction (see Tx); if it does not, the underlying
// events.Publisher (per its own documented contract) returns an error rather
// than silently writing outside the mutation's atomicity boundary.
// TenantID is set automatically when left zero in event.
func (a *ActionContext) Publish(ctx context.Context, event def.ActionEvent) error {
	tenantID := event.TenantID
	if tenantID == uuid.Nil {
		tenantID = a.tenantID
	}

	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("runtime: ActionContext.Publish: marshal payload: %w", err)
	}

	return a.publish.Publish(ctx, events.DomainEvent{
		ID:            uuid.New(),
		TenantID:      tenantID,
		Type:          events.EventActionFired,
		EntityName:    a.entityName,
		RecordID:      a.recordID,
		ActorID:       actorUserID(a.actor),
		ActionName:    event.Topic,
		CorrelationID: correlationIDFromContext(ctx),
		Payload:       payload,
		OccurredAt:    time.Now().UTC(),
	})
}

// StartWorkflow records a durable intent to start a Temporal workflow — it
// does NOT call Temporal (ADR-025 §4.D, §10). The mutation transaction must
// never depend on a direct Temporal network call: StartWorkflow instead
// publishes an EventWorkflowTriggerFired domain event, through the exact
// same transactional outbox Publish uses, carrying the resolved
// def.ActionWorkflowSpec (WorkflowFn, TaskQueue, WorkflowID, Input) as its
// JSON payload. The durable boundary is:
//
//	mutation → audit → events_outbox INSERT → COMMIT → relay → workflow dispatch
//
// If called inside Tx and the surrounding transaction later fails, this
// event — like any other outbox write in that transaction — rolls back with
// it: no workflow-start intent survives a rolled-back mutation. If the
// transaction commits, the intent is durable; whether Temporal ever
// executes it depends on the relay's WorkflowTriggerSubscriber successfully
// reaching Temporal, which happens strictly after commit, on a separate,
// independently-retried path. A Temporal outage discovered by the relay
// after commit can never roll back this already-committed mutation — that
// is a property of the database transaction already having completed
// before dispatch is ever attempted, not something this method enforces.
//
// This method itself never claims exactly-once execution — only that the
// intent to start is durable and will be dispatched at-least-once.
//
// WorkflowID is auto-generated when left empty in spec, exactly as before —
// the returned string is that resolved WorkflowID (the same identity the
// relay's later dispatch will use), not a value Temporal itself has
// returned, since Temporal has not been contacted yet.
func (a *ActionContext) StartWorkflow(ctx context.Context, spec def.ActionWorkflowSpec) (string, error) {
	wid := spec.WorkflowID
	if wid == "" {
		wid = fmt.Sprintf("%s.%s.%s", a.tenantID, spec.TaskQueue, uuid.New())
	}
	spec.WorkflowID = wid

	payload, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("runtime: ActionContext.StartWorkflow: marshal workflow spec: %w", err)
	}

	if err := a.publish.Publish(ctx, events.DomainEvent{
		ID:            uuid.New(),
		TenantID:      a.tenantID,
		Type:          events.EventWorkflowTriggerFired,
		EntityName:    a.entityName,
		RecordID:      a.recordID,
		ActorID:       actorUserID(a.actor),
		CorrelationID: correlationIDFromContext(ctx),
		Payload:       payload,
		OccurredAt:    time.Now().UTC(),
	}); err != nil {
		return "", fmt.Errorf("runtime: ActionContext.StartWorkflow: publish workflow intent: %w", err)
	}
	return wid, nil
}

// Notify sends a notification through configured channels. TenantID is set
// automatically when left zero in n. No-op when no notify function is wired.
func (a *ActionContext) Notify(ctx context.Context, n def.ActionNotification) error {
	if n.TenantID == uuid.Nil {
		n.TenantID = a.tenantID
	}
	if a.notifyFn == nil {
		return nil
	}
	return a.notifyFn(ctx, n)
}

// InvalidateCache clears cached SDUI schemas and feature-flag evaluations for
// the given entity. No-op when no invalidate function is wired.
func (a *ActionContext) InvalidateCache(ctx context.Context, entityName string) error {
	if a.invalidateFn == nil {
		return nil
	}
	return a.invalidateFn(ctx, entityName)
}

// Cache returns the tenant-namespaced cache accessor.
func (a *ActionContext) Cache() def.ActionCache {
	return a.cache
}

// Clock returns the current wall-clock time in UTC.
// Use this instead of time.Now() to enable deterministic testing.
func (a *ActionContext) Clock() time.Time {
	return time.Now().UTC()
}

// Logger returns a structured logger pre-seeded with tenant_id and user_id.
func (a *ActionContext) Logger() *slog.Logger {
	return a.logger
}

// TenantID returns the UUID of the tenant this action executes within.
func (a *ActionContext) TenantID() uuid.UUID {
	return a.tenantID
}

// Actor returns the authenticated principal who invoked the action.
func (a *ActionContext) Actor() *def.Actor {
	return a.actor
}

// actorUserID returns actor.UserID, or uuid.Nil if actor is nil.
func actorUserID(actor *def.Actor) uuid.UUID {
	if actor == nil {
		return uuid.Nil
	}
	return actor.UserID
}

// correlationIDFromContext reads the HTTP request correlation ID from the
// audit.RequestContext injected by RequireAuth, matching
// api/service.EntityService.publishLifecycleEvent's identical convention —
// the same source, the same opaque-string semantics (not a validated UUID),
// used consistently across both the lifecycle-event and action-event paths.
// Returns "" for background/system-originated or unauthenticated contexts,
// where no RequestContext was ever injected.
func correlationIDFromContext(ctx context.Context) string {
	if rc, ok := audit.RequestContextFromContext(ctx); ok {
		return rc.RequestID
	}
	return ""
}
