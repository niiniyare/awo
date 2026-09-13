> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Wire Provider Sets"
id: kern-009
status: accepted
category: SPEC
stability: STABLE
audience: [framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[def Package Reference](def-package-reference.md)"
  - "[ADR-023: Wire Dependency Injection](../17-adr/adr-023-wire-dependency-injection.md)"
  - "[SQLC Integration](../05-persistence/sqlc-integration.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Wire Provider Sets

**KERN-009 | Status: Accepted | Stability: Stable**

How Google Wire provider sets are organized in Awo, what each set contains, and how to add a new module's providers.

---

## 1. Wire Overview

Google Wire is a compile-time dependency injection code generator. It reads `wire.go` files containing provider functions and `wire.Build(...)` calls, then generates `wire_gen.go` with the concrete initialization code.

Awo uses Wire to assemble the full dependency graph at startup without reflection:

```
cmd/server/wire.go      ← wire.Build(...) — the composition root
cmd/server/wire_gen.go  ← generated — do not edit
```

---

## 2. Provider Set Organization

Provider sets are grouped by layer and module:

```
cmd/server/wire.go
    │
    ├── InfrastructureSet        ← pgx pool, Redis client, config
    ├── PlatformSet              ← platform module providers (IAM, Tenant, etc.)
    ├── CoreSet                  ← business module providers (Finance, HR, etc.)
    └── ServerSet                ← Fiber app, routes, middleware
```

Each set is a `wire.ProviderSet` variable defined in the relevant package.

---

## 3. Infrastructure Provider Set

```go
// internal/infra/wire.go

var InfrastructureSet = wire.NewSet(
    ProvidePgxPool,       // *pgxpool.Pool from DATABASE_URL
    ProvideRedisClient,   // *redis.Client from REDIS_URL
    ProvideConfig,        // *config.Config from env
    ProvideSQLCQuerier,   // sqlc.Querier (wraps pgx pool)
)

func ProvidePgxPool(cfg *config.Config) (*pgxpool.Pool, func(), error) {
    poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
    if err != nil {
        return nil, nil, fmt.Errorf("parse db url: %w", err)
    }
    // PgBouncer transaction mode: disable prepared statements
    poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

    pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
    if err != nil {
        return nil, nil, fmt.Errorf("create pgx pool: %w", err)
    }
    cleanup := func() { pool.Close() }
    return pool, cleanup, nil
}

func ProvideSQLCQuerier(pool *pgxpool.Pool) sqlc.Querier {
    return sqlc.New(pool)
}

func ProvideRedisClient(cfg *config.Config) (*redis.Client, func(), error) {
    opt, err := redis.ParseURL(cfg.RedisURL)
    if err != nil {
        return nil, nil, fmt.Errorf("parse redis url: %w", err)
    }
    client := redis.NewClient(opt)
    cleanup := func() { client.Close() }
    return client, cleanup, nil
}
```

---

## 4. Platform Module Provider Sets

Each platform module declares its own set:

```go
// internal/platform/iam/wire.go

var IAMSet = wire.NewSet(
    ProvideIAMRepository,
    ProvideIAMService,
    ProvideSessionStore,
    ProvideCasbinEnforcer,
)

func ProvideIAMRepository(q sqlc.Querier) *IAMRepository {
    return &IAMRepository{q: q}
}

func ProvideIAMService(repo *IAMRepository, sessions *SessionStore, enforcer *casbin.Enforcer) *IAMService {
    return &IAMService{repo: repo, sessions: sessions, enforcer: enforcer}
}

func ProvideSessionStore(redis *redis.Client) *SessionStore {
    return &SessionStore{client: redis}
}

func ProvideCasbinEnforcer(q sqlc.Querier) (*casbin.Enforcer, error) {
    adapter := casbinpgx.NewAdapter(q)
    enforcer, err := casbin.NewEnforcer(casbinModel, adapter)
    if err != nil {
        return nil, fmt.Errorf("casbin enforcer: %w", err)
    }
    return enforcer, nil
}
```

```go
// internal/platform/tenant/wire.go

var TenantSet = wire.NewSet(
    ProvideTenantRepository,
    ProvideTenantService,
)
```

The `PlatformSet` in `cmd/server/wire.go` aggregates them:

```go
var PlatformSet = wire.NewSet(
    iam.IAMSet,
    tenant.TenantSet,
    flags.FeatureFlagSet,
    settings.SettingsSet,
    audit.AuditSet,
    metadata.MetadataSet,
    registry.RegistrySet,
)
```

---

## 5. Core (Business) Module Provider Sets

Business modules follow the same pattern:

```go
// internal/core/finance/wire.go

var FinanceSet = wire.NewSet(
    ProvideInvoiceRepository,
    ProvideInvoiceService,
    ProvideCustomerRepository,
    ProvideCustomerService,
)

func ProvideInvoiceRepository(q sqlc.Querier) *InvoiceRepository {
    return &InvoiceRepository{q: q}
}

func ProvideInvoiceService(
    repo *InvoiceRepository,
    audit *audit.AuditService,
) *InvoiceService {
    return &InvoiceService{repo: repo, audit: audit}
}
```

```go
// cmd/server/wire.go

var CoreSet = wire.NewSet(
    finance.FinanceSet,
    hr.HRSet,
    inventory.InventorySet,
    crm.CRMSet,
)
```

---

## 6. Server Provider Set

```go
// internal/server/wire.go

var ServerSet = wire.NewSet(
    ProvideFiberApp,
    ProvideMiddleware,
    ProvideRouter,
)

func ProvideFiberApp(cfg *config.Config) *fiber.App {
    return fiber.New(fiber.Config{
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
    })
}

func ProvideRouter(
    app *fiber.App,
    iam *iam.IAMService,
    finance *finance.InvoiceService,
    // ... all services that register routes
    registry *registry.EntityRegistry,
) *Router {
    r := &Router{app: app, registry: registry}
    r.Register(iam)
    r.Register(finance)
    return r
}
```

---

## 7. The Composition Root

`cmd/server/wire.go` is the single place all sets are composed:

```go
//go:build wireinject

package main

import (
    "github.com/google/wire"
    "awo.so/internal/infra"
    "awo.so/internal/platform/iam"
    "awo.so/internal/platform/tenant"
    "awo.so/internal/core/finance"
    "awo.so/internal/server"
)

func InitializeApp(cfg *config.Config) (*App, func(), error) {
    wire.Build(
        infra.InfrastructureSet,
        PlatformSet,
        CoreSet,
        server.ServerSet,
        NewApp,
    )
    return nil, nil, nil  // wire replaces this
}
```

Running `wire ./cmd/server/` generates `wire_gen.go` with the concrete initialization sequence.

---

## 8. Adding a New Module

When adding a new business module (e.g. `internal/core/procurement`):

1. **Create `wire.go`** in the module:

```go
// internal/core/procurement/wire.go

package procurement

import "github.com/google/wire"

var ProcurementSet = wire.NewSet(
    ProvidePurchaseOrderRepository,
    ProvidePurchaseOrderService,
)

func ProvidePurchaseOrderRepository(q sqlc.Querier) *PurchaseOrderRepository {
    return &PurchaseOrderRepository{q: q}
}

func ProvidePurchaseOrderService(repo *PurchaseOrderRepository) *PurchaseOrderService {
    return &PurchaseOrderService{repo: repo}
}
```

2. **Add to `CoreSet`** in `cmd/server/wire.go`:

```go
var CoreSet = wire.NewSet(
    finance.FinanceSet,
    hr.HRSet,
    inventory.InventorySet,
    crm.CRMSet,
    procurement.ProcurementSet,  // ← add here
)
```

3. **Regenerate Wire**:

```bash
wire ./cmd/server/
```

4. **Verify `wire_gen.go`** contains the new providers in correct initialization order.

---

## 9. Provider Function Rules

| Rule | Why |
|---|---|
| Return concrete types, not interfaces (except at boundaries) | Wire needs to match types exactly |
| Return `(T, func(), error)` for resources that need cleanup | Wire chains cleanup functions |
| Return `(T, error)` for providers that can fail | Wire propagates errors to `InitializeApp` |
| Return `T` for infallible providers | Simplest signature |
| Never call `log.Fatal` in providers | Wire caller handles errors |
| No global state | All state flows through the dependency graph |

Cleanup functions (the `func()` return value) are called in reverse order when the app shuts down — Wire generates the correct teardown sequence.

---

## 10. Regenerating After Changes

```bash
# Regenerate wire_gen.go
wire ./cmd/server/

# wire_gen.go should be committed — it is the actual running code
git add cmd/server/wire_gen.go
```

If Wire fails with a missing provider error:
```
cannot find provider for *finance.InvoiceService
```

→ A dependency is not in any provider set reachable from `wire.Build`. Add a provider function or add the missing set to the composition root.

---

## Related Documents

- [def Package Reference](def-package-reference.md) — types providers supply to EntityDefinition consumers
- [ADR-023: Wire Dependency Injection](../17-adr/adr-023-wire-dependency-injection.md) — why Wire was chosen
- [SQLC Integration](../05-persistence/sqlc-integration.md) — `sqlc.Querier` wired via `InfrastructureSet`
- [Local Development Setup](../14-operations/local-development.md) — running wire locally
