// Package runtime implements the request pipeline, hook executor, policy
// engine, and entity lifecycle manager.
//
// # Boot sequence
//
// The [Boot] function executes the full startup sequence in dependency order:
//
//  1. Config load & validate
//  2. PostgreSQL pool init (ping)
//  3. Redis client init (ping)
//  4. def registry sealed + registry.Build()
//  5. compiler.Compile()
//  6. Fiber app init + route registration (from compiled routes)
//  7. Fiber server start
//  8. Temporal worker start (concurrent; failure = degraded, not fatal)
//
// EntityRegistry failure is always fatal — the process cannot serve requests
// without a compiled schema.
//
// # Request pipeline
//
// The [Pipeline] type runs a mutation request through the full hook sequence:
//
//	ASSEMBLE → BeforeValidate → VALIDATE → AUTHORIZE
//	         → BeforeSave/BeforeCreate/BeforeUpdate
//	         → [TX begins]
//	         → PERSIST
//	         → AfterSave/AfterCreate/AfterUpdate
//	         → [TX commits]
//	         → Workflow start (outside TX)
//
// # Tenant context
//
// The [tenant] sub-package handles TenantContext propagation. It is the
// canonical source of tenant identity within a request — never global
// variables, never function parameters alongside context.Context.
package runtime
