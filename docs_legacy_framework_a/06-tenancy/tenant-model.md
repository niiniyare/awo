> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Tenant Model"
id: ten-001
status: accepted
category: SPEC
stability: FROZEN
audience: [framework-authors, module-authors, operators]
since: "1.0"
normative-level: normative
related:
  - "[Row-Level Security](rls.md)"
  - "[Tenant Lifecycle](tenant-lifecycle.md)"
  - "[Startup Sequence](../03-kernel/startup-sequence.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Tenant Model

**TEN-001 | Status: Accepted | Stability: Frozen**

This document specifies how tenants are identified in inbound requests, how tenant context is propagated through the system, and the infrastructure requirements (PgBouncer mode) for correct tenant isolation.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Tenant Identification

Every inbound HTTP request MUST be associated with exactly one tenant before any application logic executes. The Tenant Resolution middleware (step 5 of the Middleware Pipeline) performs this identification using the following priority order:

### Priority 1: X-Tenant-ID Header (UUID)

```
X-Tenant-ID: a1b2c3d4-e5f6-7890-abcd-ef1234567890
```

Preferred method for API clients. The value MUST be a valid UUID. Invalid UUIDs produce HTTP 400.

### Priority 2: tenant_id Query Parameter

```
GET /api/v1/entities/finance_invoice?tenant_id=a1b2c3d4-...
```

For webhooks and legacy integrations. MUST be disabled in production environments by setting `AllowQueryParamTenantID: false` in configuration. Exposing tenant IDs in query parameters causes them to appear in server access logs, which MUST NOT contain tenant-identifying information outside the request body.

### Priority 3: Subdomain Parsing

For browser access. The subdomain identifies the tenant by subdomain slug (not UUID). The framework looks up the tenant UUID from the `tenants` global table using the subdomain.

```
https://acme.app.awo.so      → tenant slug "acme"
https://acme.portal.awo.so   → tenant slug "acme" (portal prefix stripped)
https://acme.api.awo.so      → tenant slug "acme" (api prefix stripped)
```

Handled subdomain prefixes: `bo.`, `portal.`, `app.`, `api.`. Any other subdomain prefix produces HTTP 400.

### Resolution Failure

If none of the three methods produces a valid tenant identifier, the middleware MUST return HTTP 400 with error code `"tenant.not_identified"`. No application code executes for unidentified requests.

If the identified tenant UUID does not exist in the `tenants` table, the middleware MUST return HTTP 404 with error code `"tenant.not_found"`.

---

## 2. TenantContext

After successful identification, the middleware creates a `TenantContext` and attaches it to the request `context.Context`:

```go
type TenantContext struct {
    TenantID   uuid.UUID
    TenantSlug string
    Locale     string  // "en-KE", "sw-KE", etc.
    Timezone   string  // "Africa/Nairobi"
    Currency   string  // "KES"
}
```

The TenantContext is the only way tenant information is propagated through the system. It MUST NOT be propagated through:
- Global variables
- Request-scoped stores outside of `context.Context`
- Function parameters passed alongside `context.Context`

All framework code that needs the current tenant MUST extract it from `context.Context` using:

```go
tc := tenant.FromContext(ctx) // returns TenantContext or panics if absent
```

The panic on absent TenantContext is intentional — it surfaces programming errors immediately rather than silently proceeding with no tenant.

---

## 3. set_tenant_context()

After attaching the TenantContext to the request context, the middleware calls `store.SetTenantContextFromCtx(ctx)`, which:

1. Validates the tenant exists in the `tenants` global table
2. Validates the tenant's status is `ACTIVE` (see [Tenant Lifecycle](tenant-lifecycle.md) for status codes)
3. Executes: `SELECT set_tenant_context($1)` where $1 is the tenant UUID

The `set_tenant_context()` stored procedure:

```sql
CREATE OR REPLACE FUNCTION set_tenant_context(p_tenant_id uuid)
RETURNS void AS $$
DECLARE
    v_status varchar;
BEGIN
    SELECT status INTO v_status FROM tenants WHERE id = p_tenant_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'tenant_not_found' USING ERRCODE = 'P0001';
    END IF;

    IF v_status != 'ACTIVE' THEN
        RAISE EXCEPTION 'tenant_not_active:%', v_status USING ERRCODE = 'P0002';
    END IF;

    PERFORM set_config('app.current_tenant_id', p_tenant_id::text, TRUE);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;
```

The `TRUE` parameter to `set_config` makes the setting transaction-local — it is reset automatically on COMMIT or ROLLBACK. This is critical for correctness: each transaction starts fresh without carry-over from prior requests.

---

## 4. PgBouncer Transaction Mode Requirement

**PgBouncer MUST be configured in transaction mode, not session mode.**

In session mode, PgBouncer allocates a PostgreSQL backend connection to a client for the entire client session. Multiple requests from the same client share the same backend connection. The transaction-local `app.current_tenant_id` is reset on COMMIT — but in session mode, COMMIT does not return the connection to the pool. The next transaction on the same connection starts with `app.current_tenant_id` unset only if the application explicitly resets it.

In transaction mode, PgBouncer allocates a backend connection for the duration of a single transaction. The connection is returned to the pool on COMMIT or ROLLBACK. The transaction-local `app.current_tenant_id` resets with the COMMIT, and the next transaction that picks up this connection has a clean state.

Session mode with Awo is not supported. Attempting to use Awo with session-mode PgBouncer will produce incorrect RLS behavior where some requests access data from the wrong tenant.

---

## 5. Context Propagation to Background Operations

Background operations (Temporal activities, outbox relay, scheduled jobs) that operate on tenant data MUST construct a TenantContext explicitly and attach it to their context before performing any repository operations:

```go
// In a Temporal activity:
func (a *Activities) SendInvoiceEmailActivity(ctx context.Context, input InvoiceEmailInput) error {
    // Construct tenant context from the activity input
    tenantCtx := tenant.TenantContext{TenantID: input.TenantID}
    ctx = tenant.WithContext(ctx, tenantCtx)

    // Now repository operations work correctly
    invoice, err := a.InvoiceRepo.Get(ctx, input.InvoiceID)
    // ...
}
```

Background operations that do not operate on tenant-scoped data use a [SystemViewer](../GLOSSARY.md#systemviewer) context instead of a TenantContext.

---

## 6. Tenant Resolution and Caching

Tenant lookup from subdomain (Priority 3) is cached in Redis to avoid repeated database round-trips:

```
Key: tenant:slug:{subdomain}
Value: tenant UUID
TTL: 5 minutes
```

UUID-based identification (Priority 1 and 2) does not require a lookup — the UUID is the identifier.

Tenant status is not cached. Every `set_tenant_context()` call reads the current status from the `tenants` table. This ensures that SUSPENDED or ARCHIVED status takes effect immediately on the next request.

---

## 7. Tenant Domain Error Catalog

All tenant-domain errors produced by the repository and service layers. Handlers translate these to HTTP responses using `errors.As` unwrapping (never type switch).

### Resolution Errors

| Error Code | HTTP | Trigger | Description |
|---|---|---|---|
| `tenant.not_found` | 404 | Unknown UUID or subdomain | No tenant with that identifier exists |
| `tenant.invalid_id` | 400 | Malformed UUID in header/param | `X-Tenant-ID` value is not a valid UUID |
| `tenant.missing` | 400 | No tenant identifier in request | No header, param, or subdomain found |
| `tenant.subdomain_ambiguous` | 400 | Subdomain matches multiple tenants | Misconfigured tenant records — operator action required |

### Status Errors

| Error Code | HTTP | Trigger | Description |
|---|---|---|---|
| `tenant.pending` | 503 | Tenant status is PENDING | Provisioning not complete. Response includes `Retry-After: 60` header |
| `tenant.suspended` | 402 | Tenant status is SUSPENDED | Payment required. Response body includes suspension reason |
| `tenant.archived` | 410 | Tenant status is ARCHIVED | Terminal state — tenant data is read-only or purged |

### Mutation Errors

| Error Code | HTTP | Trigger | Description |
|---|---|---|---|
| `tenant.duplicate_slug` | 409 | Subdomain already taken | Unique constraint on `tenants.subdomain` |
| `tenant.duplicate_email` | 409 | Admin email already registered | Unique constraint on `tenants.admin_email` |
| `tenant.invalid_transition` | 422 | Illegal status machine move | e.g. ARCHIVED → ACTIVE is not allowed |
| `tenant.immutable_field` | 422 | Attempt to change read-only field | `name`, `subdomain`, `country_code` are set-once |
| `tenant.invalid_company_size` | 422 | `company_size` not in allowed set | Must be one of the defined enum values (stored uppercase) |

### Context Errors

| Error Code | HTTP | Trigger | Description |
|---|---|---|---|
| `tenant.context_missing` | 500 | `set_tenant_context()` not called before repo access | Framework bug — request handler ran without middleware |
| `tenant.rls_violation` | 403 | Row returned with wrong tenant_id | Should never occur in production — indicates RLS misconfiguration |

### Go Sentinel Values

```go
// internal/platform/tenant/errors.go

var (
    ErrTenantNotFound        = &BusinessError{Code: "tenant.not_found",        Status: 404}
    ErrTenantInvalidID       = &BusinessError{Code: "tenant.invalid_id",        Status: 400}
    ErrTenantMissing         = &BusinessError{Code: "tenant.missing",           Status: 400}
    ErrTenantPending         = &BusinessError{Code: "tenant.pending",           Status: 503}
    ErrTenantSuspended       = &BusinessError{Code: "tenant.suspended",         Status: 402}
    ErrTenantArchived        = &BusinessError{Code: "tenant.archived",          Status: 410}
    ErrTenantInvalidTransition = &BusinessError{Code: "tenant.invalid_transition", Status: 422}
)
```

Check errors with `errors.As`, not equality comparison — the error chain may wrap these values:

```go
var be *BusinessError
if errors.As(err, &be) && be.Code == "tenant.suspended" {
    c.Set("X-Suspension-Reason", be.Message)
    return c.Status(402).JSON(errorEnvelope(be))
}
```

---

## Related Documents

- [Row-Level Security](rls.md) — how set_tenant_context() enables RLS
- [Tenant Lifecycle](tenant-lifecycle.md) — tenant status machine and HTTP response codes
- [Architecture Laws](../02-architecture/laws.md) — LAW-005, LAW-015
- [Architecture Invariants](../02-architecture/invariants.md) — INV-001, INV-011
- [Glossary](../GLOSSARY.md) — Tenant, TenantContext, set_tenant_context(), Tenant Identification, PgBouncer
