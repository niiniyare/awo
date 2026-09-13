> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "API Layer — Section Overview"
id: api-000-readme
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[API Conventions](conventions.md)"
  - "[Error Handling](error-handling.md)"
  - "[RBAC](../07-iam/rbac.md)"
  - "[Sessions](../07-iam/sessions.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# API Layer

**Section 11 | API Layer**

The API Layer is Awo's external boundary. It translates HTTP requests into domain operations and HTTP responses into structured envelopes. The framework auto-generates standard CRUD routes from `EntityDefinition`s — custom handlers are for exceptional cases only.

---

## Contents

| Document | ID | Purpose | Stability |
|---|---|---|---|
| [API Conventions](conventions.md) | API-001 | URL structure, response envelope, pagination, datetime serialization | STABLE |
| [Error Handling](error-handling.md) | API-002 | Error types, HTTP status mapping, envelope format, field-level errors | STABLE |
| [Bulk Operations](bulk-operations.md) | API-003 | Bulk create, bulk update, bulk delete, import — atomicity and size limits | STABLE |
| [Webhooks](webhooks.md) | API-004 | Subscriptions, payload format, signature verification, retry schedule | STABLE |
| [Rate Limiting](rate-limiting.md) | API-005 | Sliding window, Redis key structure, per-tenant and per-user limits | STABLE |
| [Entity API Reference](entity-api-reference.md) | API-006 | Complete CRUD endpoint spec: parameters, request/response, error codes | STABLE |
| [Pagination Guide](pagination-guide.md) | API-007 | Cursor pagination: fetching pages, sorting, count, amis integration | STABLE |
| [API Authentication](authentication.md) | API-008 | Session tokens, API client credentials, tenant identification, error responses | FROZEN |
| [Webhooks Consumer Guide](webhooks-guide.md) | API-009 | Subscribe, receive, verify signatures, handle retries, idempotency | STABLE |

---

## Prerequisites

- [Five-Layer Architecture](../02-architecture/five-layer.md) — API Layer constraints
- [EntityDefinition](../03-kernel/entity-def.md) — what drives route generation
- [RBAC](../07-iam/rbac.md) — how permissions are checked in route handlers
- [Sessions](../07-iam/sessions.md) — session validation middleware
- [Glossary](../GLOSSARY.md) — API Layer, Route Handler, Action, Response Envelope
