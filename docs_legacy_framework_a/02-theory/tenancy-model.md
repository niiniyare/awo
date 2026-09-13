> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Tenancy Model

## What a Tenant Is

A **tenant** is the top-level infrastructure isolation boundary. Every record in every table belongs to exactly one tenant, enforced at the PostgreSQL kernel level via Row-Level Security (RLS).

A tenant represents a customer account — one company (or group of companies) that subscribes to the ERP. A tenant is not a business unit, not a branch, not a department. Those concepts belong to the organization model (see [organization-model.md](./organization-model.md)).

---

## Tenant vs Organization

| | Tenant | Organization |
|---|---|---|
| **What it is** | Infrastructure account | Business hierarchy node |
| **Who creates it** | Awo platform provisioning | Tenant admin |
| **Multiplicity** | One per customer account | Many per tenant |
| **Isolation** | PostgreSQL RLS | Application-layer scope |
| **Lifecycle** | PENDING → ACTIVE → SUSPENDED → ARCHIVED | active / inactive |
| **Immutability** | slug is immutable | code is immutable |
| **RLS policy** | Yes (on all business tables) | No (visibility in Go) |

A tenant has exactly one `platform_tenant` record. A tenant has zero or more `platform_organization` nodes. The organization tree lives entirely within the tenant's RLS boundary.

---

## Tenant Lifecycle

```
PENDING → ACTIVE → SUSPENDED → ACTIVE    (payment resolved)
PENDING → ARCHIVED                        (abandoned signup)
ACTIVE  → ARCHIVED                        (account deletion)
SUSPENDED → ARCHIVED                      (grace period expired)
```

ARCHIVED is terminal. No transitions out of ARCHIVED are permitted.

| Status | HTTP Response to Requests |
|---|---|
| PENDING | 503 + `Retry-After: 60` |
| ACTIVE | Normal |
| SUSPENDED | 402 Payment Required |
| ARCHIVED | 410 Gone |

---

## Tenant Identification

Resolution order on each request:

1. `X-Tenant-ID` header (UUID) — preferred for API clients and service-to-service calls
2. `tenant_id` query param — webhooks and legacy integrations only; disable in production
3. Subdomain parsing — browser access (`bo.`, `portal.`, `app.`, `api.` prefixes)

---

## RLS Enforcement

Every tenant-scoped table has:

```sql
ALTER TABLE my_entity ENABLE ROW LEVEL SECURITY;
ALTER TABLE my_entity FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON my_entity
    USING (tenant_id = current_tenant_id());
```

`current_tenant_id()` reads the `app.current_tenant_id` PostgreSQL transaction-local variable, set by:

```go
store.SetTenantContextFromCtx(ctx)  // calls set_tenant_context($1)
```

`set_tenant_context()` validates tenant exists and status = `'ACTIVE'`, then:

```sql
SELECT set_config('app.current_tenant_id', $1, TRUE);
-- TRUE = transaction-local; resets on COMMIT/ROLLBACK automatically
```

**PgBouncer requirement:** Must run in **transaction mode**. Session mode breaks the transaction-local reset — a connection reused by a different tenant would carry the previous tenant's ID.

---

## Global Tables (No RLS)

These tables are readable by the `platform_reader` role without tenant context:

- `platform_tenant` — tenant registry (RLS would prevent bootstrapping)
- `timezones`, `currencies`, `countries`, `paye_bands` — reference data
- `platform_admins` — platform-level operator accounts

Business modules must never query these tables directly — use the service interfaces.

---

## Organization Tables (Tenant RLS Only)

`platform_organization`, `platform_org_type`, and `platform_org_assignment` have standard tenant RLS — identical to all other business entity tables:

```sql
CREATE POLICY tenant_isolation ON platform_organization
    USING (tenant_id = current_tenant_id());
```

They carry **no** org-visibility predicates. Organization visibility is an application-layer concern evaluated by `OrganizationService.ResolveScope()`. The distinction:

- **Tenant RLS** (Stage 1): prevents cross-tenant leaks — same policy as `invoice`, `contact`, every other table.
- **Org scope** (Stage 2): restricts which org nodes within a tenant are visible to a user — resolved in Go, applied as an explicit `IN` predicate.

RLS is never responsible for org visibility. Org scope is never implemented through RLS predicates.

See [authorization.md](./authorization.md) for the full two-stage pipeline.

---

## Tenant Fields

| Field | Notes |
|---|---|
| `id` | UUID PK |
| `name` | Display name |
| `slug` | Immutable — embedded in URLs, workflow IDs, Redis keys |
| `status` | Lifecycle state |
| `plan` | Subscription plan: free, starter, growth, enterprise |
| `country` | ISO 3166-1 alpha-2 |
| `locale` | IETF language tag (default: `en-KE`) |
| `timezone` | IANA tz (default: `Africa/Nairobi`) |
| `currency` | ISO 4217 (default: `KES`) |
| `contact_email` | Billing and notifications |
| `contact_phone` | Support contact |
| `trial_ends_at` | Trial expiry (sensitive) |
| `suspended_at` | Suspension timestamp (sensitive) |
| `suspension_reason` | Human-readable reason (sensitive) |

`company_size` was removed in v1.1 — it is a business attribute, not a tenant infrastructure attribute. Tenant admins who need to record company size should use a `platform_organization` node of type `company` with a custom field.

---

## Related

- [organization-model.md](./organization-model.md) — org tree, scope resolution, assignments
- [authorization.md](./authorization.md) — two-stage authorization pipeline
