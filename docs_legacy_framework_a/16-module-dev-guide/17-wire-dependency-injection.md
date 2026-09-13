> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Wire Dependency Injection"
id: mdg-017
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[Module System](../10-modules/module-system.md)"
  - "[Startup Sequence](../03-kernel/startup-sequence.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Wire Dependency Injection

**MDG-017 | Status: Accepted | Stability: Stable**

How Awo wires dependencies between modules, hooks, services, and handlers using Google Wire.

---

## 1. Overview

Awo uses [Google Wire](https://github.com/google/wire) for compile-time dependency injection. Wire generates the wiring code from provider functions — no runtime reflection, no magic, no startup panic from missing dependency.

Every injectable component provides a `NewXxx(deps...) *Xxx` constructor. Wire traces the dependency graph at build time and generates a `wire_gen.go` file that calls constructors in the correct order.

---

## 2. Provider Function Pattern

```go
// internal/core/finance/wire.go

// ProvideInvoiceHooks constructs all hooks that need injected dependencies.
// Hooks with no dependencies (pure logic) are instantiated directly in EntityDefinition.
func ProvideInvoiceHooks(
    settings service.SettingsService,
    customerRepo def.EntityRepository[Customer],
) *InvoiceHooks {
    return &InvoiceHooks{
        Settings:     settings,
        CustomerRepo: customerRepo,
    }
}

// ProvideFinanceModule wires the entire finance module.
func ProvideFinanceModule(hooks *InvoiceHooks, ...) *FinanceModule {
    // Inject hooks into entity definitions
    InvoiceDefinition.Hooks.BeforeCreate = append(
        InvoiceDefinition.Hooks.BeforeCreate,
        &hooks.InvoiceValidator,
        &hooks.InvoiceApprovalGuard,
    )
    return &FinanceModule{...}
}
```

---

## 3. Wire Provider Sets

Each module exports a `ProviderSet` that groups all its providers:

```go
// internal/core/finance/wire.go

var FinanceProviderSet = wire.NewSet(
    ProvideInvoiceHooks,
    ProvideFinanceService,
    ProvideFinanceHandlers,
    ProvideFinanceModule,
)
```

The top-level `cmd/server/wire.go` assembles all modules:

```go
// cmd/server/wire.go

func InitializeApp(cfg *config.Config) (*App, error) {
    wire.Build(
        // Infrastructure
        db.ProvidePool,
        redis.ProvideClient,
        temporal.ProvideClient,

        // Platform modules
        platform.TenantProviderSet,
        platform.IAMProviderSet,
        platform.FlagsProviderSet,
        platform.SettingsProviderSet,
        platform.AuditProviderSet,

        // Business modules
        finance.FinanceProviderSet,
        hr.HRProviderSet,
        inventory.InventoryProviderSet,

        // App assembly
        ProvideApp,
    )
    return nil, nil  // Wire replaces this with generated code
}
```

---

## 4. Repository Provision

`EntityRepository[T]` implementations are provisioned by the framework, not by module authors. Module authors declare the type they need; Wire resolves it:

```go
// Framework provides this generic constructor
func ProvideEntityRepository[T def.Entity](
    pool *pgxpool.Pool,
    def *def.EntityDefinition,
) def.EntityRepository[T] {
    return &pgxEntityRepository[T]{pool: pool, def: def}
}
```

Module authors inject `EntityRepository` via constructor:

```go
type InvoiceHooks struct {
    Settings     service.SettingsService
    CustomerRepo def.EntityRepository[Customer]  // Wire resolves this
}
```

---

## 5. Cross-Module Service Interfaces

Cross-module dependencies use interfaces, not concrete types (see [Module Boundaries](../02-architecture/module-boundaries.md)):

```go
// internal/core/projects/deps.go
// FinanceService interface — projects module depends on this, not on finance internals
type FinanceService interface {
    CreateProjectInvoice(ctx context.Context, input finance.ProjectInvoiceInput) (uuid.UUID, error)
}

// internal/core/projects/wire.go
func ProvideProjectService(
    repo   def.EntityRepository[Project],
    finance FinanceService,  // injected — Wire resolves to *finance.FinanceServiceImpl
) *ProjectService {
    return &ProjectService{Repo: repo, Finance: finance}
}
```

Wire binds the interface to the implementation:

```go
// cmd/server/wire.go
wire.Bind(new(projects.FinanceService), new(*finance.FinanceServiceImpl)),
```

---

## 6. Hooks with No Dependencies

Pure hooks (no I/O, no external dependencies) are instantiated directly in the `EntityDefinition` variable:

```go
var InvoiceDefinition = def.SystemDefinition{
    // ...
    Hooks: def.HookSet{
        BeforeSave: []def.BeforeSaveHook{
            &TaskCompletionHook{},  // no dependencies — instantiate directly
        },
    },
}
```

Hooks with dependencies are injected after `EntityDefinition` is declared:

```go
// In ProvideFinanceModule(hooks *InvoiceHooks):
InvoiceDefinition.Hooks.BeforeCreate = []def.BeforeCreateHook{
    &hooks.InvoiceValidator,       // needs SettingsService
    &hooks.InvoiceApprovalGuard,   // needs CustomerRepo
}
```

---

## 7. Generating Wire Code

After modifying providers:

```bash
# Run in the module directory (or project root)
wire gen ./cmd/server/
```

This regenerates `cmd/server/wire_gen.go`. The generated file is committed to source control — it is not auto-generated at build time.

Never edit `wire_gen.go` manually. Changes are overwritten on next `wire gen`.

---

## 8. Common Wire Errors

### `wire: no provider found for X`

A constructor requires a type for which no provider is registered. Fix: add the provider to the relevant `ProviderSet`, or add a `wire.Bind` for an interface.

### `wire: cycle detected`

Module A depends on Module B, which depends on Module A. Fix: extract the shared dependency into `internal/shared/` or invert the dependency via an interface.

### `wire: provider X is never used`

A provider was added to a ProviderSet but nothing depends on it. Fix: either remove the provider or add a dependent.

---

## Related Documents

- [Module System](../10-modules/module-system.md) — module manifest and registration
- [Startup Sequence](../03-kernel/startup-sequence.md) — when Wire-generated code runs
- [Module Boundaries](../02-architecture/module-boundaries.md) — why cross-module deps use interfaces
- [Common Mistakes](14-common-mistakes.md) — Wire-related errors in common mistakes section
