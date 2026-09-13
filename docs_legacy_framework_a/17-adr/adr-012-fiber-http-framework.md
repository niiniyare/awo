> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-012: Fiber v2 as the HTTP Framework"
id: adr-012
status: accepted
category: ADR
stability: FROZEN
audience: [framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[API Conventions](../11-api/conventions.md)"
  - "[Five-Layer Architecture](../02-architecture/five-layer.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# ADR-012: Fiber v2 as the HTTP Framework

**Status**: Accepted | **Stability**: Frozen

---

## Context

Awo's API layer needs an HTTP framework that:
- Handles high request volume (multi-tenant, potentially thousands of concurrent requests)
- Supports middleware pipelines with a fixed order
- Has minimal allocation overhead (GC pressure affects latency at scale)
- Is production-proven in Go ERP/SaaS applications
- Has good routing ergonomics for `{entity-type}/{id}/{action}` URL patterns

---

## Decision

Use **Fiber v2** (`github.com/gofiber/fiber/v2`) as the HTTP framework.

Key properties relied upon:
- Zero-allocation routing via `fasthttp`
- Middleware pipeline via `Use()` — supports fixed-order security middleware
- Route groups for entity namespacing: `app.Group("/api/v1/entities/:type")`
- `c.Locals()` for per-request state (request ID, tenant ID, actor)
- Body parsing via `c.BodyParser()` — handles JSON, form data
- Streaming responses for export endpoints

### Why Not net/http Standard Library

The standard library is excellent but requires more boilerplate for routing, middleware composition, and `fasthttp`-level performance. For an ERP handling thousands of concurrent requests across many tenants, allocation reduction matters.

### Why Not Echo or Chi

Both are solid frameworks. Fiber's `fasthttp` backend provides lower allocations per request than either. The routing and middleware API is ergonomically similar to Echo. Fiber's `c.Locals()` maps cleanly to Awo's per-request tenant/actor context pattern.

### Prefork Mode Disabled

Fiber supports a `Prefork: true` mode that forks the process to use multiple CPU cores via `SO_REUSEPORT`. This is incompatible with Awo's architecture: the Temporal worker runs in the same process as the HTTP server. Forked processes would each start a Temporal worker, causing duplicate workflow task processing.

`Prefork` is always `false`. Horizontal scaling is achieved via Kubernetes pod replicas.

---

## Alternatives Considered

### net/http + gorilla/mux (Rejected)

More allocations per request; more boilerplate for the middleware pipeline.

### Echo (Rejected)

Would work well. Fiber's allocation profile is better under sustained high concurrency. Decision was close; Fiber chosen.

### gRPC (Rejected for API Layer)

Awo's UI is browser-based (amis JSON schemas). Browsers cannot call gRPC directly. REST over HTTP/1.1 is the correct choice for the primary API.

---

## Consequences

### Positive

- Low allocation overhead reduces GC pressure under load
- Middleware pipeline supports Awo's fixed 7-step security order
- Route parameter (`c.Params("type")`) cleanly supports entity-type routing

### Negative

- `fasthttp` uses a custom `net.Conn` abstraction — some standard `net/http` middleware is not directly compatible
- Fiber is not `net/http` compatible — cannot use `http.Handler` interfaces without adapter
- `c.Context()` is a `*fasthttp.RequestCtx`, not a `context.Context` — must call `c.UserContext()` for Go context

### The `c.Context()` Gotcha

In Fiber, `c.Context()` returns `*fasthttp.RequestCtx` (the raw fasthttp context). To get the Go `context.Context` (which carries tenant ID, actor, etc.):

```go
// WRONG: c.Context() is fasthttp context, not Go context
tenantID := tenant.IDFromContext(c.Context())  // wrong type

// CORRECT: c.UserContext() returns the Go context.Context
tenantID := tenant.IDFromContext(c.UserContext())
```

This is documented in the module dev guide and caught by the code review checklist.

---

## Related

- [API Conventions](../11-api/conventions.md) — URL structure, middleware pipeline
- [Deployment](../14-operations/deployment.md) — Prefork: false rationale
