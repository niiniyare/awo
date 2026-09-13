// Package harness provides a complete in-memory Awo test environment.
// It wires together a Registry, CompiledSchema, fakestore, and tenant/actor
// context so that hook and service tests can run without a database.
package harness

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/registry"
	"awo.so/awo/runtime/tenant"
	"awo.so/awo/testing/fakestore"
)

// Harness is a self-contained test environment.
type Harness struct {
	// Registry is the sealed entity registry.
	Registry *registry.Registry

	// Schema is the compiled schema derived from Registry.
	Schema *compiler.CompiledSchema

	// Store is the in-memory record store.
	Store *fakestore.Store

	// TenantID is the tenant used for all context operations.
	TenantID uuid.UUID

	// ActorID is the user ID placed in the actor context.
	ActorID uuid.UUID

	// ActorRoles are the roles assigned to the test actor.
	ActorRoles []string
}

// New builds a Harness from the provided EntityDefinitions.
// Compilation errors are reported as test failures via t.Fatal.
func New(t testing.TB, defs ...def.EntityDefinition) *Harness {
	t.Helper()

	reg, err := registry.BuildFrom(defs)
	if err != nil {
		t.Fatalf("harness: build registry: %v", err)
	}

	schema, err := compiler.Compile(reg)
	if err != nil {
		t.Fatalf("harness: compile schema: %v", err)
	}

	return &Harness{
		Registry:   reg,
		Schema:     schema,
		Store:      fakestore.New(),
		TenantID:   uuid.New(),
		ActorID:    uuid.New(),
		ActorRoles: []string{"role:tenant.admin"},
	}
}

// Context returns a context carrying the harness TenantContext and Actor.
func (h *Harness) Context() context.Context {
	tc := tenant.TenantContext{
		TenantID:   h.TenantID,
		TenantSlug: "test",
		Locale:     "en-KE",
		Timezone:   "Africa/Nairobi",
		Currency:   "KES",
	}
	ctx := tenant.WithContext(context.Background(), tc)
	return withActor(ctx, &def.Actor{
		UserID:   h.ActorID,
		TenantID: h.TenantID,
		Roles:    h.ActorRoles,
	})
}

// WithRoles returns a context with the given roles instead of ActorRoles.
func (h *Harness) WithRoles(roles ...string) context.Context {
	tc := tenant.TenantContext{
		TenantID:   h.TenantID,
		TenantSlug: "test",
		Locale:     "en-KE",
		Timezone:   "Africa/Nairobi",
		Currency:   "KES",
	}
	ctx := tenant.WithContext(context.Background(), tc)
	return withActor(ctx, &def.Actor{
		UserID:   h.ActorID,
		TenantID: h.TenantID,
		Roles:    roles,
	})
}

type actorKey struct{}

func withActor(ctx context.Context, actor *def.Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, actor)
}

// ActorFromContext extracts the Actor from ctx, or returns nil if absent.
func ActorFromContext(ctx context.Context) *def.Actor {
	a, _ := ctx.Value(actorKey{}).(*def.Actor)
	return a
}
