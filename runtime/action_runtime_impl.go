package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"awo.so/awo/cache"
	"awo.so/awo/compiler"
	"awo.so/awo/def"
)

// ── Dependency interfaces injected at wiring time ─────────────────────────────

// EntityDriver is the storage layer contract. The driver manages transactions
// and calls Pipeline hooks at the correct lifecycle stages.
type EntityDriver interface {
	// Get fetches a single record. Applies RLS via tenant context.
	Get(ctx context.Context, entityName string, id uuid.UUID) (*def.EntityRecord, error)

	// Query returns records matching the filter.
	Query(ctx context.Context, entityName string, f def.ActionFilter, cfg *def.ActionQueryConfig) ([]*def.EntityRecord, error)

	// Count returns the number of records matching the filter.
	Count(ctx context.Context, entityName string, f def.ActionFilter) (int64, error)

	// Exists returns true when at least one record matches.
	Exists(ctx context.Context, entityName string, f def.ActionFilter) (bool, error)

	// Create inserts a record. Runs the Create pipeline internally.
	Create(ctx context.Context, entityName string, data map[string]any, actor *def.Actor) (*def.EntityRecord, error)

	// Update applies a patch. Runs the Update pipeline internally.
	Update(ctx context.Context, entityName string, id uuid.UUID, patch map[string]any, actor *def.Actor) (*def.EntityRecord, error)

	// Delete removes a record. Runs the Delete pipeline internally.
	Delete(ctx context.Context, entityName string, id uuid.UUID, actor *def.Actor) error

	// Tx executes fn inside a database transaction.
	Tx(ctx context.Context, fn func(ctx context.Context) error) error
}

// EventBus publishes domain events to the event outbox.
type EventBus interface {
	Publish(ctx context.Context, event def.ActionEvent) error
}

// WorkflowRuntime enqueues Temporal workflow starts.
type WorkflowRuntime interface {
	StartWorkflow(ctx context.Context, spec def.ActionWorkflowSpec) (string, error)
}

// NotificationService delivers notifications.
type NotificationService interface {
	Notify(ctx context.Context, n def.ActionNotification) error
}

// CacheInvalidator clears SDUI page caches.
type CacheInvalidator interface {
	InvalidateEntity(ctx context.Context, entityName string) error
}

// ── RuntimeFactory ────────────────────────────────────────────────────────────

// RuntimeFactory constructs an ActionRuntime for a specific action invocation.
// Wire one factory at startup and use it in all action route handlers.
type RuntimeFactory struct {
	schema        *compiler.CompiledSchema
	driver        EntityDriver
	bus           EventBus
	workflows     WorkflowRuntime
	notifications NotificationService
	cacheStore    cache.Cache
	invalidator   CacheInvalidator
	logger        *slog.Logger
}

// NewRuntimeFactory creates a RuntimeFactory. All dependencies are required;
// pass NoopEventBus / NoopWorkflowRuntime stubs when a service is not
// deployed (e.g. in tests or minimal deployments).
func NewRuntimeFactory(
	schema *compiler.CompiledSchema,
	driver EntityDriver,
	bus EventBus,
	workflows WorkflowRuntime,
	notifications NotificationService,
	cacheStore cache.Cache,
	invalidator CacheInvalidator,
	logger *slog.Logger,
) *RuntimeFactory {
	return &RuntimeFactory{
		schema:        schema,
		driver:        driver,
		bus:           bus,
		workflows:     workflows,
		notifications: notifications,
		cacheStore:    cacheStore,
		invalidator:   invalidator,
		logger:        logger,
	}
}

// Build constructs an ActionRuntime for a single action invocation.
func (f *RuntimeFactory) Build(
	tenantID uuid.UUID,
	actor *def.Actor,
	entityName, actionName, requestID string,
) def.ActionRuntime {
	log := f.logger.With(
		"tenant_id", tenantID,
		"user_id", actor.UserID,
		"entity", entityName,
		"action", actionName,
		"request_id", requestID,
	)
	return &defaultActionRuntime{
		factory:    f,
		tenantID:   tenantID,
		actor:      actor,
		entityName: entityName,
		actionName: actionName,
		log:        log,
	}
}

// ── DefaultActionRuntime ──────────────────────────────────────────────────────

type defaultActionRuntime struct {
	factory    *RuntimeFactory
	tenantID   uuid.UUID
	actor      *def.Actor
	entityName string
	actionName string
	log        *slog.Logger
}

var _ def.ActionRuntime = (*defaultActionRuntime)(nil)

func (r *defaultActionRuntime) TenantID() uuid.UUID  { return r.tenantID }
func (r *defaultActionRuntime) Actor() *def.Actor    { return r.actor }
func (r *defaultActionRuntime) Clock() time.Time     { return time.Now().UTC() }
func (r *defaultActionRuntime) Logger() *slog.Logger { return r.log }

func (r *defaultActionRuntime) Repo(entityName string) def.ActionEntityRepo {
	return &actionEntityRepo{
		entityName: entityName,
		actor:      r.actor,
		driver:     r.factory.driver,
	}
}

func (r *defaultActionRuntime) Tx(ctx context.Context, fn func(ctx context.Context) error) error {
	return r.factory.driver.Tx(ctx, fn)
}

func (r *defaultActionRuntime) Publish(ctx context.Context, event def.ActionEvent) error {
	if event.TenantID == uuid.Nil {
		event.TenantID = r.tenantID
	}
	return r.factory.bus.Publish(ctx, event)
}

func (r *defaultActionRuntime) StartWorkflow(ctx context.Context, spec def.ActionWorkflowSpec) (string, error) {
	if spec.WorkflowID == "" {
		spec.WorkflowID = fmt.Sprintf("%s.%s.%s", r.tenantID, r.entityName, r.actionName)
	}
	return r.factory.workflows.StartWorkflow(ctx, spec)
}

func (r *defaultActionRuntime) Notify(ctx context.Context, n def.ActionNotification) error {
	if n.TenantID == uuid.Nil {
		n.TenantID = r.tenantID
	}
	if n.Channel == "" {
		n.Channel = "in_app"
	}
	return r.factory.notifications.Notify(ctx, n)
}

func (r *defaultActionRuntime) InvalidateCache(ctx context.Context, entityName string) error {
	return r.factory.invalidator.InvalidateEntity(ctx, entityName)
}

func (r *defaultActionRuntime) Cache() def.ActionCache {
	return &tenantScopedCache{
		prefix: fmt.Sprintf("action:%s:", r.tenantID),
		store:  r.factory.cacheStore,
	}
}

// ── actionEntityRepo ──────────────────────────────────────────────────────────

type actionEntityRepo struct {
	entityName string
	actor      *def.Actor
	driver     EntityDriver
}

var _ def.ActionEntityRepo = (*actionEntityRepo)(nil)

func (r *actionEntityRepo) EntityName() string { return r.entityName }

func (r *actionEntityRepo) Get(ctx context.Context, id uuid.UUID) (*def.EntityRecord, error) {
	return r.driver.Get(ctx, r.entityName, id)
}

func (r *actionEntityRepo) Query(ctx context.Context, f def.ActionFilter, opts ...def.ActionQueryOpt) ([]*def.EntityRecord, error) {
	cfg := &def.ActionQueryConfig{}
	for _, o := range opts {
		o(cfg)
	}
	return r.driver.Query(ctx, r.entityName, f, cfg)
}

func (r *actionEntityRepo) Count(ctx context.Context, f def.ActionFilter) (int64, error) {
	return r.driver.Count(ctx, r.entityName, f)
}

func (r *actionEntityRepo) Exists(ctx context.Context, f def.ActionFilter) (bool, error) {
	return r.driver.Exists(ctx, r.entityName, f)
}

func (r *actionEntityRepo) Create(ctx context.Context, data map[string]any) (*def.EntityRecord, error) {
	return r.driver.Create(ctx, r.entityName, data, r.actor)
}

func (r *actionEntityRepo) Update(ctx context.Context, id uuid.UUID, patch map[string]any) (*def.EntityRecord, error) {
	return r.driver.Update(ctx, r.entityName, id, patch, r.actor)
}

func (r *actionEntityRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.driver.Delete(ctx, r.entityName, id, r.actor)
}

// ── tenantScopedCache ─────────────────────────────────────────────────────────

type tenantScopedCache struct {
	prefix string
	store  cache.Cache
}

var _ def.ActionCache = (*tenantScopedCache)(nil)

func (c *tenantScopedCache) Get(ctx context.Context, key string, dst any) error {
	return c.store.Get(ctx, c.prefix+key, dst)
}

func (c *tenantScopedCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return c.store.Set(ctx, c.prefix+key, value, ttl)
}

func (c *tenantScopedCache) Delete(ctx context.Context, key string) error {
	return c.store.Delete(ctx, c.prefix+key)
}

func (c *tenantScopedCache) DeletePrefix(ctx context.Context, prefix string) error {
	return c.store.DeletePrefix(ctx, c.prefix+prefix)
}

// ── Noop stubs for minimal / test deployments ─────────────────────────────────

// NoopEventBus discards all events. Use in tests or deployments without an
// event broker.
type NoopEventBus struct{}

func (NoopEventBus) Publish(_ context.Context, _ def.ActionEvent) error { return nil }

// NoopWorkflowRuntime discards all workflow starts.
type NoopWorkflowRuntime struct{}

func (NoopWorkflowRuntime) StartWorkflow(_ context.Context, spec def.ActionWorkflowSpec) (string, error) {
	return spec.WorkflowID, nil
}

// NoopNotificationService discards all notifications.
type NoopNotificationService struct{}

func (NoopNotificationService) Notify(_ context.Context, _ def.ActionNotification) error { return nil }

// NoopCacheInvalidator discards all invalidation requests.
type NoopCacheInvalidator struct{}

func (NoopCacheInvalidator) InvalidateEntity(_ context.Context, _ string) error { return nil }
