> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Cache Key Reference

**Classification:** Reference — Tier 2
**Owner:** `11-cache/CACHE_KEY_REFERENCE.md`
**Status:** Living document — updated when new cached subsystems are added

---

## Purpose

This document is the exhaustive inventory of all Redis key patterns used by the Awo Framework. Every cache key used anywhere in the framework MUST be documented here.

---

## Key Pattern Index

### Session Keys

| Pattern | Example | TTL | Owner |
|---------|---------|-----|-------|
| `session:{token}` | `session:abc123def456...` | Session expiry (default 8h) | `awo/auth` |

Written on: successful login. Deleted on: logout, session revocation, expiry.

Value: JSON-serialised `auth.Session` struct.

---

### SDUI Page Schema Keys

| Pattern | Example | TTL | Owner |
|---------|---------|-----|-------|
| `page:{qualified_name}:{view}:{roles_hash}:{tenant_id}` | `page:finance_invoice:list:a3f2b1c0d4e5f6a7:550e8400-...` | 5 min | `awo/sdui` |

- `qualified_name`: entity qualified name, e.g. `finance_invoice`
- `view`: `list` / `create` / `edit` / `detail`
- `roles_hash`: SHA-256 of sorted role list, first 16 hex chars
- `tenant_id`: UUID string (no dashes? — include dashes for readability)

Written on: first request for that entity+view+viewer+tenant combination.
Invalidated on: `ActionRuntime.InvalidateCache()`, permission change, feature flag change.

Value: JSON-serialised amis schema (`map[string]any`).

---

### Feature Flag Evaluation Keys

| Pattern | Example | TTL | Owner |
|---------|---------|-----|-------|
| `eval:{sha256(flag_name+tenant_id+user_id)}` | `eval:d7a2f3b1...` | 5 min | `awo/flags` |

- Hash input: concatenation of `flag_name + ":" + tenant_id + ":" + user_id`
- Hash function: SHA-256, full hex string

Written on: first evaluation of the flag for that context.
Invalidated on: flag value change (immediate invalidation, not TTL-based).

Value: JSON boolean or string depending on flag type.

---

### Rate Limiting Keys

| Pattern | Example | TTL | Owner |
|---------|---------|-----|-------|
| `rl:{tenant_id}:{user_id}:{window_start_unix}` | `rl:550e8400-...:660e9400-...:1721472000` | Window duration | `awo/middleware` |

- `window_start_unix`: Unix timestamp (seconds) of the current window start, floored to window size.
- Window size is configurable (default: 60 seconds).

Written on: first request in a rate limit window. Incremented via `Counter.Increment`.
Expires: automatically at window end.

Value: integer count (Redis `INCR` counter).

---

### Naming Series Counter Keys

| Pattern | Example | TTL | Owner |
|---------|---------|-----|-------|
| `ns:{pattern_hash}:{tenant_id}:{period}` | `ns:a1b2c3d4:550e8400-...:2026-07` | Until period reset | `awo/naming` |

- `pattern_hash`: SHA-256 of the NamingSeries pattern string, first 8 hex chars
- `period`: ISO period string derived from reset frequency:
  - `reset: "yearly"` → `{YYYY}` e.g. `2026`
  - `reset: "monthly"` → `{YYYY-MM}` e.g. `2026-07`
  - `reset: "never"` → `global` (constant string)

Written on: first `NamingSeries.Allocate()` call for the period.
Reset: counter deleted and recreated on period change.

Value: integer sequence number (Redis `INCR` counter). TTL = duration to period end + 1 day buffer.

---

### Idempotency Cache Keys

| Pattern | Example | TTL | Owner |
|---------|---------|-----|-------|
| `idempotency_cache:{tenant_id}:{idempotency_key}` | `idempotency_cache:550e8400-...:user-provided-key-123` | 24 h | `awo/middleware` |

Written on: first request with that `X-Idempotency-Key` header for the tenant.
Invalidated: only by TTL expiry.

Value: JSON-serialised `IdempotencyRecord` (status code, response body, completed_at).

---

### Tenant Status Cache Keys

| Pattern | Example | TTL | Owner |
|---------|---------|-----|-------|
| `tenant:{tenant_id}:status` | `tenant:550e8400-...:status` | 30 s | `awo/middleware` |

Written on: tenant resolution in middleware.
Invalidated on: tenant status change.

Value: string — one of `PENDING`, `ACTIVE`, `SUSPENDED`, `ARCHIVED`.

Short TTL (30s) is intentional: tenant status changes must propagate quickly for billing/suspension workflows.

---

## Key Construction Rules

1. **All key components MUST be lowercase.** No uppercase letters in key patterns.
2. **UUID components MUST include dashes** (standard UUID format, not compact hex).
3. **Hash components use the hex string** produced by `crypto/sha256`, NOT base64.
4. **Period components use ISO format** (YYYY, YYYY-MM) for date-based keys.
5. **All keys MUST have TTL** — infinite-TTL keys are prohibited except `reset: "never"` naming counters.
6. **Tenant isolation** — all non-session keys include `tenant_id` in the key. Cross-tenant access is impossible by key structure.

---

## Prohibited Key Patterns

| Pattern | Why Prohibited |
|---------|---------------|
| `user:{user_id}:*` (without tenant) | Cross-tenant access risk |
| Keys without `tenant_id` for tenant-scoped data | RLS bypass in cache layer |
| Keys with raw entity data (not IDs) | Sensitive data in Redis |
| Keys with user-supplied strings without sanitisation | Cache poisoning / key collision |

---

## References

- [`11-cache/CACHE_SPEC.md`](CACHE_SPEC.md) — Cache and Counter interfaces
- [`14-api/IDEMPOTENCY_SPEC.md`](../14-api/IDEMPOTENCY_SPEC.md) — Idempotency key protocol
- [`07-naming/NAMING_SERIES_SPEC.md`](../07-naming/NAMING_SERIES_SPEC.md) — Counter key derivation
