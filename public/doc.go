// Package public documents the stable public API of the Awo framework.
//
// # Framework Entry Point
//
// Initialize the framework by running bootstrap and wiring dependencies manually:
//
//	result, err := bootstrap.Run(ctx, bootstrap.Config{
//	    DatabaseURL: "postgres://...",
//	    RedisURL:    "redis://...",
//	})
//
// # Entity Registration
//
// Register entities in module init() functions only. Never call [def.Register]
// from a handler, service, or after bootstrap completes:
//
//	func init() {
//	    def.Register(&MyEntityDefinition)
//	}
//
// # Module Pattern
//
// Group related entities and services into a module. Modules are imported for
// their init() side effects (entity registration) and for explicit service
// wiring in main/bootstrap:
//
//	// In your module package:
//	var Definition = def.SystemDefinition{
//	    Name: "mymodule_record",
//	    // ...
//	}
//
//	func init() {
//	    def.Register(&Definition)
//	}
//
// # Dependency Injection
//
// The framework uses the explicit Options/functional-options pattern. There is
// no Wire codegen, no global service locator, and no reflection-based DI
// container. All dependencies are constructed explicitly in main or bootstrap:
//
//	sessions := contribredis.NewSessionStore(rdb)
//	iamModule := iam.New(pool, sessions, tokenCache, repos)
//
// # Stable Packages (v1.0 API)
//
// These packages form the stable public API surface. Breaking changes require
// a new ADR and a semver major version bump:
//
//   - [awo.so/awo/def] — EntityDefinition DSL (SystemDefinition, CustomDefinition,
//     FieldDef, PermissionSet, HookSet, ActionDef, WorkflowDef)
//   - [awo.so/awo/compiler] — Schema compilation (EntityDefinition → CompiledSchema)
//   - [awo.so/awo/registry] — Entity registry (Register, Lookup, All, Seal, Build)
//   - [awo.so/awo/runtime] — Entity lifecycle pipeline (BeforeValidate → Persist → AfterCreate)
//   - [awo.so/awo/driver] — Storage abstraction (EntityRepository[T], QueryOptions)
//   - [awo.so/awo/filter] — Query filter predicates (Eq, Gt, Lt, And, Or, Not, fluent builder)
//   - [awo.so/awo/auth] — Session and ViewerContext (Session, ViewerContext, SessionStore)
//   - [awo.so/awo/events] — Domain events (DomainEvent, Publisher, Bus, outbox relay)
//   - [awo.so/awo/workflow] — Workflow executor (WorkflowExecutor, NoopExecutor, TemporalExecutor)
//   - [awo.so/awo/bootstrap] — Startup sequence (Run, Config, Result, Shutdown)
//
// # Unstable Packages (subject to change before v1.0)
//
// These packages are implementation details or actively evolving. Their APIs
// may change without a major version bump until v1.0 is declared:
//
//   - [awo.so/awo/contrib/pgx] — PostgreSQL driver (implements driver.EntityRepository)
//   - [awo.so/awo/contrib/redis] — Redis session store (implements auth.SessionStore)
//   - [awo.so/awo/sdui] — Server-driven UI engine (API stabilizing in Phase 17)
//   - [awo.so/awo/generator] — Migration and OpenAPI generation (CLI use only)
//   - [awo.so/awo/platform] — Platform entities (iam, tenant, audit, flags, settings)
//
// # Authorization Model
//
// Authorization is two-layer (ADR-001, ADR-011):
//
//  1. Declaration: [def.PermissionSet] on EntityDefinition — stable permission identifiers only.
//  2. Enforcement: [auth.PolicyEvaluator] (default: Casbin) — replaceable without touching entities.
//
// EntityDefinition MUST NOT reference role names, JWT claims, or any RBAC backend detail.
//
// # Multi-Tenancy
//
// Every request carries a TenantID extracted by the TenantResolver middleware.
// Tenant context is propagated via context.Context, never as function arguments:
//
//	tenantID := tenant.FromContext(ctx) // panics if middleware was bypassed
//
// PostgreSQL Row-Level Security (RLS) enforces tenant isolation at the database
// layer unconditionally. Application-layer PolicyFunc is an optimization only.
//
// # Naming Convention
//
// Entity names follow the pattern {module}_{noun} in singular snake_case:
//
//	finance_invoice   iam_user   platform_tenant
//
// Entity names are permanent. They are embedded in migration filenames,
// Temporal workflow IDs, Redis keys, and Casbin policies. Never rename an
// entity after its migration has been applied to any environment.
package public
