> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Awo Framework — Architecture Overview

**Classification:** Specification — Tier 0
**Owner:** `00-overview/ARCH_OVERVIEW.md`
**Status:** Frozen at v1.0

---

## Purpose

This document is the entry point for understanding the Awo Framework's structure. It describes the five-layer architecture, the central primitive, the startup sequence, the package dependency graph, and the technology stack. Every other specification document assumes familiarity with this overview.

## Scope

This document covers the structural architecture of the framework. It does not specify the internals of any subsystem. Each subsystem has its own specification document.

## Dependencies

None. This document has no prerequisites.

---

## 1. What Awo Is

Awo is a Go-native framework for building multi-tenant ERP systems. It is not an application. It is a framework that module authors use to declare entities, and that the framework uses to automatically generate persistence, API routes, UI schemas, authorization policies, and workflow triggers.

**The central primitive is the `EntityDefinition`.** One `def.Register(&MyEntityDef)` call drives five subsystems simultaneously:

| Subsystem | What it does |
|-----------|-------------|
| **Persistence** | Routes SQL (system) or JSONB (custom) storage |
| **API** | Auto-generates CRUD + action Fiber routes |
| **UI** | Builds WidgetTree → amis JSON page schemas |
| **Authorization** | Compiles Casbin capability grants |
| **Workflows** | Binds Temporal workflow starts to lifecycle events |

Module authors declare. The framework executes.

---

## 2. Five-Layer Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  UI Layer                                                    │
│  amis JSON schemas served from API                          │
│  Generated dynamically from WidgetTree IR                   │
├─────────────────────────────────────────────────────────────┤
│  API Layer                                                   │
│  Fiber v2, middleware pipeline, thin route handlers         │
│  Routes auto-registered from EntityDefinition               │
├─────────────────────────────────────────────────────────────┤
│  Domain Layer                                                │
│  EntityDefinition, hooks, validators, permission policies   │
│  Stateless; zero external dependencies at interface level   │
├─────────────────────────────────────────────────────────────┤
│  Workflow Layer                                              │
│  Temporal workflows + activities (async, durable)           │
│  Dispatched via workflow_outbox, outside DB transaction     │
├─────────────────────────────────────────────────────────────┤
│  Store Layer                                                 │
│  EntityRepository interface → PostgreSQL via pgx            │
│  RLS enforced by set_tenant_context() on every request      │
└─────────────────────────────────────────────────────────────┘
```

**Dependency rule:** No layer may import from a higher layer. The domain layer has zero external dependencies at the interface level.

---

## 3. Technology Stack

| Component | Technology | Notes |
|-----------|-----------|-------|
| Language | Go | Minimum version: 1.22 |
| HTTP server | Fiber v2 | Fasthttp-based |
| Database | PostgreSQL 15+ | pgx/v5 driver |
| Connection pooling | PgBouncer | **Must be in transaction mode** |
| Session store | Redis 7+ | All cache, sessions, rate limiting |
| Workflows | Temporal | Durable orchestration |
| Logging | `log/slog` | Structured JSON |
| Currency | `shopspring/decimal` | `numeric(20,4)` in PostgreSQL |
| UUID | `google/uuid` | UUIDv7 for primary keys |
| Authorization | Casbin | Default; replaceable via PolicyEvaluator |
| UI renderer | amis (Baidu) | Pinned version in `web/sdk/` |
| Migrations | golang-migrate | `.up.sql` / `.down.sql` pairs |

---

## 4. Package Architecture

```
awo/
├── def/          Framework vocabulary (EntityDefinition, Actor, etc.)
├── filter/       Composable predicate DSL
├── auth/         Identity and authorization contracts
├── registry/     Definition validation and sealing
├── compiler/     EntityDefinitions → runtime descriptors
├── runtime/      Hook pipeline execution + service wiring
├── naming/       Naming series allocation
├── cache/        Cache and counter contracts
├── outbox/       Durable delivery contracts
├── audit/        Audit record contract
└── sdui/
    ├── widget/   WidgetTree IR (Node, NodeKind)
    └── amis/     amis JSON renderer
```

**Package Dependency Graph (acyclic):**

```
def         → (nothing)
filter      → (nothing)
cache       → (nothing)
outbox      → (nothing)
audit       → def
auth        → def
naming      → cache, def
registry    → def
compiler    → def, registry
sdui/widget → (nothing)
sdui/amis   → sdui/widget
sdui        → compiler, sdui/widget, sdui/amis, def, cache
runtime     → def, auth, compiler, naming, cache, outbox, audit
```

No circular dependencies exist. The graph is a strict DAG.

---

## 5. Multi-Tenancy Model

**Tenant-per-context**, not tenant-per-schema. A single PostgreSQL schema serves all tenants. Row Level Security (RLS) enforces isolation at the database level.

Every request flows through:
1. Tenant resolution (X-Tenant-ID header, subdomain, or query param)
2. `set_tenant_context($tenant_id)` — validates tenant is ACTIVE, sets GUC variable
3. All queries automatically filtered by `tenant_id = current_tenant_id()`

**PgBouncer MUST operate in transaction mode.** Session mode breaks the transaction-local GUC reset.

See [`04-multitenancy/RLS_SPEC.md`](../04-multitenancy/RLS_SPEC.md).

---

## 6. Startup Sequence

The following sequence is a hard dependency chain. Any failure before Fiber start is fatal (process exits).

```
1. Config load & validate (fail fast on missing required env vars)
       ↓
2. PostgreSQL pool init (ping; exit if unreachable)
       ↓
3. Redis client init (ping; exit if unreachable)
       ↓
4. EntityRegistry init
   → all modules call def.Register() via init()
   → Build() validates all definitions
   → Compile() produces CompiledSchema
       ↓
5. Fiber app init
   → middleware pipeline registered (fixed order)
   → routes registered from CompiledSchema.Routes
       ↓
6. Fiber server start
       ↓ (concurrent)
7. Temporal worker start (degraded if fails; CRUD still works)
```

**Failure behavior:**

| Component | Failure at Startup | Failure During Operation |
|-----------|--------------------|--------------------------|
| PostgreSQL | Fatal — process exits | 503 on all requests |
| Redis | Fatal — process exits | 503 on auth; cache degrades |
| EntityRegistry | Fatal — process exits | N/A (loaded once) |
| Temporal | Degraded — CRUD works | Workflow starts queue in outbox |

---

## 7. Middleware Pipeline

The middleware pipeline is executed in this fixed order for every request:

```
1. Request ID       — from X-Request-ID header or generated UUID
2. Structured log   — slog JSON, pre-seeds request fields
3. Panic recovery   — converts panics to HTTP 500
4. CORS             — per-tenant subdomain origin validation
5. Tenant resolve   — X-Tenant-ID / subdomain / query param
6. set_tenant_context() — validates tenant ACTIVE, sets RLS GUC
7. Session validate — Redis lookup, expiry check
8. ViewerContext    — injects auth.ViewerContext into context
9. Rate limiting    — Redis sliding window per tenant + user
```

Order is not configurable at runtime. Changing it requires code change + redeploy. This is intentional — the pipeline is a security boundary.

---

## 8. Request Lifecycle (Every Mutation)

```
HTTP Request
    │
    ▼
Middleware Pipeline (steps 1–9)
    │
    ▼
Route Handler (dispatched by CompiledSchema.Routes)
    │
    ▼
ASSEMBLE       — construct EntityRecord from request body
    │
    ▼
before_validate — module hooks run; may return ValidationError
    │
    ▼
VALIDATE       — required fields, type coercion, field validators
    │
    ▼
AUTHORIZE      — PolicyEvaluator.CanPerform(); returns 403 on deny
    │
    ▼
before_save    — module hooks; may return BusinessError
    │
    ▼
[TX begins]
    │
    ▼
PERSIST        — write to PostgreSQL (SQL or JSONB)
    │
    ▼
AUDIT RECORD   — write AuditRecord to audit_log (same TX)
    │
    ▼
after_save     — module hooks; error causes TX rollback
    │
    ▼
[TX commits]
    │
    ▼
Workflow start — outbox worker dispatches to Temporal (outside TX)
    │
    ▼
HTTP Response
```

See [`02-pipeline/LIFECYCLE_SPEC.md`](../02-pipeline/LIFECYCLE_SPEC.md) for the complete specification.

---

## 9. Entity Types

### System Entity
- SQL-backed: typed columns, FK constraints, CHECK constraints
- Required for: ledger entries, stock moves, payments, users, tenants, journal entries, tax entries
- Escalation threshold: any entity requiring financial integrity, inventory accounting, or IAM data

### Custom Entity
- JSONB-backed: fields stored in `custom_fields jsonb`
- Use for: tenant-specific schemas, frequently evolving fields, low write rates
- Escalate to system when: >10M records, financial calculations, FK constraints needed

---

## 10. Platform Modules

Seven modules ship with the framework. All are unconditional on every deployment:

| Module | Responsibility |
|--------|---------------|
| Tenant | Isolation boundary; tenant lifecycle state machine |
| IAM | Users, roles, sessions, authentication |
| Feature Flags | Per-tenant feature switches; Redis-cached |
| Settings | Hierarchical config: system → tenant → branch |
| Audit | Tamper-evident mutation history |
| Metadata | Runtime schema extension via custom fields |
| Module Registry | Tracks installed/activated business modules per tenant |

Platform modules use identical patterns to business modules. No special framework access.

---

## References

- [`CLAUDE.md`](../../CLAUDE.md) — Critical development rules (read first)
- [`00-overview/DECISION_REGISTER.md`](DECISION_REGISTER.md) — All 12 architectural decisions
- [`00-overview/GLOSSARY.md`](GLOSSARY.md) — Term definitions
- [`00-overview/PRINCIPLES.md`](PRINCIPLES.md) — Design philosophy
- [`00-overview/PACKAGE_DEPENDENCY_MAP.md`](PACKAGE_DEPENDENCY_MAP.md) — Full package DAG
- [`01-entity/ENTITY_DEFINITION_SPEC.md`](../01-entity/ENTITY_DEFINITION_SPEC.md) — EntityDefinition interface
- [`02-pipeline/LIFECYCLE_SPEC.md`](../02-pipeline/LIFECYCLE_SPEC.md) — Pipeline stages
- [`04-multitenancy/RLS_SPEC.md`](../04-multitenancy/RLS_SPEC.md) — RLS enforcement
