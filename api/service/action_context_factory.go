package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"awo.so/awo/def"
	"awo.so/awo/events"
	"awo.so/awo/runtime"
	"awo.so/awo/runtime/tenant"
)

// ActionContextFactory builds a *runtime.ActionContext for a single action
// invocation (Phase 2 Step 4 — PHASE2_ARCHITECTURE_PLAN.md §21). It is the
// one place that wires runtime.ActionContextConfig's TxFn/RepoFn/Publish
// fields to real infrastructure, so def.ActionContext.Runtime is non-nil
// (and genuinely functional, not a stub) for every real action invocation.
//
// Register every entity's *EntityService with Register before serving any
// request — api/router.Register does this once, in the same loop that
// constructs each EntityService, before the app starts accepting traffic.
// New's RepoFn closure looks up entities in the registered set at call time
// (per request), so a single construction pass is sufficient: by the time
// any HTTP request reaches New, every entity's EntityService has already
// been registered.
//
// Not safe for concurrent Register calls; safe for concurrent New calls
// once all Register calls have completed (matches the single-threaded
// startup / concurrent-serving lifecycle api/router.Register already has).
type ActionContextFactory struct {
	services map[string]*EntityService
	publish  events.Publisher
}

// NewActionContextFactory creates a factory whose ActionContexts publish
// durable events via publish — the same events.Publisher every entity's
// EntityService uses for its own lifecycle events, so action-originated and
// lifecycle-originated events share one outbox mechanism, never two.
func NewActionContextFactory(publish events.Publisher) *ActionContextFactory {
	if publish == nil {
		publish = events.NoopPublisher{}
	}
	return &ActionContextFactory{services: make(map[string]*EntityService), publish: publish}
}

// Register makes svc's entity available both as New's own EntityName target
// and as a cross-entity Repo(entityName) target for any other entity's
// action.
func (f *ActionContextFactory) Register(svc *EntityService) {
	f.services[svc.schema.QualifiedName] = svc
}

// New constructs a *runtime.ActionContext for one action invocation on
// entityName/recordID, acting as actor. entityName must already have been
// registered via Register — every real caller (api/handler.EntityHandler.Action)
// passes the qualified name of the entity the action's own route is declared
// against, which api/router.Register always registers before any route can
// be reached.
func (f *ActionContextFactory) New(ctx context.Context, entityName string, recordID uuid.UUID, actor *def.Actor) (*runtime.ActionContext, error) {
	svc, ok := f.services[entityName]
	if !ok {
		return nil, fmt.Errorf("api/service: ActionContextFactory.New: entity %q is not registered", entityName)
	}

	tenantID := actorTenantID(actor)
	if tc, ok := tenant.TryFromContext(ctx); ok {
		tenantID = tc.TenantID
	}

	return runtime.NewActionContext(runtime.ActionContextConfig{
		Ctx:        ctx,
		TenantID:   tenantID,
		Actor:      actor,
		EntityName: entityName,
		RecordID:   recordID,
		Publish:    f.publish,
		TxFn:       svc.WithTx,
		RepoFn: func(name string) def.ActionEntityRepo {
			other, ok := f.services[name]
			if !ok {
				return missingEntityRepo{name: name}
			}
			return other.AsActionRepo(actor)
		},
	}), nil
}

func actorTenantID(actor *def.Actor) uuid.UUID {
	if actor == nil {
		return uuid.Nil
	}
	return actor.TenantID
}

// missingEntityRepo implements def.ActionEntityRepo for an entity name that
// was never registered with the ActionContextFactory — a module-author
// error (a typo'd entity name passed to Repo(...)), surfaced as a clear
// error from every method rather than a nil-pointer panic or silent no-op.
type missingEntityRepo struct{ name string }

func (r missingEntityRepo) EntityName() string { return r.name }

func (r missingEntityRepo) err() error {
	return fmt.Errorf("api/service: ActionContext.Repo(%q): entity is not registered", r.name)
}

func (r missingEntityRepo) Get(context.Context, uuid.UUID) (*def.EntityRecord, error) {
	return nil, r.err()
}
func (r missingEntityRepo) Query(context.Context, def.ActionFilter, ...def.ActionQueryOpt) ([]*def.EntityRecord, error) {
	return nil, r.err()
}
func (r missingEntityRepo) Count(context.Context, def.ActionFilter) (int64, error) { return 0, r.err() }
func (r missingEntityRepo) Exists(context.Context, def.ActionFilter) (bool, error) {
	return false, r.err()
}
func (r missingEntityRepo) Create(context.Context, map[string]any) (*def.EntityRecord, error) {
	return nil, r.err()
}
func (r missingEntityRepo) Update(context.Context, uuid.UUID, map[string]any) (*def.EntityRecord, error) {
	return nil, r.err()
}
func (r missingEntityRepo) Delete(context.Context, uuid.UUID) error { return r.err() }
