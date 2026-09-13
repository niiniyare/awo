> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Security Model

## Isolation Boundaries

Awo enforces two independent isolation boundaries. Understanding the distinction is critical for implementing secure ERP modules.

### Boundary 1 — Tenant Isolation (Framework)

**Mechanism:** PostgreSQL Row-Level Security
**Enforced by:** Framework (`set_tenant_context()` + RLS policies)
**Guarantee:** Data for tenant A never appears in tenant B's queries

The framework guarantees `WHERE tenant_id = current_tenant_id()` on every RLS-enabled table. This is a hard, kernel-level enforcement. Application code cannot bypass it — not even with a bug.

Tables with RLS: all business entity tables, IAM tables, audit log, settings, flags, notifications.
Tables without RLS: `platform_tenant` (bootstrap), reference data, organization tables (see below).

### Boundary 2 — Organization Scope (Application)

**Mechanism:** `OrganizationService.ResolveScope()` → explicit `IN` predicate
**Enforced by:** Application services
**Guarantee:** Users see only the org nodes their `VisibilityMode` permits

Organization visibility is evaluated entirely in Go. The result is a `[]uuid.UUID` that application services append as an explicit `IN` predicate before calling any repository method.

Organization tables (`platform_organization`, `platform_org_type`, `platform_org_assignment`) carry **standard tenant RLS** — `tenant_id = current_tenant_id()` — identical to every other business entity table. They carry no org-visibility RLS predicates. RLS handles tenant boundaries. The application handles org visibility.

**Why no org-visibility RLS?**
Org scope is context-dependent and composable — it depends on the user's active org, their assignment set, and their visibility mode. RLS cannot express this without per-user session variables for each dimension, which is fragile and hard to audit. Go code is testable, traceable, and easy to change independently of schema migrations.

---

## Authentication

Session lifecycle:

1. `POST /api/v1/auth/login` → validates credentials, creates `iam_session`.
2. Session token hashed with SHA-256; only the hash is stored in DB.
3. Token stored in Redis (`session:{hash}`, TTL = session expiry).
4. Every subsequent request: middleware validates token against Redis.
5. Redis failure → **503** (not 401). Cannot authenticate without session store — this is correct security behavior.
6. `POST /api/v1/auth/logout` → deletes Redis key + marks DB session inactive.

**Redis is required for session validation.** Redis failure is not a degraded-mode scenario for auth — it is a hard failure. Do not add a DB fallback path for session lookup.

---

## Authorization Pipeline

```
Request
  ↓
Auth middleware: resolve tenant → set_tenant_context()
  ↓
Auth middleware: validate session token (Redis)
  ↓
Auth middleware: load ViewerContext (user, roles, org assignments)
  ↓
Stage 1: Tenant RLS (DB kernel — automatic)
  ↓
Stage 2: Org scope — OrganizationService.ResolveScope() → []uuid.UUID
  ↓
RBAC — Casbin: enforce(user, tenant, entity_type, action)
  ↓
Policy — EntityDefinition.Policy: row-level predicate
  ↓
Repository.Query()
```

See [authorization.md](./authorization.md) for full detail.

---

## Token Security

| Token Type | Storage | Hashing | Expiry |
|---|---|---|---|
| Session token | Redis (hash only) | SHA-256 | Configurable (default 8h) |
| API token | DB `iam_api_token` (hash only) | SHA-256 | Explicit expiry or never |
| Refresh token | Not implemented in v1.0 | — | — |

Raw tokens are never stored. Only the SHA-256 hash. If the Redis store is compromised, stolen hashes cannot be reversed to valid tokens.

---

## RBAC (Casbin)

Policy model: `(subject, domain, object, action)`

- **Subject**: `user:{uuid}` or `role:{name}`
- **Domain**: tenant UUID or `_platform_`
- **Object**: entity type name
- **Action**: `read`, `write`, `create`, `delete`, `submit`, `cancel`, …

System roles (seeded at tenant bootstrap, cannot delete):

| Role | Scope |
|---|---|
| `role:platform-admin` | Bypasses Casbin. Full access to all tenants. |
| `role:tenant.admin` | Full access within tenant. `VisibilityEntireTenant`. |
| `role:tenant.user` | Standard access. Org scope applies. |
| `role:api-client` | Machine-to-machine. Limited scopes. |

---

## SQL Injection Prevention

All query predicates are constructed as typed `filter.Filter` values in Go. The `contrib/pgx/sqlbuild` package translates filters to parameterized SQL. No string interpolation is used in SQL construction. Application code never concatenates user input into SQL strings.

---

## Sensitive Fields

Fields marked `Sensitive: true` in `FieldDef`:

- Excluded from structured log output
- Excluded from standard API response payloads unless explicitly requested
- Included in audit log write (raw value captured at mutation time)

Examples: `trial_ends_at`, `suspended_at`, `suspension_reason`, password hashes, token hashes.

---

## Stack Trace Policy

Internal stack traces are never exposed to API clients. The error envelope:

```json
{"error": {"code": "internal_error", "message": "An unexpected error occurred"}}
```

Full error context (including stack trace if available) is written to the structured log with `request_id`, `tenant_id`, and `user_id` for correlation.

---

## Known Risks (v1.0)

| Risk | Severity | Mitigation |
|---|---|---|
| Org scope enforcement requires correct application code | Medium | Framework provides primitives; not enforced at DB level — module authors must call ResolveScope |
| `fakestore` does not enforce tenant isolation | Low | Test-only; integration tests use real Postgres |
| Casbin enforcer not yet wired to CompiledSchema | Medium | Routes unprotected until v1.1 wiring; mitigate with explicit permission checks in handlers |
| Redis connection failure → 503 (intentional) | Low | Documented behavior; surface to ops via health check |
