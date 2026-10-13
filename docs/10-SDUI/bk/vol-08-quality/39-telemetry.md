# Chapter 39 — Telemetry

> **Volume:** VIII — Quality, Observability & Operations
> **Phase:** 4
> **Audience:** Frontend Engineers, Mobile Engineers, Backend Engineers, Data Engineers
> **Prerequisites:** Chapters 17–21, 38

---

## Table of Contents

- [39.1 Telemetry Philosophy](#391-telemetry-philosophy)
- [39.2 Client Telemetry Events](#392-client-telemetry-events)
- [39.3 Telemetry Event Schema](#393-telemetry-event-schema)
- [39.4 Telemetry Pipeline](#394-telemetry-pipeline)
- [39.5 Web Vitals](#395-web-vitals)
- [39.6 Mobile Performance Telemetry](#396-mobile-performance-telemetry)
- [39.7 Privacy and Consent](#397-privacy-and-consent)
- [39.8 Telemetry in Development](#398-telemetry-in-development)

---

## 39.1 Telemetry Philosophy

Telemetry is the product team's eyes and ears inside the running system. It answers questions that metrics and logs cannot: How are users actually navigating? Which surfaces are slow on real devices in Nairobi? Where are users abandoning multi-step workflows?

**Telemetry is not surveillance.** Every event is:
- Aggregated, not individually identifiable
- Stripped of PII before leaving the client
- Subject to tenant-level opt-out
- Disclosed in the platform's privacy policy

The telemetry system distinguishes between platform telemetry (used by the AwoERP team to improve the platform) and tenant analytics (data a tenant can use to understand their users' behavior). These are separate pipelines with separate retention and access controls.

---

## 39.2 Client Telemetry Events

### 39.2.1 Event Categories

| Category | Events | Used For |
|----------|--------|---------|
| `surface` | load, ready, error | Surface performance tracking |
| `action` | triggered, completed, failed | Action success rate, failure analysis |
| `navigation` | surface_opened, surface_closed, tab_changed | Navigation flow analysis |
| `error` | js_error, api_error, render_error | Error monitoring |
| `performance` | web_vital, compile_latency, data_load_latency | Performance regression detection |
| `sync` | (see Chapter 35.8) | Sync health monitoring |
| `offline` | queue_added, queue_replayed, queue_failed | Offline usage patterns |

### 39.2.2 Core Events

**`surface.load_started`** — Emitted when a surface begins loading (after navigation or initial render).

**`surface.ready`** — Emitted when a surface is interactive (definition compiled + first data loaded).

**`surface.load_failed`** — Emitted when a surface fails to load.

**`action.triggered`** — Emitted when a user triggers any action (button click, form submit).

**`action.completed`** — Emitted when an action successfully completes.

**`action.failed`** — Emitted when an action fails, with the error code.

**`error.js_error`** — Emitted when an unhandled JavaScript error occurs.

**`error.api_error`** — Emitted when an API call returns a non-retryable error code.

---

## 39.3 Telemetry Event Schema

### 39.3.1 Base Event Structure

All telemetry events share a common base structure:

```typescript
interface TelemetryEvent {
    // Identity
    event_name: string;              // e.g., "surface.ready"
    event_id: string;                // UUID, for deduplication
    session_id: string;              // Anonymous session identifier (rotated every 24h)
    tenant_id: string;               // Tenant identifier (NOT user_id)

    // Context
    surface_id: string | null;       // Current surface when event occurred
    platform: 'web' | 'ios' | 'android' | 'terminal';
    client_version: string;          // App version
    locale: string;                  // e.g., "sw-KE"

    // Timing
    timestamp: string;               // ISO 8601 UTC
    client_time_offset_ms: number;   // Offset from server time (for clock skew compensation)

    // Properties
    properties: Record<string, string | number | boolean | null>;
}
```

### 39.3.2 No PII in Telemetry

The client-side telemetry SDK strips PII before events leave the client:

```typescript
class TelemetrySDK {
    private piiFields = new Set([
        'user_id', 'email', 'name', 'phone', 'ip_address',
        'vendor_name', 'invoice_number', 'amount',
    ]);

    track(event: TelemetryEvent): void {
        // Strip PII from properties
        const sanitized = {
            ...event,
            properties: Object.fromEntries(
                Object.entries(event.properties).filter(
                    ([key]) => !this.piiFields.has(key)
                )
            ),
        };
        this.buffer.push(sanitized);
    }
}
```

The `tenant_id` is included (not a PII field — it identifies the organization, not an individual). `user_id` is never included in telemetry events.

### 39.3.3 Event-Specific Properties

**`surface.ready`:**
```typescript
{
    compile_latency_ms: number;    // Time to receive compiled definition
    data_load_latency_ms: number;  // Time for first data source to load
    total_latency_ms: number;      // compile + data
    cache_result: 'hit_l1' | 'hit_l2' | 'compiled' | 'cached_304';
    component_count: number;
}
```

**`action.completed`:**
```typescript
{
    action_id: string;             // Action identifier from surface def
    action_type: string;           // e.g., "action.http_request"
    duration_ms: number;           // Time from trigger to completion
    was_offline_queued: boolean;   // True if executed via offline queue
}
```

**`action.failed`:**
```typescript
{
    action_id: string;
    action_type: string;
    error_code: string;            // API error code, e.g., "validation_error"
    http_status: number | null;
    was_offline_queued: boolean;
}
```

---

## 39.4 Telemetry Pipeline

### 39.4.1 Client Batching

Telemetry events are batched client-side and sent to the ingestion endpoint every 10 seconds, or when the batch reaches 50 events, or on page unload:

```typescript
class TelemetryBatcher {
    private buffer: TelemetryEvent[] = [];
    private readonly maxBatchSize = 50;
    private readonly flushIntervalMs = 10_000;

    constructor() {
        setInterval(() => this.flush(), this.flushIntervalMs);
        window.addEventListener('visibilitychange', () => {
            if (document.visibilityState === 'hidden') this.flush();
        });
    }

    push(event: TelemetryEvent): void {
        this.buffer.push(event);
        if (this.buffer.length >= this.maxBatchSize) {
            this.flush();
        }
    }

    private flush(): void {
        if (this.buffer.length === 0) return;
        const batch = this.buffer.splice(0);
        // Use sendBeacon for unload reliability
        navigator.sendBeacon('/api/telemetry', JSON.stringify(batch));
    }
}
```

### 39.4.2 Ingestion Endpoint

```go
// POST /api/telemetry
// No auth required — events include tenant_id but not user credentials
// Rate limited: 1,000 events per session per minute
func (h *TelemetryHandler) Ingest(w http.ResponseWriter, r *http.Request) {
    var batch []TelemetryEvent
    if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&batch); err != nil {
        http.Error(w, "bad request", 400)
        return
    }

    // Validate and sanitize
    for i := range batch {
        if err := validateEvent(&batch[i]); err != nil {
            continue // Drop invalid events silently
        }
        sanitizeEvent(&batch[i])
        batch[i].ReceivedAt = time.Now().UTC()
    }

    // Publish to Kafka topic
    h.producer.Publish(r.Context(), "telemetry-events", batch)

    w.WriteHeader(204)
}
```

### 39.4.3 Processing Pipeline

```
Client → POST /api/telemetry
    → Kafka topic: telemetry-events
        → Flink streaming job:
            1. Validate event schema
            2. Enrich: add server_received_at, adjust for clock skew
            3. Route: platform_telemetry vs tenant_analytics topics
            4. Deduplicate by event_id (24h window)
        → Clickhouse (platform telemetry — 90-day retention)
        → Tenant analytics DB (per-tenant — configurable retention)
```

### 39.4.4 Telemetry Dashboards

Grafana dashboards built on the Clickhouse data:

**Surface Performance Dashboard:**
- p50/p95/p99 `surface.ready` latency by surface_id and platform
- Surface load failure rate by surface_id
- Cache hit rate by surface_id

**Action Success Dashboard:**
- Action completion rate by action_id
- Action failure breakdown by error_code
- Offline queue usage rate

**Client Error Dashboard:**
- JS error rate by client_version (regression detection)
- API error rate by error_code

---

## 39.5 Web Vitals

### 39.5.1 Core Web Vitals

AwoERP tracks Core Web Vitals for all web surfaces using the `web-vitals` library:

```typescript
import { onCLS, onFID, onLCP, onFCP, onTTFB, onINP } from 'web-vitals';

function initWebVitals(telemetry: TelemetrySDK, surfaceId: string) {
    const report = (metric: Metric) => {
        telemetry.track({
            event_name: 'performance.web_vital',
            surface_id: surfaceId,
            properties: {
                metric_name: metric.name,  // LCP, FID, CLS, FCP, TTFB, INP
                value: metric.value,
                rating: metric.rating,     // 'good' | 'needs-improvement' | 'poor'
                delta: metric.delta,
                id: metric.id,
            },
        });
    };

    onCLS(report);
    onFID(report);
    onLCP(report);
    onFCP(report);
    onTTFB(report);
    onINP(report);
}
```

### 39.5.2 Web Vitals Targets

| Metric | Good | Needs Improvement | AwoERP Target |
|--------|------|-----------------|--------------|
| LCP (Largest Contentful Paint) | < 2.5s | 2.5–4s | p75 < 2.5s |
| INP (Interaction to Next Paint) | < 200ms | 200–500ms | p75 < 200ms |
| CLS (Cumulative Layout Shift) | < 0.1 | 0.1–0.25 | p75 < 0.1 |
| FCP (First Contentful Paint) | < 1.8s | 1.8–3s | p75 < 1.8s |
| TTFB (Time to First Byte) | < 800ms | 800ms–1.8s | p75 < 500ms |

Regressions in web vitals (p75 crossing from "good" to "needs improvement") trigger a Grafana alert routed to the frontend engineering channel.

---

## 39.6 Mobile Performance Telemetry

### 39.6.1 iOS Performance Events

```swift
class PerformanceTelemetry {
    // Measure surface render time
    func measureSurfaceRender(surfaceId: String, block: () -> Void) {
        let start = CACurrentMediaTime()
        block()
        let duration = CACurrentMediaTime() - start

        telemetry.track(TelemetryEvent(
            name: "performance.surface_render",
            properties: [
                "surface_id": surfaceId,
                "duration_ms": duration * 1000,
                "main_thread": Thread.isMainThread,
            ]
        ))
    }

    // Track memory pressure events
    func startMemoryPressureMonitoring() {
        NotificationCenter.default.addObserver(
            forName: UIApplication.didReceiveMemoryWarningNotification,
            object: nil,
            queue: .main
        ) { [weak self] _ in
            self?.telemetry.track(TelemetryEvent(
                name: "performance.memory_pressure",
                properties: [
                    "surface_id": self?.currentSurfaceId ?? "unknown",
                ]
            ))
        }
    }
}
```

### 39.6.2 Android Performance Events

```kotlin
class PerformanceTelemetry(private val telemetry: TelemetrySDK) {
    // Measure frame rendering
    fun measureFrameTime() {
        Choreographer.getInstance().postFrameCallback { frameTimeNanos ->
            val frameTimeMs = frameTimeNanos / 1_000_000.0
            if (frameTimeMs > 16.67) { // Dropped frame (>60fps threshold)
                telemetry.track(TelemetryEvent(
                    name = "performance.dropped_frame",
                    properties = mapOf(
                        "frame_time_ms" to frameTimeMs,
                        "surface_id" to currentSurfaceId,
                    )
                ))
            }
        }
    }
}
```

### 39.6.3 Mobile Performance Targets

| Metric | Target |
|--------|--------|
| Surface render time (cold) | p95 < 500ms |
| Surface render time (warm) | p95 < 100ms |
| Scroll frame rate | p95 ≥ 60fps |
| App launch to first surface | p95 < 2s |
| Background sync duration | p95 < 15s |

---

## 39.7 Privacy and Consent

### 39.7.1 Tenant Opt-Out

Tenants can disable platform telemetry in their settings. When disabled, the client SDK is told via the session context:

```json
{
  "session_config": {
    "telemetry": {
      "platform_telemetry_enabled": false,
      "tenant_analytics_enabled": true
    }
  }
}
```

When `platform_telemetry_enabled` is false, the SDK discards all events before they are buffered.

### 39.7.2 User-Level Opt-Out

Individual users can also opt out of telemetry in their account settings. The opt-out preference is stored server-side and reflected in the session context.

### 39.7.3 Data Retention

| Data | Retention |
|------|-----------|
| Platform telemetry (Clickhouse) | 90 days |
| Aggregated metrics (Prometheus) | 13 months |
| Tenant analytics | Tenant-configured (default 1 year) |
| Web Vitals raw events | 30 days |

After retention expiry, data is permanently deleted (not archived).

---

## 39.8 Telemetry in Development

### 39.8.1 Local Telemetry Console

In development mode, telemetry events are logged to the browser console instead of sent to the ingestion endpoint:

```typescript
const telemetry = process.env.NODE_ENV === 'development'
    ? new ConsoleTelemetrySDK()   // Logs to console
    : new RemoteTelemetrySDK();   // Sends to /api/telemetry
```

The `ConsoleTelemetrySDK` formats events as a readable table:

```
[Telemetry] surface.ready {
  surface_id: "invoice_list",
  compile_latency_ms: 42,
  data_load_latency_ms: 87,
  total_latency_ms: 129,
  cache_result: "compiled"
}
```

### 39.8.2 Telemetry in Integration Tests

Integration tests can assert on telemetry events using a test double:

```typescript
// In Playwright tests
const telemetryCollector = new TelemetryCollector(page);
await telemetryCollector.start();

await page.goto('/invoices');
await page.waitForSelector('[data-testid="invoice-table"]');

const readyEvent = await telemetryCollector.waitForEvent('surface.ready', {
    surface_id: 'invoice_list',
});
expect(readyEvent.properties.total_latency_ms).toBeLessThan(500);
```

---

*End of Chapter 39*

**Previous:** [Chapter 38 — Observability](./38-observability.md)
**Next:** [Chapter 40 — Logging](./40-logging.md)
