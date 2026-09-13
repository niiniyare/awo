> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Module Boundary Rules"
id: arch-004
status: accepted
category: SPEC
stability: FROZEN
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[Architecture Laws](laws.md)"
  - "[Five-Layer Architecture](five-layer.md)"
  - "[Module System](../10-modules/module-system.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Module Boundary Rules

**ARCH-004 | Status: Accepted | Stability: Frozen**

This document specifies the rules governing what modules may import from each other, how cross-module data access is performed, and what constitutes a boundary violation.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. The Boundary Rule

> **Modules MUST NOT import each other's repository packages directly.**

Each module owns its data. Cross-module data access goes through service interfaces — never through repository cross-imports.

```go
// WRONG: Finance imports CRM's repository
import "awo.so/internal/core/crm/repo"  // in finance package

func (s *FinanceService) GetCustomer(ctx context.Context, id uuid.UUID) (*Customer, error) {
    return crm_repo.Get(ctx, id)  // boundary violation
}

// CORRECT: Finance calls CRM via a service interface
type CustomerService interface {
    GetCustomer(ctx context.Context, id uuid.UUID) (*crm.Customer, error)
}

func (s *FinanceService) GetCustomer(ctx context.Context, id uuid.UUID) (*crm.Customer, error) {
    return s.CRMService.GetCustomer(ctx, id)  // correct
}
```

---

## 2. Permitted Cross-Module Imports

| What | Permitted | Reason |
|---|---|---|
| Domain types (structs, enums) from another module | Yes | Read-only types; no behavior |
| Service interfaces from another module | Yes | Interface definitions carry no implementation |
| Repository implementations from another module | **No** | Repository ownership belongs to the declaring module |
| HTTP handlers from another module | **No** | Each module's handlers are internal |
| Migration files from another module | **No** | Schema ownership belongs to declaring module |

```go
// CORRECT: import CRM's domain types and service interface
import (
    "awo.so/internal/core/crm/domain"    // types: Customer, Contact
    "awo.so/internal/core/crm/service"   // interface: CustomerService
)
```

---

## 3. Service Interface Pattern

Each module exposes a service interface for data that other modules legitimately need:

```go
// internal/core/crm/service/interface.go

type CustomerService interface {
    GetCustomer(ctx context.Context, id uuid.UUID) (*domain.Customer, error)
    ListCustomersByStatus(ctx context.Context, status string) ([]*domain.Customer, error)
}
```

```go
// internal/core/crm/service/customer_service.go — implements the interface

type customerService struct {
    repo entity.EntityRepository[Customer]
}

func (s *customerService) GetCustomer(ctx context.Context, id uuid.UUID) (*domain.Customer, error) {
    return s.repo.Get(ctx, id)
}
```

The Finance module receives `service.CustomerService` via dependency injection (wire) — it never imports `crm/repo`.

---

## 4. Cross-Module Link Fields

`FieldLink` between entities in different modules is permitted:

```go
// In finance module: invoice links to crm_contact
{
    Name:       "customer",
    Type:       def.FieldLink,
    LinkTarget: "crm_contact",  // cross-module link — OK
    Required:   true,
}
```

The framework resolves link field values (UUID → entity record) via the EntityRegistry, not via direct module imports. The Finance module does not import the CRM module to resolve customer links.

---

## 5. Shared Domain Types

Types shared across modules live in `internal/shared/` — not in any module:

```go
// internal/shared/types/money.go
type Money = decimal.Decimal  // alias with semantic meaning

// internal/shared/types/period.go
type AccountingPeriod struct {
    Year  int
    Month time.Month
}
```

Module-specific types that happen to be needed by another module should be moved to `internal/shared/` rather than creating a cross-module import.

---

## 6. Platform Module Access

Business modules MAY import platform module service interfaces:

```go
// Finance module uses IAM for actor context — permitted
import "awo.so/internal/platform/iam/service"

actor := iamService.ActorFromContext(ctx)
```

Platform modules MUST NOT import business module packages (IAM must not import Finance).

Platform module access goes through service interfaces, not repository imports — same rule as cross-business-module access.

---

## 7. Enforcement

Module boundary violations are detected by:

1. **Go import graph analysis** (CI): `go list -f '{{.Deps}}' ./...` checked against allowed import paths
2. **Code review checklist** (MDG-012): module boundary section
3. **Integration test isolation**: each module's tests inject mocked interfaces

A boundary violation does not always cause a compilation error — it often compiles successfully but creates tight coupling that breaks when the imported module changes. This is why automated import graph checks are required.

---

## 8. Anti-Patterns

### Repository Aliasing

```go
// WRONG: aliasing another module's repository to "own" it
type FinanceCustomerRepo = crm.ContactRepository  // type alias — still a boundary violation
```

### Shared Database Transactions Across Modules

```go
// WRONG: Finance and CRM sharing a transaction directly
crmRepo.WithTx(ctx, func(ctx context.Context, crmTxRepo ...) error {
    financeRepo.Create(ctx, ...)  // Finance using CRM's transaction
})

// CORRECT: coordinate via Temporal Saga if cross-module atomic operations are needed
// Or: design the domain so atomic cross-module operations are not required
```

### Module-Level Globals Accessed Cross-Module

```go
// WRONG: package-level variable accessed from another module
var ActiveCustomers []*Customer  // global in CRM, accessed by Finance

// CORRECT: pass data through function parameters or service interface calls
```

---

## Related Documents

- [Architecture Laws](laws.md) — LAW-008 (import direction), LAW-019 (no cross-module repo import)
- [Five-Layer Architecture](five-layer.md) — layer-level import rules
- [Module System](../10-modules/module-system.md) — ModuleManifest dependency declarations
- [Module Dev Guide §1](../16-module-dev-guide/01-module-scaffold.md) — directory layout and package structure
