# Cache Specification

**Classification:** Specification — Tier 1
**Owner:** `11-cache/CACHE_SPEC.md`
**Status:** Frozen at v1.0
**Package:** `awo.so/awo/cache`

---

## Purpose

This document specifies the `Cache` interface, the `Counter` interface, their Redis implementation, TTL policy, and the rules for cache key construction within the Awo Framework.

---

## 1. Cache Interface

```go
// Package: awo.so/awo/cache

// Cache provides key-value storage backed by Redis.
// All operations are tenant-namespaced when called through ActionRuntime.Cache().
// The top-level cache (used by framework internals) is not tenant-namespaced.
type Cache interface {
    // Get deserialises the value at key into dst using encoding/json.
    // Returns ErrMiss when the key does not exist or has expired.
    Get(ctx context.Context, key string, dst any) error

    // Set stores value at key, serialised via encoding/json, with the given TTL.
    // A zero TTL means the key never expires. Prefer explicit TTLs.
    Set(ctx context.Context, key string, value any, ttl time.Duration) error

    // Delete removes key. No-op when the key does not exist.
    Delete(ctx context.Context, key string) error

    // DeletePrefix removes all keys sharing the given prefix.
    // CAUTION: implemented via SCAN + DEL; avoid on high-cardinality prefixes under load.
    DeletePrefix(ctx context.Context, prefix string) error
}

// ErrMiss is returned by Cache.Get when the key is absent or expired.
var ErrMiss = errors.New("cache: miss")
```

---

## 2. Counter Interface

```go
// Counter provides atomic increment operations for sequences and rate limiting.
// All counters are tenant-namespaced unless documented otherwise.
type Counter interface {
    // Increment atomically increments the counter at key by 1 and returns the new value.
    // If the key does not exist, it is created with value 1.
    // Sets TTL on first creation only (subsequent increments do not reset TTL).
    Increment(ctx context.Context, key string, ttl time.Duration) (int64, error)

    // IncrementBy atomically increments the counter at key by delta and returns the new value.
    IncrementBy(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error)

    // Reset sets the counter to zero and resets the TTL.
    Reset(ctx context.Context, key string, ttl time.Duration) error

    // Value returns the current counter value without incrementing.
    // Returns 0 and nil when the key does not exist.
    Value(ctx context.Context, key string) (int64, error)
}
```

---

## 3. Redis Implementation

The framework ships `cache.RedisCache` and `cache.RedisCounter`, both backed by the same Redis connection pool. Separate struct types prevent accidental mixing of cache and counter operations.

```go
// Constructors
func NewRedisCache(client redis.UniversalClient) Cache
func NewRedisCounter(client redis.UniversalClient) Counter
```

For testing:
```go
func NewNoopCache() Cache      // all Get calls return ErrMiss; Set/Delete are no-ops
func NewMemoryCache() Cache    // in-process map; not safe across processes
func NewNoopCounter() Counter  // Increment always returns sequential values from 1
```

---

## 4. TTL Policy

| Use Case | Key Pattern | TTL |
|---------|-------------|-----|
| Session token | `session:{token}` | Session expiry (configurable; default 8h) |
| SDUI page schema | `page:{entity}:{view}:{roles_hash}:{tenant_id}` | 5 min |
| Feature flag evaluation | `eval:{sha256(flag+tenant+user)}` | 5 min |
| Rate limit window | `rl:{tenant_id}:{user_id}:{window_start}` | Window duration |
| Naming series counter | `ns:{pattern_hash}:{tenant_id}:{period}` | Until period reset |
| Idempotency record | `idempotency_cache:{tenant_id}:{key}` | 24 h |
| Custom (via ActionRuntime) | Caller-defined | Caller-defined |

Rules:
- TTL MUST be set on every key. Infinite TTL is prohibited except for naming series counters that never reset (`reset: "never"`).
- Session TTL is the authority for session expiry — the Redis key expiry and the `Session.ExpiresAt` field MUST be synchronized.

---

## 5. Cache Stampede Prevention

For heavily-contested keys (page schemas, feature flags), the framework uses probabilistic early expiration (PER):

```
effective_ttl = ttl - (random jitter 0..30s)
```

This distributes recomputation across the TTL window and prevents simultaneous expiry of all cached instances of the same key. The jitter is applied per-instance at write time.

For critical keys where only one recomputation should occur, use Redis `SET NX` (set-if-not-exists) with a short lock TTL before writing the full value.

---

## 6. Cache Invalidation

### Explicit Invalidation (MUST use)

Call `ActionRuntime.InvalidateCache(ctx, entityName)` after mutations that change data visible to SDUI schemas or feature flag results.

This call:
1. Deletes all `page:{entityName}:*` keys via `DeletePrefix`.
2. Deletes `eval:*` keys for the tenant via `DeletePrefix`.

### TTL-Based Expiry (fallback)

Without explicit invalidation, cached data expires after 5 minutes. This is the maximum staleness window for SDUI schemas and feature flag evaluations.

---

## 7. Tenant Namespacing

`ActionRuntime.Cache()` returns a tenant-namespaced `ActionCache` wrapper. The wrapper prepends `{tenant_id}:` to every key before delegating to the underlying `Cache`. This prevents cross-tenant cache pollution without requiring callers to include the tenant ID manually.

Framework-internal caches (session, idempotency, page schema) include `tenant_id` in the key explicitly because they operate outside the tenant-namespaced wrapper.

---

## 8. Normative Requirements

- All cache keys MUST include a TTL. Infinite TTL is prohibited except for `reset: "never"` naming series.
- `Cache.Get` returning `ErrMiss` MUST be handled — never treat it as a fatal error.
- Redis failure for page schema / feature flag cache MUST NOT cause 5xx — serve a cache miss (recompute on next request).
- Redis failure for session cache MUST return HTTP 503 — session validation is a hard dependency.
- Redis failure for idempotency cache MUST return HTTP 503 — idempotency cannot be guaranteed without it.
- Cross-tenant cache access is prohibited. All framework cache paths include `tenant_id` in the key.

---

## References

- `awo/cache/cache.go` — Cache interface, ErrMiss, RedisCache
- `awo/cache/counter.go` — Counter interface, RedisCounter
- [`11-cache/CACHE_KEY_REFERENCE.md`](CACHE_KEY_REFERENCE.md) — All key patterns
- [`07-naming/NAMING_SERIES_SPEC.md`](../07-naming/NAMING_SERIES_SPEC.md) — Counter usage for naming series
