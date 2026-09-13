# Security Model

**Classification:** Specification — Tier 1
**Owner:** `18-security/SECURITY_MODEL.md`
**Status:** Frozen at v1.0

---

## Purpose

This document describes the Awo Framework's defence-in-depth security model — the layers that together prevent data leakage, privilege escalation, injection, and session hijacking.

---

## 1. Threat Model

Primary threats:

| Threat | Mitigated By |
|--------|-------------|
| Cross-tenant data access | RLS + `set_tenant_context()` + no raw SQL in app code |
| Privilege escalation | Casbin RBAC + PolicyEvaluator abstraction |
| SQL injection | No raw SQL in business logic; all persistence via EntityRepository |
| Session hijacking | Short-lived tokens; Redis-backed revocation; device/IP binding |
| Sensitive data exposure | `Sensitive: true` field exclusion from logs and SDUI |
| Workflow/event duplication | Idempotency keys; Temporal deduplication |
| Audit log tampering | No UPDATE/DELETE privileges on `audit_log` for `awo_app` role |
| Secret leakage | Env vars / Vault; never in code or logs |

---

## 2. Layer 1 — Network

- API endpoints: HTTPS only (TLS termination at load balancer).
- `/metrics`, `/health/*`: internal network only (not exposed via ingress).
- PgBouncer: internal network only; no direct PostgreSQL access from internet.
- Redis: internal network only; `requirepass` enforced.

---

## 3. Layer 2 — Tenant Isolation (RLS)

Every tenant-scoped table has:

```sql
ALTER TABLE t ENABLE ROW LEVEL SECURITY;
ALTER TABLE t FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON t USING (tenant_id = current_tenant_id());
```

`current_tenant_id()` reads `app.current_tenant_id` — a transaction-local GUC variable set by the `set_tenant_context()` stored procedure. The procedure validates tenant existence and `status = 'ACTIVE'` before setting the GUC.

PgBouncer MUST run in transaction mode so the GUC resets on every transaction boundary, preventing cross-request tenant context leakage.

**There is no application code path that bypasses RLS.** The `awo_app` PostgreSQL role is subject to RLS for all tenant-scoped tables. The framework never connects with a superuser role for application queries.

---

## 4. Layer 3 — Session Authentication

Session tokens are:
- 256-bit random values from `crypto/rand`
- Base64url encoded
- Stored in Redis as `session:{token}` → `auth.Session` JSON
- Never stored in application database
- Compared with `crypto/subtle.ConstantTimeCompare` (timing-safe)

Session validation (in middleware):
1. Extract `Authorization: Bearer {token}` header
2. Lookup Redis key `session:{token}`
3. Deserialise `auth.Session`
4. Check `Session.ExpiresAt` — reject if expired
5. Validate `TenantID` matches `X-Tenant-ID` header
6. Validate `IPAddress` if `strict_ip_binding` feature flag enabled
7. Attach `auth.ViewerContext` to request context via `auth.WithViewer()`

Redis failure → HTTP 503 (session validation is a hard dependency).

---

## 5. Layer 4 — RBAC (Operation Gate)

Before any entity operation, the framework checks:

```
CanPerform(viewer.TenantID(), viewer.Roles(), entityType, action) → bool
```

`PolicyEvaluator` (ADR-001) is the abstraction. The default implementation uses Casbin. The permission check runs before the database query — unauthorised requests never touch the database.

`CapabilityGrant` records (compiled from `PermissionSet` on `EntityDefinition`) are loaded into Casbin at startup. No runtime policy editing.

---

## 6. Layer 5 — Privacy Policies (Row Filter)

RBAC controls which operations are allowed. Privacy policies control which rows are visible within allowed operations.

`PolicyFunc` on `EntityDefinition` injects additional WHERE predicates into every query:

```go
Policy: def.PolicyFunc(func(ctx context.Context) def.Filter {
    actor := auth.ViewerFromContext(ctx).Actor()
    return filter.Eq("assigned_to", actor.UserID)
}),
```

Privacy policy is applied by the EntityRepository implementation. It cannot be bypassed from application code.

---

## 7. Layer 6 — Audit

Every entity mutation (when `AuditEnabled: true`) writes an `AuditRecord` within the same database transaction. The record is immutable — the `awo_app` role has no `UPDATE` or `DELETE` on `audit_log`.

Sensitive fields are excluded from audit records (same exclusion list as SDUI and logging).

---

## 8. Layer 7 — Rate Limiting

Per-tenant and per-user sliding window rate limiting via Redis counters:

```
rl:{tenant_id}:{user_id}:{window_start} → count
```

Default: 1000 requests per minute per user. Configurable per tenant via Settings module.

Exceeding the limit: HTTP 429 with `Retry-After` header.

---

## 9. Layer 8 — Input Sanitisation

All entity field values pass through the validation pipeline before persistence:
- Type-level validation (string, number, date format)
- Field-level `FieldValidator` functions
- Hook-level `BeforeCreate`/`BeforeUpdate` validators

There is no raw SQL execution from application code. EntityRepository implementations use parameterised queries only. SQL injection is structurally prevented.

---

## 10. OWASP Top 10 Coverage

| OWASP Category | Mitigated By |
|---------------|-------------|
| A01: Broken Access Control | RLS + RBAC + PolicyEvaluator |
| A02: Cryptographic Failures | `crypto/rand` tokens; TLS; never store sensitive fields in logs |
| A03: Injection | No raw SQL; parameterised queries only; EntityRepository abstraction |
| A04: Insecure Design | Architecture enforces security structurally (absent not hidden) |
| A05: Security Misconfiguration | Fail-fast config validation; no default credentials |
| A06: Vulnerable Components | Pinned dependencies; amis SDK version-locked |
| A07: Auth Failures | Short-lived sessions; Redis revocation; ConstantTimeCompare |
| A08: Integrity Failures | Tamper-evident audit log; workflow idempotency |
| A09: Logging Failures | Mandatory structured logging; sensitive field exclusion |
| A10: SSRF | No user-controlled URL fetching in application layer |

---

## References

- [`04-multitenancy/RLS_SPEC.md`](../04-multitenancy/RLS_SPEC.md) — RLS implementation
- [`03-auth/AUTHORIZATION_SPEC.md`](../03-auth/AUTHORIZATION_SPEC.md) — RBAC and PolicyEvaluator
- [`03-auth/SESSION_MODEL.md`](../03-auth/SESSION_MODEL.md) — Session security
- [`18-security/SENSITIVE_FIELDS.md`](SENSITIVE_FIELDS.md) — Sensitive field semantics
- [`18-security/SECRET_MANAGEMENT.md`](SECRET_MANAGEMENT.md) — Secret handling
- [`12-audit/AUDIT_SPEC.md`](../12-audit/AUDIT_SPEC.md) — Audit immutability
