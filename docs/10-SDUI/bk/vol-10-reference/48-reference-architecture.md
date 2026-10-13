# Chapter 48 — Reference Architecture

> **Volume:** X — Reference
> **Phase:** 4
> **Audience:** All Engineers, Architects
> **Prerequisites:** All previous chapters

---

## Table of Contents

- [48.1 System Overview](#481-system-overview)
- [48.2 Request Flow: Surface Load](#482-request-flow-surface-load)
- [48.3 Request Flow: Mutation](#483-request-flow-mutation)
- [48.4 Request Flow: Workflow](#484-request-flow-workflow)
- [48.5 Data Architecture](#485-data-architecture)
- [48.6 Deployment Architecture](#486-deployment-architecture)
- [48.7 Security Architecture](#487-security-architecture)
- [48.8 Component Inventory](#488-component-inventory)

---

## 48.1 System Overview

AwoERP is a multi-tenant ERP platform. Its architecture reflects three constraints:

1. **Multi-tenancy at the database layer** — Row-level security enforces tenant isolation at the PostgreSQL layer, not in application code.
2. **Server-driven UI** — Clients are rendering engines; all product logic lives in the surface compiler on the server.
3. **Async-first operations** — Any operation that may take more than a second is a Temporal workflow, not a synchronous HTTP call.

```
┌─────────────────────────────────────────────────────────────────┐
│                          Clients                                 │
│   ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐       │
│   │   Web    │  │   iOS    │  │ Android  │  │Terminal  │       │
│   │  (React) │  │ (Swift)  │  │ (Kotlin) │  │  (TUI)   │       │
│   └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘       │
└────────┼─────────────┼─────────────┼──────────────┼─────────────┘
         │             │             │              │
         └─────────────┴─────────────┴──────────────┘
                                 │ HTTPS
                     ┌───────────▼──────────────┐
                     │       API Gateway         │
                     │  (auth, rate limit, TLS)  │
                     └───────────┬──────────────┘
                                 │ gRPC / HTTP
               ┌─────────────────┼─────────────────┐
               │                 │                 │
    ┌──────────▼──────┐ ┌───────▼────────┐ ┌──────▼──────────┐
    │  Surface Service │ │  Core API      │ │  Workflow API   │
    │  (SDUI compiler) │ │  (Goa/Go)     │ │  (Temporal)     │
    └──────────┬──────┘ └───────┬────────┘ └──────┬──────────┘
               │                │                 │
        ┌──────┴────────────────┴─────────────────┴──────┐
        │                  Shared Services                │
        │  PostgreSQL (RLS) │ Redis │ OpenFGA │ Object   │
        │                   │       │         │ Storage  │
        └─────────────────────────────────────────────────┘
```

---

## 48.2 Request Flow: Surface Load

The surface load request is the primary read path. Clients call it on every screen navigation.

```
Client
  │
  │ GET /api/v1/surfaces/{surface_id}
  │ Authorization: Bearer <jwt>
  │ X-Tenant-ID: <tenant_id>
  │ X-Client-Version: 2.3.1
  │
  ▼
API Gateway
  │ 1. Verify JWT signature; extract user_id, tenant_id
  │ 2. Rate limit by tenant_id
  │ 3. Attach request_id (ULID)
  │
  ▼
Surface Service
  │ 4. Check Redis cache: surface:{surface_id}:tenant:{tenant_id}:role:{role}
  │    Cache hit  → skip to step 9
  │    Cache miss → continue
  │
  │ 5. Load surface definition from DB (surface_definitions table, RLS enforced)
  │
  │ 6. Check authorization:
  │    FGA: user:{user_id} can view surface:{surface_id}?
  │
  │ 7. Compile surface definition:
  │    a. Resolve locale (user pref → tenant default → Accept-Language → en-KE)
  │    b. Apply role-based visibility (remove fields/actions not visible to role)
  │    c. Resolve label keys → translated strings (from pre-built translation bundle)
  │    d. Resolve data sources (generate data fetch instructions)
  │    e. Apply tenant label overrides (4-level resolution)
  │    f. Validate compiled output against JSON Schema
  │
  │ 8. Cache compiled surface in Redis (TTL: 5 minutes)
  │
  │ 9. Return compiled surface definition
  │
  ▼
Client
  │ 10. Render surface using the rendering engine
  │ 11. Fetch data sources declared in the surface definition
  │ 12. Open SSE connection for live updates
```

**Key latency targets:** p50 < 30ms (cache hit), p99 < 300ms (cache miss + compile)

---

## 48.3 Request Flow: Mutation

Mutations follow the command pattern: validate → authorize → execute → notify.

```
Client
  │
  │ POST /api/v1/invoices/{id}/approve
  │ Authorization: Bearer <jwt>
  │ Idempotency-Key: <client-generated-uuid>
  │
  ▼
API Gateway
  │ 1. Verify JWT; attach request_id
  │ 2. Check idempotency key in Redis
  │    Duplicate → return cached response
  │
  ▼
Core API (Goa handler)
  │ 3. Validate request against OpenAPI schema (middleware)
  │ 4. Set tenant context: SET LOCAL app.tenant_id = <tenant_id>
  │ 5. Authorize:
  │    FGA: user:{user_id} can approve invoice:{invoice_id}?
  │    → Forbidden → 403
  │
  │ 6. Execute in transaction:
  │    a. Read invoice (RLS enforced)
  │    b. Validate business rules (status must be "pending_approval")
  │    c. Update invoice status to "approved"
  │    d. Write audit log entry (append-only)
  │    e. Commit
  │
  │ 7. Cache invalidation:
  │    Publish to Redis channel: surface:invoice_list:tenant:{tenant_id}
  │    (SurfaceIndex maps invoice → invoice_list surface)
  │
  │ 8. Return 200 with updated resource
  │
  ▼
SSE Service (separate goroutine)
  │ 9. Receive cache invalidation event
  │ 10. Recompile invoice_list surface
  │ 11. Build patch document (operations to update the displayed list)
  │ 12. Publish patch to SSE channels for all connected clients on this surface+tenant
  │
  ▼
Client
  │ 13. Receive SSE patch
  │ 14. Apply patch to local surface state
  │ 15. Re-render changed components
```

**Key latency target:** p99 < 500ms end-to-end (API handler only, excluding SSE propagation)

---

## 48.4 Request Flow: Workflow

Long-running operations are Temporal workflows, decoupled from the HTTP request.

```
Client
  │
  │ POST /api/v1/invoices/export
  │ { "format": "csv", "filters": { "date_range": "2024-Q1" } }
  │
  ▼
Core API
  │ 1. Validate and authorize
  │ 2. Start Temporal workflow:
  │    workflow.ExecuteWorkflow(ctx, ExportInvoicesWorkflow, ExportParams{
  │        TenantID:  tenantID,
  │        Format:    "csv",
  │        Filters:   filters,
  │        RequestedBy: userID,
  │    })
  │ 3. Return 202 Accepted with job_id
  │
  ▼
Client
  │ Polls GET /api/v1/jobs/{job_id} for status
  │   OR subscribes to SSE for job completion notification
  │
  ▼
Temporal Worker
  │ ExportInvoicesWorkflow:
  │   Activity: FetchInvoices(ctx, tenantID, filters)  → page through DB
  │   Activity: FormatCSV(ctx, invoices)               → build CSV in memory
  │   Activity: UploadToObjectStorage(ctx, csvData)    → write to S3/GCS
  │   Activity: NotifyCompletion(ctx, userID, fileURL) → SSE notification
  │
  ▼
Client
  │ Receives SSE notification with download URL
  │ Downloads file from object storage (pre-signed URL, 1hr TTL)
```

**Key design principles:** Job ID returned immediately; workflow is retryable; client is not blocked.

---

## 48.5 Data Architecture

### 48.5.1 Primary Store: PostgreSQL

All transactional data lives in PostgreSQL. Schema design:

- Tenant data is co-located in the same database (not separate databases per tenant)
- Every table that holds tenant data has a `tenant_id UUID NOT NULL` column
- RLS policies on every tenant table enforce `app.tenant_id` context
- Audit-sensitive tables have append-only mirror tables in a separate audit schema

### 48.5.2 Cache: Redis

Redis serves three purposes:
- **Surface cache:** Compiled surface definitions (key: `surface:{id}:tenant:{tid}:role:{role}`, TTL 5m)
- **Idempotency store:** Request deduplication (key: `idempotency:{key}`, TTL 24h)
- **SSE pub/sub:** Real-time event delivery between API servers and SSE handlers

### 48.5.3 Blob Storage: Object Storage (S3-compatible)

File attachments, invoice PDFs, export files, and report outputs are stored in object storage. Application code stores object keys in the database; files are never stored as database BLOBs.

### 48.5.4 Workflow State: Temporal

Temporal persists workflow state in its own PostgreSQL database (separate from the application database). Workflow history is the source of truth for long-running operations.

### 48.5.5 Authorization State: OpenFGA

OpenFGA stores the relationship tuples that define authorization (who can do what to which resource). Tuples are written on entity creation and updated on permission changes. The FGA store is the source of truth for authorization; the application database is the source of truth for entity data.

---

## 48.6 Deployment Architecture

```
                        ┌─────────────────────┐
                        │   Load Balancer      │
                        │   (TLS termination)  │
                        └──────────┬──────────┘
                                   │
              ┌────────────────────┼────────────────────┐
              │                   │                    │
   ┌──────────▼──────┐  ┌────────▼────────┐  ┌───────▼──────────┐
   │  API Pods        │  │  Surface Pods   │  │  Worker Pods      │
   │  (3–10 replicas) │  │  (2–6 replicas) │  │  (2–8 replicas)  │
   │  HPA: CPU 60%    │  │  HPA: CPU 70%   │  │  HPA: queue depth│
   └──────────┬──────┘  └────────┬────────┘  └───────┬──────────┘
              │                  │                    │
              └──────────────────┼────────────────────┘
                                 │
         ┌───────────────────────┼────────────────────────┐
         │                       │                        │
┌────────▼──────┐      ┌─────────▼──────┐      ┌─────────▼──────┐
│  PostgreSQL   │      │    Redis        │      │  Temporal       │
│  (primary +   │      │  (cluster)      │      │  (cluster)      │
│   2 replicas) │      │                 │      │                 │
└───────────────┘      └─────────────────┘      └─────────────────┘
```

All Kubernetes resources are defined in the `deploy/` directory. Horizontal Pod Autoscalers scale API and surface pods on CPU utilization; worker pods scale on Temporal task queue depth.

---

## 48.7 Security Architecture

The security architecture has five layers:

| Layer | Mechanism | Chapter |
|-------|-----------|---------|
| **Transport** | TLS 1.3 everywhere; mTLS between internal services | Ch 22 |
| **Authentication** | JWT signed with rotating RS256 keys | Ch 12 |
| **Coarse authorization** | OpenFGA relationship tuples (user → relation → resource) | Ch 13 |
| **Fine authorization** | Field/action visibility in surface compiler | Ch 14 |
| **Data isolation** | PostgreSQL RLS enforced at the database layer | Ch 28 |

Each layer is independent. A flaw in one layer does not compromise the others. An attacker who somehow bypasses JWT verification is stopped by RLS. An attacker who bypasses RLS (e.g., via a direct DB connection) does not bypass TLS on the wire.

---

## 48.8 Component Inventory

| Component | Technology | Purpose |
|-----------|-----------|---------|
| Web client | React, TypeScript, Vite | SDUI rendering engine for browsers |
| iOS client | Swift, UIKit | SDUI rendering engine for iOS |
| Android client | Kotlin, Jetpack Compose | SDUI rendering engine for Android |
| API gateway | Nginx / Envoy | TLS termination, auth, rate limiting |
| Core API | Go, Goa | Business logic, CRUD endpoints |
| Surface service | Go | SDUI compilation, surface cache, SSE |
| Temporal workers | Go | Long-running workflow execution |
| PostgreSQL | PostgreSQL 16 | Primary transactional store with RLS |
| Redis | Redis 7 | Surface cache, idempotency, pub/sub |
| Temporal | Temporal 1.24 | Workflow orchestration and state |
| OpenFGA | OpenFGA | Authorization relationship store |
| Object storage | S3-compatible | Files, exports, attachments |
| Prometheus | Prometheus | Metrics collection |
| Grafana | Grafana | Metrics dashboards |
| Jaeger | Jaeger | Distributed trace storage and UI |
| Loki | Grafana Loki | Log aggregation |
| Kafka | Apache Kafka | Telemetry event ingestion |
| ClickHouse | ClickHouse | Telemetry analytics store |

---

*End of Chapter 48*

**Previous:** [Chapter 47 — Anti-Patterns](./47-anti-patterns.md)
**Next:** [Chapter 49 — End-to-End Examples](./49-e2e-examples.md)
