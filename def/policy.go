package def

import "context"

// Filter is the opaque predicate type used by PolicyFunc and the Filter DSL.
// The concrete implementation lives in awo.so/awo/filter. Using an interface
// here keeps def free of filter package imports.
//
// The zero value (nil) means "no additional restriction" — all tenant-scoped
// rows that pass RLS are returned.
//
// The concrete type is *filter.Filter. Runtime code type-asserts to it.
// Do not implement this interface with types outside awo.so/awo/filter.
type Filter any

// PolicyFunc is a row-level filter injected into every query for an entity.
// It runs at request time and receives the full request context, allowing
// policies to inspect the authenticated actor, feature flags, and tenant
// settings.
//
// PolicyFunc complements RBAC (operation-level gates): RBAC controls whether
// an actor can perform an operation at all; PolicyFunc controls which rows the
// actor can see or mutate.
//
// # Example: owner-only policy
//
//	def.PolicyFunc(func(ctx context.Context) def.Filter {
//	    actor := session.ActorFromContext(ctx)
//	    return filter.Eq("assigned_to", actor.UserID)
//	})
//
// # Nil return
//
// Returning nil means no additional row restriction. All tenant-scoped rows
// visible after RLS are returned. This is appropriate for admin roles.
//
// # Composition
//
// Multiple policies are AND-composed by the runtime. Do not OR-compose by
// returning a compound filter from a single PolicyFunc — declare separate
// policies and let the runtime compose them.
type PolicyFunc func(ctx context.Context) Filter

// NoPolicy is a PolicyFunc that applies no additional row restriction.
// Use when RBAC gates are sufficient and no row-level filtering is needed.
var NoPolicy PolicyFunc = func(_ context.Context) Filter { return nil }
