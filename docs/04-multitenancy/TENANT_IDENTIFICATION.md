# Tenant Identification

**Classification:** Specification — Tier 1
**Owner:** `04-multitenancy/TENANT_IDENTIFICATION.md`
**Status:** Frozen at v1.0

---

## Purpose

This document specifies how the Awo Framework resolves the tenant for each incoming HTTP request.

---

## 1. Resolution Priority

Tenant resolution uses the following sources in priority order. The first source that yields a valid UUID is used:

1. **`X-Tenant-ID` header** (UUID) — preferred for API clients and service accounts
2. **Subdomain parsing** — browser access via tenant-specific subdomains
3. **`tenant_id` query parameter** — webhooks and legacy integrations only; SHOULD be disabled in production

---

## 2. X-Tenant-ID Header

```
X-Tenant-ID: 550e8400-e29b-41d4-a716-446655440000
```

The value MUST be a valid UUID (any version). The middleware validates it is a parseable UUID; `set_tenant_context()` validates it refers to an existing tenant.

This is the **preferred** mechanism for API clients because:
- It is explicit and unambiguous
- It works across all request types (REST, WebSocket, multipart)
- It does not require subdomain DNS configuration per tenant

---

## 3. Subdomain Resolution

For browser-based access, the middleware extracts the tenant from the request hostname:

```
Subdomain patterns resolved to tenant:
  {slug}.app.awo.so      → tenant with slug = {slug}
  {slug}.portal.awo.so   → tenant with slug = {slug}
  {slug}.api.awo.so      → tenant with slug = {slug}
  bo.{slug}.awo.so       → backoffice for tenant with slug = {slug}

Reserved subdomains (not tenant-resolved):
  www.awo.so             → marketing site
  docs.awo.so            → documentation
  status.awo.so          → status page
```

The middleware looks up the slug in the `tenants` table to get the `tenant_id`.

---

## 4. Query Parameter Fallback

```
GET /api/v1/finance/invoices?tenant_id=550e8400-e29b-41d4-a716-446655440000
```

SHOULD be disabled in production via configuration. Permitted for:
- Webhook delivery where header injection is impractical
- Legacy integrations during migration
- Local development tooling

**Security note:** Query parameters appear in server access logs, browser history, and referrer headers. Tenant IDs in query parameters are a minor information disclosure. For production, use the `X-Tenant-ID` header.

---

## 5. Resolution Failure

If no valid tenant can be resolved from any source:
- HTTP 400 Bad Request
- Response: `{"error": {"code": "tenant_required", "message": "Tenant identification is required"}}`

Unauthenticated endpoints (health checks, login) bypass tenant resolution.

---

## 6. Implementation

```go
// Middleware: resolve tenant ID
func resolveTenantID(c *fiber.Ctx) (uuid.UUID, error) {
    // Priority 1: X-Tenant-ID header
    if raw := c.Get("X-Tenant-ID"); raw != "" {
        id, err := uuid.Parse(raw)
        if err != nil {
            return uuid.Nil, &def.BusinessError{
                Code:    "invalid_tenant_id",
                Message: "X-Tenant-ID header must be a valid UUID",
                Status:  400,
            }
        }
        return id, nil
    }

    // Priority 2: subdomain
    if id := resolveFromSubdomain(c.Hostname()); id != uuid.Nil {
        return id, nil
    }

    // Priority 3: query parameter (if enabled)
    if cfg.AllowTenantQueryParam {
        if raw := c.Query("tenant_id"); raw != "" {
            return uuid.Parse(raw)
        }
    }

    return uuid.Nil, &def.BusinessError{
        Code:    "tenant_required",
        Message: "Tenant identification is required",
        Status:  400,
    }
}
```

---

## References

- [`04-multitenancy/RLS_SPEC.md`](RLS_SPEC.md) — What happens after tenant is resolved
- [`04-multitenancy/TENANT_LIFECYCLE.md`](TENANT_LIFECYCLE.md) — Tenant status validation
