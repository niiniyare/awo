> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Five-Layer Dependency Rules"
id: arch-006
status: accepted
category: SPEC
stability: FROZEN
audience: [module-authors, framework-authors, contributors]
since: "1.0"
normative-level: normative
related:
  - "[Five-Layer Architecture](five-layer.md)"
  - "[Module Boundaries](module-boundaries.md)"
  - "[Architecture Laws](laws.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Five-Layer Dependency Rules

**ARCH-006 | Status: Accepted | Stability: Frozen**

Precise import rules for each layer: what it may import, what it must not import, and why.

---

## Layer Stack

```
UI Layer      (amis JSON — not Go code)
API Layer     (Fiber v2 route handlers)
Domain Layer  (EntityDefinition, hooks, validators, policies)
Workflow Layer (Temporal workflows + activities)
Store Layer   (EntityRepository implementations, pgx)
```

Dependencies flow **top-down only**. No layer may import from a higher layer.

---

## 1. UI Layer

**Technology**: amis JSON schemas (Go `[]byte` or `map[string]any`)

**May import**: Nothing — UI layer has no Go imports. It is data produced by the API layer.

**Must not**: Contain Go business logic. All decisions are made before the schema is generated.

---

## 2. API Layer

**Location**: `internal/core/{module}/handler.go`, `internal/platform/{module}/handler.go`

**May import**:
- Domain layer types (`def.*`, entity structs, service interfaces)
- Shared utilities (`internal/shared/errors`, `internal/shared/pagination`)
- `github.com/gofiber/fiber/v2`
- `context`, `fmt`, `log/slog`, standard library

**Must NOT import**:
- Store layer directly (`pgx`, `pgxpool`, repository implementations)
- Workflow layer (`go.temporal.io/sdk`)
- Other modules' handlers
- Other modules' repositories (use service interfaces)

**Rationale**: Handlers are thin (~50 lines). Business logic belongs in the domain layer. Storage access belongs in the store layer (via the repository interface injected at startup).

---

## 3. Domain Layer

**Location**: `internal/core/{module}/def.go`, `hooks.go`, `policy.go`, `validators.go`

**May import**:
- `awo.so/awo/def` — EntityDefinition, FieldDef, HookSet, etc.
- `awo.so/awo/filter` — Filter DSL predicates
- `awo.so/awo/session` — ActorFromContext, TenantIDFromContext
- `github.com/shopspring/decimal` — for decimal.Decimal in hooks
- `github.com/google/uuid`
- `context`, `fmt`, `time`, standard library
- Other modules' **service interfaces** (declared in the consuming module)

**Must NOT import**:
- `pgx` or any database driver
- `go.temporal.io/sdk`
- `github.com/gofiber/fiber/v2`
- Another module's concrete types (`internal/core/finance/...` from within `internal/core/hr/`)
- Any infrastructure package (`redis`, `aws-sdk`, `sendgrid`)

**Rationale**: The domain layer must be testable without any infrastructure. Zero external dependencies enables unit tests that run in milliseconds with mock repositories.

---

## 4. Workflow Layer

**Location**: `internal/core/{module}/workflows/`, `internal/core/{module}/activities/`

**May import**:
- `go.temporal.io/sdk/workflow`, `go.temporal.io/sdk/activity`
- Domain layer types (for input/output structs)
- Service interfaces (for activities)
- `awo.so/awo/def` — for EntityRepository in activities
- `context`, `fmt`, standard library

**Must NOT import**:
- `github.com/gofiber/fiber/v2`
- Direct pgx (use EntityRepository interface)
- Other modules' workflow functions directly (use Temporal signals/queries)

**Special rules for workflow functions** (determinism requirements):
- No `time.Now()` → `workflow.Now(ctx)`
- No `time.Sleep()` → `workflow.Sleep(ctx, duration)`
- No `rand` → `workflow.SideEffect`
- No direct I/O → all in activities

**Special rules for activity functions**:
- MUST call `store.SetTenantContextFromCtx(ctx)` before any DB operation
- MAY use external APIs, file I/O, network calls — activities are the I/O layer

---

## 5. Store Layer

**Location**: `framework/store/`, `internal/platform/{module}/store.go`

**May import**:
- `github.com/jackc/pgx/v5`
- `github.com/jackc/pgx/v5/pgxpool`
- `awo.so/awo/def` — EntityRepository interface + types
- `awo.so/awo/filter` — Filter DSL translation to SQL
- `github.com/shopspring/decimal`
- Standard library

**Must NOT import**:
- Domain layer business logic
- API layer (Fiber)
- Workflow layer (Temporal)
- Any module-specific code (`internal/core/...`)

**Rationale**: The store layer is the implementation of the EntityRepository interface. It translates abstract operations (Filter DSL, QueryOption) to SQL. Module-specific knowledge does not belong here.

---

## Import Violation Detection

CI enforces import rules via a Go import graph checker. Violations fail the build:

```bash
# Checks run in CI
go run tools/import-checker/main.go ./...
```

The checker specifically:
1. Verifies domain layer packages do not import pgx or fiber
2. Verifies workflow functions do not import fiber
3. Verifies handlers do not import pgx directly
4. Verifies no circular imports between modules

---

## Permitted Cross-Cutting Imports

These packages may be imported from any layer:

| Package | Purpose |
|---|---|
| `awo.so/awo/def` | Core types |
| `awo.so/awo/filter` | Filter DSL |
| `awo.so/awo/session` | Actor/tenant context |
| `awo.so/awo/errors` | Shared error types |
| `internal/shared/errors` | HTTP error mapping |
| `internal/shared/pagination` | PageInfo types |
| `github.com/shopspring/decimal` | Monetary amounts |
| `github.com/google/uuid` | UUIDs |
| Standard library | Always |

---

## Related Documents

- [Five-Layer Architecture](five-layer.md) — conceptual layer description
- [Module Boundaries](module-boundaries.md) — cross-module import rules
- [Architecture Laws](laws.md) — LAW-008 (domain layer zero external deps)
- [Wire Dependency Injection](../16-module-dev-guide/17-wire-dependency-injection.md) — how cross-layer deps are wired
