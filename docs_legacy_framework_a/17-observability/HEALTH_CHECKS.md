> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Health Checks

**Classification:** Specification — Tier 1
**Owner:** `17-observability/HEALTH_CHECKS.md`
**Status:** Frozen at v1.0

---

## Purpose

This document specifies the `/health/live` and `/health/ready` endpoints — their contracts, dependency checks, Kubernetes probe configuration, and failure behaviour.

---

## 1. Liveness Probe

```
GET /health/live
```

**Returns:** HTTP 200 with body `{"status":"ok"}` if the process is running.

**Dependency checks:** None. The liveness probe MUST NOT check external dependencies — it only verifies the process is alive and able to accept HTTP connections.

**Kubernetes probe:**
```yaml
livenessProbe:
  httpGet:
    path: /health/live
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
  failureThreshold: 3
```

**On failure:** Kubernetes restarts the container. Only fail liveness when the process is deadlocked or the HTTP server is unresponsive.

---

## 2. Readiness Probe

```
GET /health/ready
```

**Returns:**
- HTTP 200 `{"status":"ok","checks":{...}}` when all dependencies are healthy.
- HTTP 503 `{"status":"unavailable","checks":{...}}` when any dependency is unhealthy.

**Dependency checks:**

| Check | Healthy Condition | Unhealthy Condition |
|-------|-------------------|---------------------|
| PostgreSQL | `db.Ping(ctx)` returns nil within 1s | Timeout or error |
| Redis | `redis.Ping(ctx)` returns "PONG" within 500ms | Timeout or error |
| EntityRegistry | `registry.IsPopulated()` returns true | Registry not yet initialised |

**Response body example (healthy):**

```json
{
  "status": "ok",
  "checks": {
    "postgres": "ok",
    "redis":    "ok",
    "registry": "ok"
  }
}
```

**Response body example (degraded):**

```json
{
  "status": "unavailable",
  "checks": {
    "postgres": "ok",
    "redis":    "connection refused",
    "registry": "ok"
  }
}
```

**Kubernetes probe:**
```yaml
readinessProbe:
  httpGet:
    path: /health/ready
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5
  failureThreshold: 2
```

**On failure:** Kubernetes removes the pod from service endpoints. Requests stop being routed to the pod. The pod is not restarted — it stays running and may recover.

---

## 3. Startup Probe (optional)

For slow-starting pods (e.g., during large migration runs):

```yaml
startupProbe:
  httpGet:
    path: /health/ready
    port: 8080
  failureThreshold: 30
  periodSeconds: 10
```

This allows up to 300 seconds for the pod to become ready before Kubernetes declares startup failure.

---

## 4. Failure Modes and Expected HTTP Status

| Dependency Failure | `/health/live` | `/health/ready` | Traffic Routed? |
|-------------------|---------------|-----------------|-----------------|
| PostgreSQL down | 200 OK | 503 | No |
| Redis down | 200 OK | 503 | No |
| Registry not populated | 200 OK | 503 | No |
| Temporal worker down | 200 OK | 200 OK | Yes (CRUD still works) |
| All dependencies healthy | 200 OK | 200 OK | Yes |

**Temporal failure is not a readiness failure** — the API server serves CRUD requests without Temporal. Workflow start failures are handled gracefully via the outbox pattern.

---

## 5. Authentication

Health check endpoints MUST NOT require authentication. Kubernetes probes do not send auth headers.

Health check endpoints MUST NOT be exposed on the public internet. They are internal service endpoints.

---

## 6. Timeout Budget

Each dependency check has an internal timeout:
- PostgreSQL: 1 second
- Redis: 500 milliseconds
- Registry: 100 milliseconds (in-process check, no network)

Total readiness probe timeout: 2 seconds. The HTTP timeout for the health endpoint MUST be set to at least 3 seconds in Kubernetes probe configuration.

---

## References

- [`17-observability/METRICS_SPEC.md`](METRICS_SPEC.md) — Prometheus metrics
- [`17-observability/LOGGING_SPEC.md`](LOGGING_SPEC.md) — Structured logging
