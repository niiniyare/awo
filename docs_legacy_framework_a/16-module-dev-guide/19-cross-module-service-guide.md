> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Cross-Module Service Interfaces"
id: mdg-019
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: normative
related:
  - "[Module Boundary Rules](../02-architecture/module-boundaries.md)"
  - "[Wire Dependency Injection](17-wire-dependency-injection.md)"
  - "[Testing Patterns](15-testing-patterns.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Cross-Module Service Interfaces

**MDG-019 | Status: Accepted | Stability: Stable**

Step-by-step guide for declaring, implementing, binding, and testing a cross-module service interface.

---

## 1. When to Use a Cross-Module Interface

Use a cross-module service interface when:

- Module A needs data owned by Module B
- Module A needs to trigger a side effect in Module B
- The dependency direction is stable (B doesn't need A)

**Do not use** a cross-module interface when:
- The data is in a global table accessible to all modules
- The operation can be done via a Temporal activity that calls the target module's repository directly (each activity sets its own tenant context)
- The coupling is temporal (event-driven) — use the outbox pattern instead

---

## 2. Step 1: Declare the Interface in the Consumer

The consuming module (A) declares what it needs. It does not reference the providing module (B) anywhere:

```go
// internal/core/finance/service_interfaces.go
package finance

import (
    "context"
    "github.com/google/uuid"
    "github.com/shopspring/decimal"
)

// EmployeeService provides HR employee data to the Finance module.
// Implemented by the HR module — Finance never imports hr package.
type EmployeeService interface {
    // GetSalary returns the current monthly gross salary for an employee.
    GetSalary(ctx context.Context, employeeID uuid.UUID) (decimal.Decimal, error)

    // GetEmployeeByUserID resolves the HR employee for a given IAM user.
    GetEmployeeByUserID(ctx context.Context, userID uuid.UUID) (EmployeeInfo, error)
}

// EmployeeInfo contains the subset of HR data Finance needs.
// Defined in Finance — no dependency on HR types.
type EmployeeInfo struct {
    EmployeeID uuid.UUID
    Name       string
    Department string
}
```

Rule: all types in the interface are defined in the consuming module (`finance`), not the providing module (`hr`). This prevents Finance from transitively depending on HR's internal types.

---

## 3. Step 2: Implement the Interface in the Provider

The HR module implements the interface declared in Finance. The implementation lives in the `hr` package (since it owns the data), in a file named `finance_adapter.go` to make the coupling explicit:

```go
// internal/core/hr/finance_adapter.go
package hr

import (
    "context"
    "fmt"
    "github.com/google/uuid"
    "github.com/shopspring/decimal"
    "awo.so/awo/def"
    "awo.so/awo/filter"
    "awo.so/internal/core/finance"  // import Finance to satisfy its interface
)

// FinanceEmployeeAdapter implements finance.EmployeeService.
// HR module exports this adapter for use by the Finance module.
type FinanceEmployeeAdapter struct {
    employeeRepo def.EntityRepository[Employee]
}

func NewFinanceEmployeeAdapter(repo def.EntityRepository[Employee]) finance.EmployeeService {
    return &FinanceEmployeeAdapter{employeeRepo: repo}
}

func (a *FinanceEmployeeAdapter) GetSalary(ctx context.Context, employeeID uuid.UUID) (decimal.Decimal, error) {
    employee, err := a.employeeRepo.Get(ctx, employeeID)
    if err != nil {
        return decimal.Zero, fmt.Errorf("FinanceEmployeeAdapter.GetSalary: %w", err)
    }
    return employee.GetDecimal("gross_salary"), nil
}

func (a *FinanceEmployeeAdapter) GetEmployeeByUserID(ctx context.Context, userID uuid.UUID) (finance.EmployeeInfo, error) {
    employees, _, err := a.employeeRepo.Query(ctx, filter.Eq("user_id", userID))
    if err != nil {
        return finance.EmployeeInfo{}, fmt.Errorf("FinanceEmployeeAdapter.GetEmployeeByUserID: %w", err)
    }
    if len(employees) == 0 {
        return finance.EmployeeInfo{}, def.ErrNotFound
    }
    emp := employees[0]
    return finance.EmployeeInfo{
        EmployeeID: emp.ID,
        Name:       emp.GetString("name"),
        Department: emp.GetString("department"),
    }, nil
}
```

---

## 4. Step 3: Wire the Interface

In `cmd/server/wire.go`, bind the HR adapter to the Finance interface:

```go
// cmd/server/wire.go
//go:build wireinject

package main

import (
    "github.com/google/wire"
    "awo.so/internal/core/finance"
    "awo.so/internal/core/hr"
)

func InitializeFinanceModule(pool *pgxpool.Pool, redis *redis.Client) (*finance.Module, error) {
    wire.Build(
        // HR providers
        hr.NewEmployeeRepository,
        hr.NewFinanceEmployeeAdapter,     // provides finance.EmployeeService
        // Finance providers
        finance.NewPayrollHook,           // depends on finance.EmployeeService
        finance.NewModule,
    )
    return nil, nil
}
```

Wire infers the type binding: `hr.NewFinanceEmployeeAdapter` returns `finance.EmployeeService`, which `finance.NewPayrollHook` requires.

If the type binding is not inferred automatically, add an explicit `wire.Bind`:

```go
wire.Bind(new(finance.EmployeeService), new(*hr.FinanceEmployeeAdapter)),
```

---

## 5. Step 4: Use the Interface in Finance

```go
// internal/core/finance/hooks.go
package finance

type PayslipGeneratorHook struct {
    EmployeeSvc EmployeeService  // interface — no hr package import
}

func (h *PayslipGeneratorHook) BeforeCreate(ctx context.Context, rec *def.EntityRecord) error {
    employeeID, _ := rec.GetUUID("employee")
    salary, err := h.EmployeeSvc.GetSalary(ctx, employeeID)
    if err != nil {
        return fmt.Errorf("PayslipGeneratorHook.BeforeCreate: get salary: %w", err)
    }
    rec.Set("gross_salary", salary)
    return nil
}
```

---

## 6. Step 5: Test with a Mock

```go
// internal/core/finance/hooks_test.go
package finance_test

import (
    "testing"
    "context"
    "github.com/shopspring/decimal"
    "github.com/google/uuid"
    "awo.so/internal/core/finance"
)

type mockEmployeeService struct {
    salary decimal.Decimal
    err    error
}

func (m *mockEmployeeService) GetSalary(_ context.Context, _ uuid.UUID) (decimal.Decimal, error) {
    return m.salary, m.err
}

func (m *mockEmployeeService) GetEmployeeByUserID(_ context.Context, _ uuid.UUID) (finance.EmployeeInfo, error) {
    return finance.EmployeeInfo{}, nil
}

func TestPayslipGeneratorHook_SetsGrossSalary(t *testing.T) {
    hook := &finance.PayslipGeneratorHook{
        EmployeeSvc: &mockEmployeeService{salary: decimal.NewFromFloat(75000)},
    }

    rec := &def.EntityRecord{}
    rec.Set("employee", uuid.New())

    if err := hook.BeforeCreate(context.Background(), rec); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    got := rec.GetDecimal("gross_salary")
    if !got.Equal(decimal.NewFromFloat(75000)) {
        t.Errorf("expected 75000, got %s", got)
    }
}
```

No HR package imported. No database needed. Fast, isolated, reliable.

---

## 7. Anti-Patterns

### Provider importing Consumer

```go
// WRONG: HR imports Finance to return Finance types directly
package hr

import "awo.so/internal/core/finance"  // reverse dependency!

func (a *HRService) GetSalaryForFinance() finance.EmployeeInfo { ... }
```

The providing module must never import the consuming module. The adapter file in the providing module imports the consuming module's interface types only — this is the one-way bridge.

### Interface in Shared Package

```go
// WRONG: interface in shared — forces HR to depend on shared/finance-types
// internal/shared/finance_interfaces.go
type EmployeeService interface { ... }
```

Interfaces belong in the consuming module. If multiple modules need the same data from HR, each declares its own interface with only the methods it needs.

### Fat Interface

```go
// WRONG: Finance declares everything HR knows
type EmployeeService interface {
    GetSalary(...)
    GetLeaveBalance(...)
    GetAttendance(...)
    GetPerformanceReview(...)
    GetEmergencyContacts(...)
    // ... 20 more methods
}
```

Declare only what Finance actually uses. Narrow interfaces are easier to mock and less brittle to changes in HR.

---

## Related Documents

- [Module Boundary Rules](../02-architecture/module-boundaries.md) — the architectural rule
- [Wire Dependency Injection](17-wire-dependency-injection.md) — binding interfaces with Wire
- [Testing Patterns](15-testing-patterns.md) — mocking interfaces in unit tests
- [Module Checklist (Extended)](16-module-checklist.md) — Gate 1: interface declaration review
