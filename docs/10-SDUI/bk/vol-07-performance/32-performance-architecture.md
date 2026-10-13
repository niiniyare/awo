# Chapter 32 — Performance Architecture

> **Volume:** VII — Performance, Offline & Sync
> **Phase:** 4
> **Audience:** Backend Engineers, Platform Engineers, Mobile Engineers, Web Engineers
> **Prerequisites:** Chapters 08, 17–21, 24, 25

---

## Table of Contents

- [32.1 Performance Philosophy](#321-performance-philosophy)
- [32.2 Latency Budgets](#322-latency-budgets)
- [32.3 Backend Performance](#323-backend-performance)
- [32.4 Surface Definition Performance](#324-surface-definition-performance)
- [32.5 Data Source Performance](#325-data-source-performance)
- [32.6 Web Engine Performance](#326-web-engine-performance)
- [32.7 Mobile Engine Performance](#327-mobile-engine-performance)
- [32.8 Network Performance](#328-network-performance)
- [32.9 Database Performance](#329-database-performance)
- [32.10 Performance Monitoring](#3210-performance-monitoring)

---

## 32.1 Performance Philosophy

**Measure before optimizing.** Every performance decision in AwoERP is driven by observed data from production telemetry, not speculation. The observability pipeline (Chapter 39) provides the data; this chapter documents the targets and the techniques used to meet them.

**Latency is a feature.** For an ERP platform used by finance staff, procurement teams, and HR departments performing repetitive daily tasks, slow screens compound directly into lost productivity. A 2-second delay on a screen visited 40 times a day costs 80 seconds per user per day — over 6 hours per year per user.

**Budget to the client.** Performance budgets are declared in the surface definition schema (payload size caps, max_rows) and enforced at compile time. This means backend developers writing surface definitions operate within constraints, not guidelines.

**Avoid client-side computation.** The SDUI model deliberately keeps business logic on the server. Complex queries, aggregations, and computations happen in PostgreSQL — not in JavaScript or Swift loops. The client renders what it receives.

---

## 32.2 Latency Budgets

### 32.2.1 Surface Load Latency Targets

| Scenario | p50 | p95 | p99 |
|----------|-----|-----|-----|
| Surface definition fetch (warm cache) | 5ms | 15ms | 30ms |
| Surface definition fetch (cold compile) | 45ms | 120ms | 200ms |
| Data source query (simple list) | 20ms | 60ms | 120ms |
| Data source query (complex join/aggregate) | 50ms | 150ms | 300ms |
| Mutation (create/update) | 30ms | 100ms | 200ms |
| Bulk mutation (50 records) | 100ms | 300ms | 600ms |
| SSE connection establishment | 10ms | 30ms | 60ms |

### 32.2.2 Client-Perceived Latency Targets

| Metric | Web | iOS | Android |
|--------|-----|-----|---------|
| Time to Interactive (first surface) | <2s | <1.5s | <1.5s |
| Navigation between cached surfaces | <200ms | <150ms | <150ms |
| Navigation to uncached surface | <800ms | <600ms | <600ms |
| Form submission feedback | <300ms | <250ms | <250ms |
| Search results (debounced 300ms) | <500ms | <400ms | <400ms |
| SSE patch application | <16ms | <16ms | <16ms |

### 32.2.3 Payload Budgets

| Client | Surface Definition | Single Data Source Response |
|--------|-------------------|---------------------------|
| Web | 200KB (uncompressed) | 500KB |
| Mobile | 100KB (uncompressed) | 200KB |
| Terminal | 30KB (uncompressed) | 50KB |

These budgets are enforced by the compiler (Chapter 08). Compilation fails with an error if the serialized definition exceeds the budget for the target platform.

---

## 32.3 Backend Performance

### 32.3.1 Go Service Architecture

The backend uses Go's concurrency model to handle high request volumes efficiently. Each request is handled by a goroutine. The HTTP server uses `net/http` with a tuned connection pool:

```go
server := &http.Server{
    Addr:         ":8080",
    Handler:      router,
    ReadTimeout:  10 * time.Second,
    WriteTimeout: 30 * time.Second,
    IdleTimeout:  60 * time.Second,
    // Allow graceful drain on shutdown
    MaxHeaderBytes: 1 << 20, // 1MB
}
```

### 32.3.2 Connection Pooling

Database connection pooling is managed by `pgxpool`:

```go
poolConfig, _ := pgxpool.ParseConfig(databaseURL)
poolConfig.MaxConns = 50                       // Per pod
poolConfig.MinConns = 5
poolConfig.MaxConnLifetime = 30 * time.Minute  // Avoid stale connections
poolConfig.MaxConnIdleTime = 5 * time.Minute
poolConfig.HealthCheckPeriod = 1 * time.Minute

pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
```

With a target of 50 connections per pod and 10 pods in a standard deployment, the platform sustains 500 total database connections — within PostgreSQL's default `max_connections = 1000` while leaving headroom for migration scripts and admin sessions.

### 32.3.3 Redis Connection Pool

Redis is used for caching, rate limiting, pub/sub (SSE), and the session revocation set. The connection pool is sized separately per use case:

```go
redisClient := redis.NewClient(&redis.Options{
    Addr:         redisURL,
    PoolSize:     20,             // Per pod
    MinIdleConns: 5,
    DialTimeout:  2 * time.Second,
    ReadTimeout:  1 * time.Second,
    WriteTimeout: 1 * time.Second,
    PoolTimeout:  2 * time.Second,
})
```

### 32.3.4 Request Tracing

Every request carries a `X-Request-ID` header (generated at the API gateway if not provided by the client). This ID propagates through all internal calls and database queries via the context:

```go
func WithRequestID(ctx context.Context, requestID string) context.Context {
    return context.WithValue(ctx, contextKeyRequestID, requestID)
}

// Passed to database session for slow query log attribution
func (s *Store) setSessionContext(ctx context.Context, tx pgx.Tx) error {
    requestID := middleware.RequestID(ctx)
    _, err := tx.Exec(ctx,
        "SET LOCAL application_name = $1",
        fmt.Sprintf("awoerp:%s", requestID))
    return err
}
```

---

## 32.4 Surface Definition Performance

### 32.4.1 Two-Tier Caching

The surface definition cache operates at two tiers:

**Tier 1 — In-process (per pod):**
- Go `sync.Map` or `ristretto` cache
- Capacity: ~200 entries per pod (most-recently-used)
- TTL: 30 seconds (matches `cache_ttl_seconds` in definition)
- Invalidation: explicit key deletion on surface definition update

**Tier 2 — Distributed (Redis):**
- Shared across all pods
- Capacity: configured Redis memory limit (typically 2–4GB for cache)
- TTL: 60 seconds (allows brief pod-level staleness)
- Key: `sdui:{surface_id}:{cache_key_hash}`

```go
func (c *SurfaceCache) Get(ctx context.Context, cacheKey string) (*SurfaceDefinition, bool) {
    // L1: in-process
    if val, ok := c.l1.Get(cacheKey); ok {
        return val.(*SurfaceDefinition), true
    }

    // L2: Redis
    data, err := c.redis.Get(ctx, "sdui:"+cacheKey).Bytes()
    if err == nil {
        var def SurfaceDefinition
        if err := proto.Unmarshal(data, &def); err == nil {
            c.l1.Set(cacheKey, &def, 1) // Promote to L1
            return &def, true
        }
    }

    return nil, false
}
```

Definitions are serialized as Protocol Buffers for Redis storage (compact binary vs JSON).

### 32.4.2 Compilation Latency Breakdown

The 6-stage compiler (Chapter 08) has per-stage latency budgets:

| Stage | Budget (warm) | Budget (cold) |
|-------|--------------|--------------|
| Parse & validate DSL | 1ms | 5ms |
| Resolve references | 1ms | 8ms |
| Apply customizations | 1ms | 5ms |
| Filter by permissions | 2ms | 10ms |
| Optimize & prune | <1ms | 3ms |
| Serialize & compress | 1ms | 8ms |
| **Total** | **~6ms** | **~39ms** |

### 32.4.3 ETag-Based Conditional Fetches

Clients that already have a surface definition cached send `If-None-Match: {etag}`. The server computes the current ETag (SHA-256 of the cache key + current definition version) without compiling:

```go
func (h *SurfaceHandler) checkETag(w http.ResponseWriter, r *http.Request,
    currentETag string) bool {
    clientETag := r.Header.Get("If-None-Match")
    if clientETag == currentETag {
        w.Header().Set("ETag", currentETag)
        w.WriteHeader(http.StatusNotModified)
        return true // Caller should return
    }
    return false
}
```

In steady-state (no surface definition changes), >95% of surface definition requests return 304 with no body, consuming minimal bandwidth and near-zero compute.

---

## 32.5 Data Source Performance

### 32.5.1 Query Optimization

Data source queries are generated from the surface definition's endpoint declarations. The backend uses `sqlc`-generated queries backed by carefully crafted indexes. The performance contract for data source endpoints:

- Every list endpoint must have a covering index for its most common filter/sort combination
- Every `page` + `sort_by` combination must avoid a sequential scan on large tables
- Aggregation queries (used for `summary` fields) run in the same transaction as the list query to avoid double-fetching

### 32.5.2 Parallel Data Source Fetch

When a surface definition declares multiple `eager` data sources, the rendering engine fetches them in parallel (not serially):

**Web:**
```typescript
// TanStack Query: parallel queries for all eager data sources
const results = useQueries({
    queries: eagerSources.map(source => ({
        queryKey: [source.id, resolvedParams(source)],
        queryFn: () => fetchDataSource(source, resolvedParams(source)),
        staleTime: (source.cache_ttl_seconds ?? 30) * 1000,
    }))
});
```

**Go (backend parallel fetch for server-side rendering):**
```go
func fetchAllSources(ctx context.Context, sources []DataSource) map[string]SourceResult {
    results := make(map[string]SourceResult, len(sources))
    var mu sync.Mutex
    var wg sync.WaitGroup

    for _, src := range sources {
        wg.Add(1)
        go func(s DataSource) {
            defer wg.Done()
            result, err := fetchSource(ctx, s)
            mu.Lock()
            results[s.ID] = SourceResult{Data: result, Err: err}
            mu.Unlock()
        }(src)
    }
    wg.Wait()
    return results
}
```

### 32.5.3 Pagination Performance

Cursor-based pagination avoids the `OFFSET` performance cliff (PostgreSQL must scan all preceding rows to satisfy `OFFSET N`):

```sql
-- Offset pagination (degrades at high offsets)
SELECT * FROM invoices WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT 25 OFFSET 500;

-- Cursor pagination (constant time regardless of depth)
SELECT * FROM invoices
WHERE tenant_id = $1
  AND (created_at, id) < ($cursor_timestamp, $cursor_id)
ORDER BY created_at DESC, id DESC
LIMIT 25;
```

For surfaces with `pagination: cursor`, the backend generates cursor-based queries. Cursor pagination is recommended for any list expected to exceed 1,000 records.

---

## 32.6 Web Engine Performance

### 32.6.1 Bundle Splitting

The web rendering engine uses Vite's code splitting to load only what each surface needs:

```typescript
// Lazy-loaded bundles registered in WebComponentRegistry
const componentBundles: Record<string, () => Promise<ComponentBundle>> = {
    'form.*':      () => import('./bundles/forms'),      // 68KB
    'table.*':     () => import('./bundles/tables'),     // 55KB
    'chart.*':     () => import('./bundles/charts'),     // 110KB
    'workflow.*':  () => import('./bundles/workflow'),   // 28KB
    'erp.*':       () => import('./bundles/erp'),        // 85KB
    'dashboard.*': () => import('./bundles/dashboard'),  // 34KB
};
```

The core bundle (42KB) is the only synchronous load. All other bundles load on demand when a surface first uses their component types.

### 32.6.2 Virtual Scrolling

Tables with more than 100 rows use TanStack Virtual to render only the visible window:

```typescript
const rowVirtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 48,    // Row height estimate in px
    overscan: 5,               // Extra rows above/below viewport
});
```

This keeps DOM node count bounded regardless of data volume, maintaining smooth scrolling at 2,000 rows.

### 32.6.3 React Rendering Optimization

The Jotai atom-per-binding pattern (Chapter 18) ensures that data changes only re-render the components that subscribe to that specific binding. A SSE patch that updates one cell does not re-render the entire table.

---

## 32.7 Mobile Engine Performance

### 32.7.1 iOS Memory Management

The iOS rendering engine uses value types (structs) for component nodes and Combine's `@Published` for reactive state to minimize retain cycles and ARC pressure:

```swift
// Value-type component tree avoids reference cycle issues
struct ComponentNode: Codable, Identifiable {
    let id: String
    let type: String
    let props: [String: AnyCodable]
    let children: [ComponentNode]?
}
```

Large lists use `LazyVStack` with explicit `id` parameters to enable SwiftUI's diffing:

```swift
LazyVStack(spacing: 0) {
    ForEach(rows, id: \.id) { row in
        RowView(row: row)
            .id(row.id)
    }
}
```

### 32.7.2 Android Lazy Composition

Jetpack Compose's `LazyColumn` with `key` lambda provides efficient list rendering:

```kotlin
LazyColumn {
    items(
        items = rows,
        key = { row -> row.id }
    ) { row ->
        RowComposable(row = row)
    }
}
```

The `key` lambda enables Compose to reuse existing composables when list items are reordered or updated, avoiding full recomposition.

### 32.7.3 Image Loading

| Platform | Library | Memory Cache | Disk Cache |
|----------|---------|-------------|-----------|
| iOS | `URLCache` | 50MB | 100MB |
| Android | Coil | 25% heap | 250MB |
| Web | Browser cache | Browser-managed | 50MB (Cache API) |

All image requests include `If-None-Match` conditional headers. CDN-served images use immutable cache headers (`Cache-Control: public, max-age=31536000, immutable`) with content-hashed URLs.

---

## 32.8 Network Performance

### 32.8.1 Compression

All API responses are compressed with gzip (web) or Brotli (mobile, where supported). The surface definition payload budget (200KB web, 100KB mobile) is measured uncompressed; typical compression ratios of 3–5x mean wire sizes of 40–66KB (web) and 20–33KB (mobile).

```go
func CompressionMiddleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return handlers.CompressHandlerLevel(next, gzip.BestSpeed)
    }
}
```

### 32.8.2 HTTP/2 and HTTP/3

The API gateway enforces HTTP/2 for all clients, enabling:
- Header compression (HPACK) — reduces per-request overhead for repeated headers (Authorization, X-Tenant-ID, etc.)
- Request multiplexing — multiple data source fetches share one TCP connection
- Server push (limited use: push the session context alongside the first surface definition response)

Mobile clients use HTTP/3 (QUIC) when available, reducing connection establishment latency on poor mobile networks (0-RTT resumption).

### 32.8.3 CDN Configuration

Static assets (JS bundles, fonts, images) are served from a CDN with:
- Cache-Control: `public, max-age=31536000, immutable` (content-hashed URLs)
- Stale-while-revalidate for API responses at the CDN edge
- Regional edge nodes in Nairobi, Lagos, and Johannesburg for African-region tenants

---

## 32.9 Database Performance

### 32.9.1 Index Strategy

Every table follows the index strategy:
1. **Primary lookup:** `(tenant_id, id)` — covered by PRIMARY KEY with tenant_id first
2. **List + sort:** `(tenant_id, status, created_at DESC)` — the most common list pattern
3. **Foreign key traversal:** `(tenant_id, vendor_id)` — for join conditions
4. **Full-text search:** `GIN` index on `to_tsvector('english', name || ' ' || description)`

```sql
-- Invoices index set
CREATE INDEX idx_invoices_list      ON invoices(tenant_id, status, created_at DESC);
CREATE INDEX idx_invoices_vendor    ON invoices(tenant_id, vendor_id);
CREATE INDEX idx_invoices_assignee  ON invoices(tenant_id, assigned_to_id) WHERE assigned_to_id IS NOT NULL;
CREATE INDEX idx_invoices_search    ON invoices USING GIN(to_tsvector('english', invoice_number || ' ' || coalesce(notes, '')));
```

### 32.9.2 Slow Query Detection

Queries slower than 100ms are logged automatically via PostgreSQL's `log_min_duration_statement = 100` setting. The Go application also wraps query execution to measure and emit telemetry:

```go
func (s *Store) queryWithTelemetry(ctx context.Context, name string,
    fn func() error) error {

    start := time.Now()
    err := fn()
    duration := time.Since(start)

    telemetry.RecordDBQuery(ctx, telemetry.DBQueryEvent{
        QueryName: name,
        DurationMs: float64(duration.Milliseconds()),
        Error: err != nil,
    })

    if duration > 100*time.Millisecond {
        slog.WarnContext(ctx, "slow query",
            "query", name,
            "duration_ms", duration.Milliseconds(),
            "request_id", middleware.RequestID(ctx))
    }

    return err
}
```

### 32.9.3 Read Replicas

Read-heavy data source queries (list endpoints, report endpoints) route to PostgreSQL read replicas. Mutations always go to the primary. The store layer handles this automatically:

```go
type Store struct {
    primary  *pgxpool.Pool  // Mutations
    replica  *pgxpool.Pool  // Read-only queries
}

func (s *Store) ListInvoices(ctx context.Context, params ListParams) ([]Invoice, int, error) {
    // Uses replica pool
    return s.listInvoicesFromDB(ctx, s.replica, params)
}

func (s *Store) CreateInvoice(ctx context.Context, params CreateParams) (Invoice, error) {
    // Uses primary pool
    return s.createInvoiceInDB(ctx, s.primary, params)
}
```

---

## 32.10 Performance Monitoring

### 32.10.1 Key Metrics

The performance dashboard (Chapter 39) tracks these metrics in real time:

| Metric | Collection Point | Alert Threshold |
|--------|-----------------|----------------|
| Surface definition p99 latency | API gateway | >300ms |
| Data source query p99 latency | API service | >500ms |
| PostgreSQL query p99 latency | Database metrics | >200ms |
| Cache hit rate (surface definitions) | Redis telemetry | <80% |
| SSE connection count | Stream service | >8,000/pod |
| Web TTI | Client telemetry | >3s |
| Mobile app launch to first surface | Client telemetry | >2s |
| Error rate (5xx) | API gateway | >0.1% |

### 32.10.2 Profiling in Production

Go's built-in `pprof` endpoint is available behind authentication in production:

```go
if config.EnableProfiling {
    r.Mount("/debug/pprof", middleware.BasicAuth("profiling", map[string]string{
        "admin": config.ProfilingPassword,
    })(http.DefaultServeMux))
}
```

CPU and memory profiles are captured automatically for 60-second windows when latency exceeds alert thresholds, and stored in blob storage for post-incident analysis.

---

*End of Chapter 32*

**Previous:** [Chapter 31 — Plugins](../vol-06-platform-apis/31-plugins.md)
**Next:** [Chapter 33 — Offline Architecture](./33-offline-architecture.md)
