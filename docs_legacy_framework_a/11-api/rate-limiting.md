> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Rate Limiting"
id: api-005
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, operators]
since: "1.0"
normative-level: normative
related:
  - "[API Conventions](conventions.md)"
  - "[Configuration](../12-configuration/configuration.md)"
  - "[Sessions](../07-iam/sessions.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Rate Limiting

**API-005 | Status: Accepted | Stability: Stable**

This document specifies the rate limiting system: sliding window algorithm, Redis key structure, per-tenant and per-user limits, and bypass rules.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Rate Limit Scopes

Rate limits are applied at two scopes simultaneously:

| Scope | Key | Default limit |
|---|---|---|
| Per-tenant | `rl:tenant:{tenant_id}:{window}` | 10,000 req/min |
| Per-user | `rl:user:{tenant_id}:{user_id}:{window}` | 300 req/min |
| Per-API-client | `rl:client:{tenant_id}:{client_id}:{window}` | 600 req/min |

Both scopes are checked on every request. The most restrictive limit that is exceeded triggers the rate limit response.

---

## 2. Sliding Window Algorithm

```go
// middleware/rate_limit.go

func CheckRateLimit(ctx context.Context, redis *redis.Client, key string, limit int) (bool, int, error) {
    now := time.Now().UnixMilli()
    windowStart := now - 60000  // 60-second sliding window

    pipe := redis.Pipeline()
    // Remove expired entries
    pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart, 10))
    // Count current entries
    countCmd := pipe.ZCard(ctx, key)
    // Add current request
    pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: now})
    // Reset TTL
    pipe.Expire(ctx, key, 2*time.Minute)

    if _, err := pipe.Exec(ctx); err != nil {
        return true, 0, err  // fail open on Redis error
    }

    count := int(countCmd.Val())
    remaining := limit - count
    if remaining < 0 {
        remaining = 0
    }
    return count < limit, remaining, nil
}
```

The sliding window uses a Redis sorted set. Score = timestamp in milliseconds; member = timestamp (unique per request). This provides accurate per-second granularity without the step-function problem of fixed windows.

---

## 3. HTTP Response Headers

Every response includes rate limit headers:

```
X-RateLimit-Limit:     300
X-RateLimit-Remaining: 247
X-RateLimit-Reset:     1710498660
```

- `X-RateLimit-Limit`: requests allowed per window
- `X-RateLimit-Remaining`: requests remaining in current window
- `X-RateLimit-Reset`: Unix timestamp when the window resets

When the rate limit is exceeded:

```
HTTP 429 Too Many Requests
Retry-After: 23
X-RateLimit-Limit:     300
X-RateLimit-Remaining: 0
X-RateLimit-Reset:     1710498660

{
  "error": {
    "code":    "rate_limit_exceeded",
    "message": "Too many requests. Please retry after 23 seconds."
  }
}
```

`Retry-After` is the number of seconds until the oldest entry in the window expires.

---

## 4. Configuration

```
# Tenant-level limits (per Settings module)
Setting: platform.rate_limit_per_tenant_per_minute = 10000

# User-level limits
Setting: platform.rate_limit_per_user_per_minute = 300

# API client limits
Setting: platform.rate_limit_per_api_client_per_minute = 600
```

Platform admins can override per-tenant:

```
Setting (tenant override): platform.rate_limit_per_user_per_minute = 1000
```

---

## 5. Bypass Rules

Certain paths are exempt from rate limiting:

| Path | Reason |
|---|---|
| `GET /health/live` | Liveness probe — must never be rate limited |
| `GET /health/ready` | Readiness probe |
| `GET /metrics` | Prometheus scrape (protected by network policy) |

Authentication endpoints have separate, stricter limits:

| Endpoint | Limit |
|---|---|
| `POST /api/v1/auth/login` | 10 req/min per IP |
| `POST /api/v1/auth/forgot-password` | 5 req/min per IP |
| `POST /api/v1/auth/mfa/verify` | 5 attempts per challenge token |

Authentication endpoint limits are per-IP (not per-session — users are not authenticated yet).

---

## 6. Redis Failure Behavior

If Redis is unavailable, rate limiting fails open — requests are allowed through. This is intentional:

- Rate limiting failure should not cause a service outage
- The trade-off: brief Redis downtime = brief rate limit bypass window

Session validation, however, fails closed on Redis unavailability (HTTP 503) — per INV-007.

---

## 7. Rate Limit Headers in Bulk Endpoints

Bulk endpoints consume more capacity. The rate limit middleware counts each HTTP request as one unit, regardless of how many items are in the payload. Callers of bulk endpoints should be aware that a single call consuming 500 item operations still counts as 1 request against the rate limit.

---

## Related Documents

- [API Conventions](conventions.md) — middleware pipeline order (rate limiting runs 7th)
- [Configuration](../12-configuration/configuration.md) — rate limit environment variables
- [Security Model](../15-security/security-model.md) — rate limiting as T4 (brute force) mitigation
- [Observability](../13-observability/metrics-reference.md) — `rate_limit_exceeded_total` metric
