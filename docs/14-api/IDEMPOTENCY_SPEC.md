# Idempotency Specification

**Classification:** Specification — Tier 1
**Owner:** `14-api/IDEMPOTENCY_SPEC.md`
**Status:** Frozen at v1.0 (ADR-009)

---

## Purpose

This document specifies the `X-Idempotency-Key` header protocol (ADR-009) — the mechanism by which POST and PATCH requests can be safely retried without causing duplicate mutations.

---

## 1. Problem Statement

Network failures can cause clients to retry requests without knowing whether the original reached the server. Without idempotency, retries create duplicate records (double invoice creation, double payment recording). ADR-009 adds server-side deduplication via a client-supplied key.

---

## 2. Header

```
X-Idempotency-Key: {client-generated unique string}
```

- Supplied by the client on POST and PATCH requests.
- The value is an arbitrary string up to 255 characters.
- Recommended format: UUID v4 (`f47ac10b-58cc-4372-a567-0e02b2c3d479`)
- Required for: all mutating API calls from payment processors, webhook handlers, mobile apps.
- Optional for: browser SDUI sessions (idempotency is handled by the SDUI submit logic).

---

## 3. Redis Storage

Idempotency records are stored in Redis:

```
Key:   idempotency_cache:{tenant_id}:{idempotency_key}
TTL:   24 hours
Value: JSON-serialised IdempotencyRecord
```

```go
type IdempotencyRecord struct {
    StatusCode  int             `json:"status_code"`
    Body        json.RawMessage `json:"body"`
    CompletedAt time.Time       `json:"completed_at"`
}
```

---

## 4. Protocol

### First Request (No Cache Hit)

1. Middleware extracts `X-Idempotency-Key` header.
2. Checks Redis key `idempotency_cache:{tenant_id}:{key}`.
3. Cache miss → proceed with normal request handling.
4. On successful response: write `IdempotencyRecord` to Redis with 24h TTL.
5. Return the response to the client.

### Subsequent Requests (Cache Hit)

1. Middleware finds existing `IdempotencyRecord` in Redis.
2. Returns the cached `status_code` and `body` immediately.
3. The handler is **not invoked** — no side effects.
4. Response includes header: `X-Idempotency-Replay: true`.

### In-Flight Requests

If a request with the same idempotency key is currently being processed (race condition):

1. The second request receives HTTP 409 with:
   ```json
   {
     "error": {
       "code":    "idempotency.in_flight",
       "message": "A request with this idempotency key is already being processed."
     }
   }
   ```
2. The client SHOULD retry after a short delay.

Implementation: use Redis `SET NX` (set-if-not-exists) with a 30-second "in-flight" sentinel before processing. Replace with the full `IdempotencyRecord` on completion.

---

## 5. Scope

Idempotency keys are scoped to `{tenant_id}:{key}`. Two different tenants can use the same key value without conflict.

Idempotency keys are NOT scoped to the user. The same key reused by two different users in the same tenant returns the first user's response to the second. API clients MUST use globally unique keys (UUID v4).

---

## 6. Failure Behaviour

If the first request fails (4xx or 5xx response), the idempotency record is NOT written to Redis. The same key can be retried on failure.

If Redis is unavailable:
- Idempotency enforcement MUST fail closed: return HTTP 503.
- Do not allow the request to proceed without idempotency check — silent deduplication failure is worse than temporary unavailability for payment operations.

---

## 7. Which Methods Are Covered

| Method | Idempotency Support |
|--------|-------------------|
| `POST` | Yes — covered by this spec |
| `PATCH` | Yes — covered by this spec |
| `DELETE` | Inherently idempotent (HTTP semantics); header accepted but not required |
| `GET` | Read-only; idempotency not applicable |

---

## 8. TTL and Expiry

The 24-hour TTL means:
- Retries within 24 hours of the first successful request return the cached response.
- After 24 hours, retrying with the same key is treated as a new request (processed normally).
- Clients MUST NOT reuse idempotency keys after 24 hours if they want deduplication.

---

## 9. Normative Requirements

- `X-Idempotency-Key` MUST be stored per `{tenant_id}:{key}` — not globally.
- Redis failure MUST result in HTTP 503 — not silent pass-through.
- On cache hit, the handler MUST NOT be invoked.
- The response on cache hit MUST include `X-Idempotency-Replay: true`.
- Failure responses (4xx/5xx) MUST NOT be cached as idempotency records.

---

## References

- [`11-cache/CACHE_KEY_REFERENCE.md`](../11-cache/CACHE_KEY_REFERENCE.md) — `idempotency_cache:` key pattern
- [`14-api/API_CONVENTIONS.md`](API_CONVENTIONS.md) — Request headers
- ADR-009 in [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md)
