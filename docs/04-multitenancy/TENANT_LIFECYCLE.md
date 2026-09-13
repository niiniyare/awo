# Tenant Lifecycle

**Classification:** Specification — Tier 1
**Owner:** `04-multitenancy/TENANT_LIFECYCLE.md`
**Status:** Frozen at v1.0

---

## Purpose

This document specifies the tenant status state machine, the allowed transitions, and the HTTP behavior associated with each status.

---

## 1. Status Values

| Status | Description |
|--------|-------------|
| `PENDING` | Tenant created; awaiting provisioning or payment confirmation |
| `ACTIVE` | Fully operational; all API requests proceed normally |
| `SUSPENDED` | Temporarily disabled; payment issue or policy violation |
| `ARCHIVED` | Permanently closed; terminal state |

---

## 2. State Machine

```
         PENDING ──────────────────────────────→ ARCHIVED
            │                                        ↑
            ↓                                        │
         ACTIVE ──── suspension ──→ SUSPENDED ───────┤
            │                           │             │
            │                           ↓             │
            │                        ACTIVE  (payment resolved)
            │
            └──────────────────────────────────────→ ARCHIVED
```

### Allowed Transitions

| From | To | Trigger |
|------|----|---------|
| PENDING | ACTIVE | Provisioning complete, payment confirmed |
| PENDING | ARCHIVED | Tenant abandoned during onboarding |
| ACTIVE | SUSPENDED | Payment failure, policy violation |
| ACTIVE | ARCHIVED | Account deletion request |
| SUSPENDED | ACTIVE | Payment resolved, violation corrected |
| SUSPENDED | ARCHIVED | Grace period expired |

**ARCHIVED is a terminal state.** No transition out of ARCHIVED is permitted.

**PENDING → SUSPENDED is prohibited.** A tenant that has never been ACTIVE cannot be suspended.

---

## 3. HTTP Response by Status

| Tenant Status | HTTP Status | Headers | Body |
|---------------|-------------|---------|------|
| `ACTIVE` | (request proceeds normally) | — | Normal response |
| `PENDING` | `503 Service Unavailable` | `Retry-After: 60` | `{"error": {"code": "tenant_pending", "message": "Account setup in progress"}}` |
| `SUSPENDED` | `402 Payment Required` | — | `{"error": {"code": "tenant_suspended", "message": "Account suspended"}}` |
| `ARCHIVED` | `410 Gone` | — | `{"error": {"code": "tenant_archived", "message": "Account closed"}}` |

The `set_tenant_context()` procedure returns a PostgreSQL exception for non-ACTIVE tenants. The middleware translates this exception to the appropriate HTTP response.

---

## 4. The tenants Table

```sql
CREATE TABLE tenants (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name         text NOT NULL,
    slug         text NOT NULL UNIQUE,
    status       text NOT NULL DEFAULT 'PENDING'
                     CHECK (status IN ('PENDING', 'ACTIVE', 'SUSPENDED', 'ARCHIVED')),
    company_size text CHECK (company_size IN ('SMALL', 'MEDIUM', 'LARGE', 'ENTERPRISE')),
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
```

The `tenants` table is a **global table** — no RLS, accessible by the application role for reads. Status is validated by `set_tenant_context()` on every request.

---

## 5. Normative Requirements

- `set_tenant_context()` MUST reject non-ACTIVE tenants before setting the GUC.
- The ARCHIVED status MUST be terminal — no code path may transition out of ARCHIVED.
- PENDING → SUSPENDED transition MUST be rejected.
- HTTP responses for non-ACTIVE tenants MUST match the table in §3.

---

## References

- [`04-multitenancy/RLS_SPEC.md`](RLS_SPEC.md) — set_tenant_context() implementation
- [`04-multitenancy/TENANT_IDENTIFICATION.md`](TENANT_IDENTIFICATION.md) — How the tenant is resolved per request
