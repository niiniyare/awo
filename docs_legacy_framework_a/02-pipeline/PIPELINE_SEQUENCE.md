> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Pipeline Sequence Diagram

**Classification:** Reference — Tier 1
**Owner:** `02-pipeline/PIPELINE_SEQUENCE.md`
**Status:** Frozen at v1.0

---

## Purpose

This document provides sequence diagrams for the complete request lifecycle — from HTTP request receipt to response — for CRUD operations and custom actions.

---

## 1. Create Record (POST /api/v1/entities/{type})

```
Client          Middleware          Runtime          EntityRepo        PostgreSQL        Temporal
  │                  │                  │                 │                 │                │
  │── POST ─────────►│                  │                 │                 │                │
  │                  │                  │                 │                 │                │
  │   ① Request ID   │                  │                 │                 │                │
  │   ② Logging      │                  │                 │                 │                │
  │   ③ Tenant res.  │                  │                 │                 │                │
  │   ④ set_tenant() │──────────────────────────────────────►SET CONFIG──►│                │
  │   ⑤ Session val. │──── Redis GET session:{token} ──────────────────────│────────────────│
  │   ⑥ Rate limit   │──── Redis INCR rl:{tenant}:{user}:{win} ───────────│────────────────│
  │   ⑦ Idempotency  │──── Redis GET idempotency_cache:{key} (miss) ──────│────────────────│
  │                  │                  │                 │                 │                │
  │                  │── invoke ────────►│                 │                 │                │
  │                  │                  │                 │                 │                │
  │                  │              ASSEMBLE               │                 │                │
  │                  │              EntityRecord            │                 │                │
  │                  │                  │                 │                 │                │
  │                  │              before_validate hooks  │                 │                │
  │                  │                  │                 │                 │                │
  │                  │              VALIDATE               │                 │                │
  │                  │              (field types, required)│                 │                │
  │                  │                  │                 │                 │                │
  │                  │              AUTHORIZE              │                 │                │
  │                  │              PolicyEvaluator.CanPerform()            │                │
  │                  │                  │                 │                 │                │
  │                  │              before_save hooks      │                 │                │
  │                  │                  │                 │                 │                │
  │                  │                  │── BEGIN TX ─────────────────────►│                │
  │                  │                  │                 │                 │                │
  │                  │              PERSIST               │                 │                │
  │                  │                  │── INSERT ───────────────────────►│                │
  │                  │                  │                 │                 │                │
  │                  │              AUDIT RECORD           │                 │                │
  │                  │                  │── INSERT audit_log ─────────────►│                │
  │                  │                  │                 │                 │                │
  │                  │              after_save hooks       │                 │                │
  │                  │              (inside TX)            │                 │                │
  │                  │                  │                 │                 │                │
  │                  │                  │── COMMIT TX ────────────────────►│                │
  │                  │                  │                 │                 │                │
  │                  │              WorkflowTrigger check  │                 │                │
  │                  │              (if OnCreate trigger)  │                 │                │
  │                  │                  │── INSERT workflow_outbox ────────►│                │
  │                  │                  │                 (outbox worker)   │                │
  │                  │                  │                                   │── StartWorkflow►│
  │                  │                  │                 │                 │                │
  │◄── 201 Created ──│◄─ response ──────│                 │                 │                │
  │                  │                  │                 │                 │                │
  │                  │── Redis SET idempotency_cache:{key} (cache response)                  │
```

---

## 2. Custom Action (POST /api/v1/entities/{type}/{id}/{action})

```
Client          Middleware          ActionRuntime        ActionEntityRepo  PostgreSQL
  │                  │                  │                      │               │
  │── POST action ──►│                  │                      │               │
  │                  │                  │                      │               │
  │   ① - ⑦ same middleware pipeline as above                  │               │
  │                  │                  │                      │               │
  │                  │── invoke ────────►│                      │               │
  │                  │              ActionContext.Runtime       │               │
  │                  │              ActionContext.RecordID      │               │
  │                  │              ActionContext.Body          │               │
  │                  │                  │                      │               │
  │                  │              Permission check            │               │
  │                  │              PolicyEvaluator.CanPerform(action.Name)    │
  │                  │                  │                      │               │
  │                  │              ActionHandlerFunc           │               │
  │                  │                  │── Repo("entity").Get─►── SELECT ────►│
  │                  │                  │                      │               │
  │                  │                  │── Tx(fn) ────────────►── BEGIN TX──►│
  │                  │                  │   │── Repo.Update ────►── UPDATE ──►│
  │                  │                  │   │── Publish(event) ─►── INSERT outbox ►│
  │                  │                  │── COMMIT TX ─────────────────────────────►│
  │                  │                  │                      │               │
  │                  │                  │── StartWorkflow ──────►── INSERT workflow_outbox ►│
  │                  │                  │                      │               │
  │◄── 200/202 ──────│◄─ ActionResult ──│                      │               │
```

---

## 3. List Query (GET /api/v1/entities/{type}?limit=20&offset=0)

```
Client          Middleware          Runtime          EntityRepo        PostgreSQL
  │                  │                  │                 │                 │
  │── GET list ─────►│                  │                 │                 │
  │                  │                  │                 │                 │
  │   ① Tenant       │──────────────────────────────────────►SET CONFIG──►│
  │   ② Session      │──── Redis GET session:{token}                       │
  │   ③ Rate limit   │──── Redis INCR                                       │
  │                  │                  │                 │                 │
  │   SDUI schema?   │                  │                 │                 │
  │   GET /ui/type   │──── Redis GET page:{entity}:{view}:{roles}:{tenant} │
  │   cache hit ──────────────────────────────────────────────────────────►│
  │   (skip DB call) │                  │                 │                 │
  │                  │                  │                 │                 │
  │   Data query     │── invoke ────────►│                 │                 │
  │                  │              PolicyFunc applies privacy filter        │
  │                  │                  │── Query(filter) ►── SELECT (RLS) ►│
  │                  │                  │                 │◄── rows ─────────│
  │                  │                  │── Count(filter) ►── COUNT(*)────►│
  │                  │                  │                 │◄── total ────────│
  │                  │                  │                 │                 │
  │◄── 200 [{...}] ──│◄─ data+meta ─────│                 │                 │
```

---

## 4. Error Paths

### Validation Error (422)

```
ASSEMBLE → before_validate → ValidationError returned
    │
    └── 422 {"error": {"code": "validation_error", "fields": {...}}}
```

### Authorization Error (403)

```
ASSEMBLE → before_validate → VALIDATE → AUTHORIZE → PermissionError
    │
    └── 403 {"error": {"code": "permission_denied", "message": "..."}}
```

### Business Rule Error (409)

```
before_save hook → BusinessError{Code: "...", Status: 409}
    │
    └── 409 {"error": {"code": "invoice.not_draft", "message": "..."}}
```

### Transaction Rollback

```
after_save hook → error
    │
    └── TX ROLLBACK (entity record + audit record both rolled back)
    └── 500 {"error": {"code": "internal_error", "message": "An unexpected error occurred."}}
```

---

## 5. Middleware Pipeline Order

The middleware pipeline executes in this fixed order for every request. The order is non-configurable.

```
1. Request ID        Extract X-Request-ID or generate UUID
2. Structured log    Attach request context to logger
3. Panic recovery    Catch panics; return 500; never expose stack trace
4. CORS              Per-tenant subdomain origin validation
5. Tenant resolve    X-Tenant-ID header > subdomain > query param
6. set_tenant()      Call set_tenant_context($1) stored proc; validate tenant ACTIVE
7. Session validate  Redis GET session:{token}; check ExpiresAt; attach ViewerContext
8. Rate limiting     Redis INCR rl:{tenant}:{user}:{window}; 429 if exceeded
9. Idempotency       Redis GET idempotency_cache:{tenant}:{key}; 200 on hit
```

Steps 6-9 are security-critical. Order MUST NOT be changed.

---

## References

- [`02-pipeline/LIFECYCLE_SPEC.md`](LIFECYCLE_SPEC.md) — Stage-by-stage specification
- [`02-pipeline/HOOK_CONTRACT.md`](HOOK_CONTRACT.md) — Hook execution order and contracts
- [`02-pipeline/ERROR_MODEL.md`](ERROR_MODEL.md) — Error types and HTTP mapping
