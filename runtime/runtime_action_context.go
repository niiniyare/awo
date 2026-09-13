package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"awo.so/awo/def"
	"awo.so/awo/events"
	"awo.so/awo/workflow"
)

// ActionContext is the concrete implementation of [def.ActionRuntime].
// It is constructed per-action-invocation by the framework (see
// [NewActionContext]) and passed to [def.ActionDef.HandlerFunc] as
// [def.ActionContext.Runtime].
//
// ActionContext is NOT safe for use after the handler returns.
// It must not be stored in goroutines that outlive the request.
type ActionContext struct {
	ctx      context.Context
	tenantID uuid.UUID
	actor    *def.Actor

	// publish is the event outbox publisher. May be events.NoopPublisher{} in
	// unit tests.
	publish events.Publisher

	// executor is the workflow executor. May be workflow.NoopExecutor{} when
	// Temporal is not configured (degraded mode).
	executor workflow.WorkflowExecutor

	// txFn is the function that wraps a callback in a database transaction.
	// Provided by the driver (contrib/pgx). May be a no-op in tests.
	txFn func(ctx context.Context, fn func(ctx context.Context) error) error

	// repoFn resolves an ActionEntityRepo for the given entity name.
	// Provided by the driver layer at construction time.
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

	// Publish writes domain events to the transactional outbox.
	// Required. Pass events.NoopPublisher{} to disable event publishing.
	Publish events.Publisher

	// Executor starts Temporal workflows.
	// Required. Pass workflow.NoopExecutor{} when Temporal is unavailable.
	Executor workflow.WorkflowExecutor

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
// Panics if Publish, Executor, TxFn, or RepoFn are nil — these are required
// dependencies; callers must supply no-op values rather than nil.
func NewActionContext(cfg ActionContextConfig) *ActionContext {
	if cfg.Publish == nil {
		panic("runtime: NewActionContext: Publish must not be nil; pass events.NoopPublisher{}")
	}
	if cfg.Executor == nil {
		panic("runtime: NewActionContext: Executor must not be nil; pass workflow.NoopExecutor{}")
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
	)
	if cfg.Actor != nil {
		logger = logger.With("user_id", cfg.Actor.UserID)
	}

	return &ActionContext{
		ctx:          cfg.Ctx,
		tenantID:     cfg.TenantID,
		actor:        cfg.Actor,
		publish:      cfg.Publish,
		executor:     cfg.Executor,
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
// transaction rolls back. Workflow starts queued inside fn are deferred to
// after the transaction commits.
func (a *ActionContext) Tx(ctx context.Context, fn func(ctx context.Context) error) error {
	return a.txFn(ctx, fn)
}

// Publish emits a domain event via the transactional outbox. The event is
// written in the same transaction as the mutation that caused it.
// TenantID is set automatically when left zero.
func (a *ActionContext) Publish(ctx context.Context, event def.ActionEvent) error {
	tenantID := event.TenantID
	if tenantID == uuid.Nil {
		tenantID = a.tenantID
	}
	return a.publish.Publish(ctx, events.DomainEvent{
		ID:         uuid.New(),
		TenantID:   tenantID,
		Type:       events.EventActionFired,
		ActionName: event.Topic,
		Payload:    []byte(fmt.Sprintf("%v", event.Payload)),
		OccurredAt: time.Now().UTC(),
	})
}

// StartWorkflow enqueues a Temporal workflow via the configured executor.
// WorkflowID is auto-generated when left empty in spec.
func (a *ActionContext) StartWorkflow(ctx context.Context, spec def.ActionWorkflowSpec) (string, error) {
	wid := spec.WorkflowID
	if wid == "" {
		wid = fmt.Sprintf("%s.%s.%s", a.tenantID, spec.TaskQueue, uuid.New())
	}
	id, err := a.executor.Start(ctx, workflow.WorkflowSpec{
		WorkflowID: wid,
		TaskQueue:  spec.TaskQueue,
		WorkflowFn: spec.WorkflowFn,
		Input:      spec.Input,
	})
	if err != nil {
		return "", err
	}
	return string(id), nil
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
