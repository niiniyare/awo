# Chapter 34 — Caching Strategy

> **Volume:** VII — Performance, Offline & Sync
> **Phase:** 4
> **Audience:** Backend Engineers, Platform Engineers, Mobile Engineers, Web Engineers
> **Prerequisites:** Chapters 08, 24, 32, 33

---

## Table of Contents

- [34.1 Caching Overview](#341-caching-overview)
- [34.2 Surface Definition Cache](#342-surface-definition-cache)
- [34.3 Data Source Cache](#343-data-source-cache)
- [34.4 Authorization Cache](#344-authorization-cache)
- [34.5 Session Cache](#345-session-cache)
- [34.6 CDN Cache](#346-cdn-cache)
- [34.7 Mobile Persistent Cache](#347-mobile-persistent-cache)
- [34.8 Cache Invalidation Patterns](#348-cache-invalidation-patterns)
- [34.9 Cache Observability](#349-cache-observability)

---

## 34.1 Caching Overview

AwoERP has a multi-layer caching architecture. Each layer addresses a different type of data with different freshness requirements and access patterns.

```
Request path:

Client
  └─ Mobile persistent cache (encrypted disk)         ← surface defs, data
       └─ HTTP conditional request (If-None-Match)
           └─ API Gateway / CDN                        ← static assets
               └─ L1 in-process cache (per pod)        ← surface defs
                   └─ L2 Redis cache                   ← surface defs, auth, sessions
                       └─ PostgreSQL                   ← source of truth
```

### Cache Summary Table

| Cache | What | TTL | Invalidation |
|-------|------|-----|--------------|
| L1 in-process | Surface definitions | 30s | Explicit key delete |
| L2 Redis (surface) | Surface definitions | 60s | Key delete on publish |
| L2 Redis (auth) | Permission checks | 30s | User/role change |
| L2 Redis (session) | User session context | 300s | Logout / role change |
| L2 Redis (rate limit) | Request counts | Window duration | Time-based |
| TanStack Query (web) | Data source responses | 30s (stale), 5min (GC) | Explicit invalidation |
| Mobile disk cache | Surface defs + data | Configurable per source | ETag mismatch |
| CDN | Static assets | 1 year (immutable) | URL hash change |

---

## 34.2 Surface Definition Cache

### 34.2.1 Cache Key

The surface definition cache key encodes all dimensions that affect the compiled output:

```
sdui:{surface_id}:{sha256(tenant_id + role_set_hash + flag_set_hash + locale + platform + client_version + capability_hash)}
```

Where:
- `role_set_hash` = SHA-256 of the sorted set of role IDs assigned to the user
- `flag_set_hash` = SHA-256 of the sorted feature flag key-value pairs for this tenant+user

Two users with different roles produce different cache keys even on the same tenant. A tenant's flag change produces a new `flag_set_hash` for all their users.

### 34.2.2 Cache Warming

When a new surface definition is published (via the platform admin), the cache warmer proactively compiles and caches definitions for the most common role combinations in each tenant:

```go
func (w *CacheWarmer) WarmSurface(ctx context.Context, surfaceID string) error {
    tenants, err := w.store.ListActiveTenants(ctx)
    if err != nil {
        return err
    }

    for _, tenant := range tenants {
        commonRoleSets, err := w.store.GetCommonRoleSets(ctx, tenant.ID)
        if err != nil {
            continue
        }

        for _, roleSet := range commonRoleSets {
            go func(t Tenant, rs RoleSet) {
                ctx := buildCompilationContext(t, rs)
                def, err := w.compiler.Compile(ctx, surfaceID)
                if err != nil {
                    return
                }
                w.cache.Set(ctx, buildCacheKey(surfaceID, t, rs), def)
            }(tenant, roleSet)
        }
    }
    return nil
}
```

### 34.2.3 ETag Generation

The ETag for a surface definition response is the SHA-256 of the serialized definition bytes:

```go
func etagForDefinition(def []byte) string {
    hash := sha256.Sum256(def)
    return fmt.Sprintf(`"%x"`, hash)
}
```

The ETag changes when the definition changes. A client holding the correct ETag gets a 304 on the next fetch — no recompilation, no response body, minimal latency.

---

## 34.3 Data Source Cache

### 34.3.1 Server-Side Data Cache

Frequently accessed, slowly changing data sources (reference data — chart of accounts, vendor lists, currency codes) are cached on the server to avoid redundant database queries across users:

```go
type DataSourceCache struct {
    redis *redis.Client
}

// Cache key: data:{endpoint_hash}:{tenant_id}:{params_hash}
func (c *DataSourceCache) cacheKey(endpoint, tenantID string, params map[string]string) string {
    paramBytes, _ := json.Marshal(sortedParams(params))
    paramHash := fmt.Sprintf("%x", sha256.Sum256(paramBytes))
    endpointHash := fmt.Sprintf("%x", sha256.Sum256([]byte(endpoint)))
    return fmt.Sprintf("data:%s:%s:%s", endpointHash, tenantID, paramHash)
}
```

Cache TTLs by data type:

| Data Type | Server-Side TTL | Client-Side TTL |
|-----------|----------------|----------------|
| Chart of accounts | 5 min | 5 min |
| Vendor / supplier list | 2 min | 2 min |
| Currency codes | 1 hour | 1 hour |
| Invoice list | 0 (no server cache) | 30s |
| Approval workflow status | 0 (no server cache) | 10s |

Invoice lists and workflow statuses are not server-side cached because they are modified by different users and must reflect the latest state.

### 34.3.2 Client-Side Cache (Web)

TanStack Query manages client-side data source caching. The query key is the fully resolved request (endpoint + params):

```typescript
function useDataSource(source: DataSourceDeclaration, resolvedParams: Record<string, string>) {
    return useQuery({
        queryKey: [source.id, source.endpoint, resolvedParams],
        queryFn: () => fetchDataSource(source.endpoint, resolvedParams),
        staleTime: (source.cache_ttl_seconds ?? 30) * 1000,
        gcTime: 5 * 60 * 1000,        // 5 minutes before GC
        refetchOnWindowFocus: false,   // Controlled by SSE instead
        retry: 2,
    });
}
```

### 34.3.3 Cache Invalidation on Mutation

When a mutation action succeeds, the action definition's `invalidate_data_sources` list causes the client to refetch the relevant data sources:

```json
{
  "id": "action_approve_invoice",
  "steps": [
    {
      "type": "action.http_request",
      "method": "POST",
      "endpoint": "/api/invoices/$row.id/approve"
    },
    {
      "type": "action.invalidate_data_source",
      "source_ids": ["invoices_pending", "approval_queue"],
      "invalidate_related": true
    }
  ]
}
```

---

## 34.4 Authorization Cache

Covered in detail in Chapter 27. Summary:

- Redis cache key: `authz:{user_id}:{relation}:{object_type}:{object_id}`
- TTL: 30 seconds
- Invalidated explicitly when user's roles change
- Fails open on Redis errors (checks OpenFGA directly) with a warning telemetry event

---

## 34.5 Session Cache

The user session context (user profile, tenant config, feature flags) is cached in Redis after the first `/api/session` fetch:

```go
const sessionCacheTTL = 5 * time.Minute

func (s *SessionService) Get(ctx context.Context, userID, tenantID string) (*Session, error) {
    key := fmt.Sprintf("session:%s:%s", userID, tenantID)

    // Try cache
    data, err := s.redis.Get(ctx, key).Bytes()
    if err == nil {
        var sess Session
        if err := json.Unmarshal(data, &sess); err == nil {
            return &sess, nil
        }
    }

    // Build from DB
    sess, err := s.buildSession(ctx, userID, tenantID)
    if err != nil {
        return nil, err
    }

    // Cache
    data, _ = json.Marshal(sess)
    s.redis.Set(ctx, key, data, sessionCacheTTL)

    return sess, nil
}

// Invalidated on: logout, role change, flag change, tenant config change
func (s *SessionService) Invalidate(ctx context.Context, userID, tenantID string) {
    key := fmt.Sprintf("session:%s:%s", userID, tenantID)
    s.redis.Del(ctx, key)
}
```

---

## 34.6 CDN Cache

### 34.6.1 Static Assets

All static assets (JS bundles, CSS, fonts, images) use content-hashed URLs and immutable cache headers:

```
GET /assets/forms.Bx3k9pQ.js
Cache-Control: public, max-age=31536000, immutable
ETag: "sha256:abc123"
```

A new deploy generates new content hashes. Old hashes remain valid (immutable cache) until evicted. There is no cache flush on deploy; new URLs replace old ones.

### 34.6.2 API Response Caching at CDN

Surface definition responses that carry `Cache-Control: private` are not cached at the CDN (they contain user-specific compiled content). Reference data endpoints (currency codes, country lists) use `Cache-Control: public, max-age=300` and are CDN-cached, serving many users from the edge without hitting the origin.

---

## 34.7 Mobile Persistent Cache

### 34.7.1 Cache Structure

The mobile persistent cache is organized as:

```
/Documents/awoerp_cache/
├── surfaces/
│   ├── {cache_key_hash}.pb    ← Surface definitions (Protocol Buffer)
│   └── index.json             ← Maps surface_id → cache_key_hash + etag
├── data/
│   ├── {source_id}_{params_hash}.json  ← Data source results
│   └── index.json             ← Maps source_id → files + fetched_at + ttl
└── attachments/
    └── {file_id}              ← Cached file attachments
```

### 34.7.2 Cache Eviction

The mobile cache enforces a maximum size (100MB by default, configurable):

```swift
class CacheEvictionManager {
    let maxCacheBytes: Int64 = 100 * 1024 * 1024  // 100MB

    func evictIfNeeded() {
        let currentSize = calculateCacheSize()
        guard currentSize > maxCacheBytes else { return }

        // LRU eviction: remove least recently accessed entries
        let entries = loadCacheIndex().sorted { $0.lastAccessedAt < $1.lastAccessedAt }
        var freed: Int64 = 0

        for entry in entries {
            if currentSize - freed <= maxCacheBytes { break }
            freed += entry.sizeBytes
            removeCacheEntry(entry)
        }
    }
}
```

The eviction manager runs after every cache write and on app foreground.

### 34.7.3 ETag-Based Conditional GET

Mobile clients include the cached ETag in conditional requests:

```swift
func buildRequest(for url: URL, cachedETag: String?) -> URLRequest {
    var request = URLRequest(url: url)
    if let etag = cachedETag {
        request.setValue(etag, forHTTPHeaderField: "If-None-Match")
    }
    return request
}

func handleResponse(_ response: HTTPURLResponse, data: Data?, cachedData: Data?) -> Data? {
    switch response.statusCode {
    case 200:
        // New content — update cache
        return data
    case 304:
        // Not modified — use cached
        return cachedData
    default:
        return nil
    }
}
```

---

## 34.8 Cache Invalidation Patterns

### 34.8.1 Event-Driven Invalidation

Cache invalidation is triggered by specific platform events published to Redis Pub/Sub:

```go
type CacheInvalidationEvent struct {
    Type     string   `json:"type"`     // "surface_published" | "role_changed" | "flag_changed" | "tenant_config_changed"
    TenantID string   `json:"tenant_id,omitempty"`
    UserID   string   `json:"user_id,omitempty"`
    Keys     []string `json:"keys,omitempty"`
}

// Publisher (e.g., surface admin service)
func (s *SurfaceAdminService) PublishSurface(ctx context.Context, surfaceID string) error {
    if err := s.store.PublishSurface(ctx, surfaceID); err != nil {
        return err
    }

    // Invalidate all cached definitions for this surface across all tenants
    event := CacheInvalidationEvent{
        Type: "surface_published",
        Keys: []string{fmt.Sprintf("sdui:%s:*", surfaceID)},
    }
    return s.pubsub.Publish(ctx, "cache_invalidation", event)
}

// Subscriber (running on every pod)
func (c *CacheInvalidator) Subscribe(ctx context.Context) {
    sub := c.redis.Subscribe(ctx, "cache_invalidation")
    for msg := range sub.Channel() {
        var event CacheInvalidationEvent
        json.Unmarshal([]byte(msg.Payload), &event)
        c.processEvent(ctx, event)
    }
}
```

### 34.8.2 Wildcard Invalidation

When a surface is republished, all cached variants of that surface (across all tenants, all role combinations) must be invalidated. Redis SCAN is used to find and delete matching keys:

```go
func (c *CacheInvalidator) invalidateSurface(ctx context.Context, surfaceID string) {
    pattern := fmt.Sprintf("sdui:%s:*", surfaceID)
    var cursor uint64

    for {
        keys, nextCursor, err := c.redis.Scan(ctx, cursor, pattern, 100).Result()
        if err != nil { break }

        if len(keys) > 0 {
            c.redis.Del(ctx, keys...)
        }

        cursor = nextCursor
        if cursor == 0 { break }
    }
}
```

### 34.8.3 Tenant-Scoped Invalidation

Flag or configuration changes for a specific tenant invalidate only that tenant's cached entries:

```go
func (c *CacheInvalidator) invalidateTenant(ctx context.Context, tenantID string) {
    // Invalidate all surface definitions for this tenant
    pattern := fmt.Sprintf("sdui:*:%s:*", tenantID)
    // ... same SCAN + DEL pattern

    // Invalidate all session caches for this tenant's users
    sessionPattern := fmt.Sprintf("session:*:%s", tenantID)
    // ... same SCAN + DEL pattern
}
```

---

## 34.9 Cache Observability

### 34.9.1 Cache Metrics

The following metrics are tracked in the observability pipeline (Chapter 39):

| Metric | Description | Target |
|--------|-------------|--------|
| `cache.surface.hit_rate` | L1 + L2 combined hit rate | >90% |
| `cache.surface.l1_hit_rate` | In-process cache hit rate | >60% |
| `cache.surface.l2_hit_rate` | Redis hit rate (on L1 miss) | >85% |
| `cache.data.hit_rate` | Data source cache hit rate | Varies by source |
| `cache.auth.hit_rate` | Auth check cache hit rate | >95% |
| `cache.redis.memory_used_bytes` | Redis memory consumption | Alert >80% of limit |
| `cache.invalidation.latency_ms` | Time from publish to cache cleared | p99 <500ms |
| `cache.304_rate` | Fraction of surface fetches returning 304 | >80% in steady state |

### 34.9.2 Cache Debugging

The platform admin surface includes a cache inspector that shows, for a given surface ID and user context:
- Current cache key hash
- Whether a cached entry exists (L1 / L2)
- Cached entry age and TTL remaining
- ETag value
- Trigger to manually invalidate

---

*End of Chapter 34*

**Previous:** [Chapter 33 — Offline Architecture](./33-offline-architecture.md)
**Next:** [Chapter 35 — Data Sync](./35-data-sync.md)
