> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-023: Google Wire for Dependency Injection"
id: adr-023
status: accepted
category: ADR
stability: STABLE
audience: [framework-authors, module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Wire Dependency Injection](../16-module-dev-guide/17-wire-dependency-injection.md)"
  - "[Module Boundary Rules](../02-architecture/module-boundaries.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# ADR-023: Google Wire for Dependency Injection

**Status**: Accepted
**Date**: 2024-01-15
**Deciders**: Awo Framework Team

---

## Context

Awo has many components with complex dependency graphs:

- HTTP handlers depend on service interfaces
- Service interfaces depend on EntityRepositories
- EntityRepositories depend on pgx connection pools
- Hooks and policies depend on service interfaces from other modules
- Temporal activities depend on external clients (email, SMS, storage)

Without a systematic approach, dependency construction becomes ad-hoc, difficult to test, and prone to "wiring" bugs (wrong dependency passed to wrong component).

The options considered:

1. **Manual wiring** — write `New...()` constructors, call them in `main.go`
2. **Reflection-based DI** (e.g., Uber Fx, dig) — runtime, magical
3. **Code-generation DI** (Google Wire) — compile-time, explicit

---

## Decision

Use **Google Wire** for all dependency injection in Awo.

Wire generates Go code at compile time. The generated code is ordinary Go — readable, debuggable, not magic.

---

## Consequences

### Positive

- **Compile-time safety**: missing providers are compile errors, not runtime panics
- **No reflection overhead**: generated code is plain function calls
- **Explicit graph**: the generated `wire_gen.go` shows every dependency path
- **Testable**: swap real providers with mock providers in tests
- **Readable output**: `wire_gen.go` is checked in — readable, greppable

### Negative

- **Build step**: `wire gen ./...` must run before `go build`
- **Learning curve**: Wire's provider/injector mental model takes time
- **Verbose for small projects**: overkill for fewer than ~10 dependencies

### Neutral

- Wire is a Google-maintained project with broad industry adoption
- The generated `wire_gen.go` is committed to source control — PR reviewers see exact wiring

---

## Alternatives Rejected

### Manual `main.go` Construction

```go
// Gets unwieldy fast
pool := newPool(cfg)
redisClient := newRedisClient(cfg)
customerRepo := crm.NewCustomerRepository(pool)
invoiceRepo := finance.NewInvoiceRepository(pool)
customerSvc := crm.NewCustomerService(customerRepo)
invoiceHook := finance.NewInvoiceCreditGuard(customerSvc)
// ... 50 more lines
```

Rejected: brittle, hard to test, dependency order errors are silent.

### Uber Fx (reflect-based)

Rejected: runtime errors instead of compile-time errors; magic `fx.Provide` / `fx.Invoke` harder to trace than generated code; adds reflection overhead at startup.

### Global Singletons

```go
var CustomerRepo = crm.NewCustomerRepository(GlobalPool)
```

Rejected: impossible to mock in tests; race conditions in test parallelism; violates Law of Demeter.

---

## Implementation Constraint

Wire provider sets are organized per module:

```go
// internal/core/finance/wire.go
var FinanceProviderSet = wire.NewSet(
    NewInvoiceRepository,
    NewInvoiceService,
    NewInvoiceCreditGuard,
    wire.Bind(new(CustomerService), new(*crm.FinanceCustomerAdapter)),
)
```

Top-level injector lives in `cmd/server/wire.go`. The `wire gen` command is run by CI before `go build`.

---

## Related Documents

- [Wire Dependency Injection](../16-module-dev-guide/17-wire-dependency-injection.md) — how to use Wire in module development
- [Module Boundary Rules](../02-architecture/module-boundaries.md) — why interfaces (not concrete types) cross module boundaries
