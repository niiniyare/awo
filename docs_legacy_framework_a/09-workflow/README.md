> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Workflow — Section Overview"
id: wf-000-readme
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[Temporal Integration](temporal-integration.md)"
  - "[Activities](activities.md)"
  - "[Sagas](sagas.md)"
  - "[Outbox Pattern](outbox-pattern.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Workflow

**Section 09 | Workflow Layer**

The Workflow Layer handles all asynchronous, durable, long-running processes. It is implemented using Temporal and provides durability, retries, saga-based compensation, and complete event history.

---

## Contents

| Document | ID | Purpose | Stability |
|---|---|---|---|
| [Temporal Integration](temporal-integration.md) | WF-001 | Temporal client setup, worker registration, workflow ID format, task queues | STABLE |
| [Activities](activities.md) | WF-002 | Activity pattern, dependency injection, retry configuration | STABLE |
| [Sagas](sagas.md) | WF-003 | Saga pattern, SagaCompensator, compensation ordering | STABLE |
| [Outbox Pattern](outbox-pattern.md) | WF-004 | Transactional outbox: schema, relay, at-least-once dispatch | FROZEN |
| [Signal Patterns](signal-patterns.md) | WF-005 | Temporal signals, queries, approval gates, state machines | STABLE |
| [Scheduled Workflows](scheduled-workflows.md) | WF-006 | Temporal Schedules, per-tenant fan-out, recurring jobs | STABLE |
| [Saga Pattern](saga-pattern.md) | WF-007 | SagaCompensator helper, LIFO compensation, idempotency, failure handling | STABLE |
| [Workflow Error Handling](error-handling.md) | WF-008 | Retry policies, non-retryable errors, timeouts, heartbeats, manual intervention | STABLE |

---

## Prerequisites

- [Architecture Laws](../02-architecture/laws.md) — LAW-006 (outbox), LAW-016 (workflow ID format), LAW-018 (outbox is framework-private)
- [Five-Layer Architecture §5](../02-architecture/five-layer.md#5-workflow-layer) — Workflow Layer responsibilities
- [EntityDefinition §8](../03-kernel/entity-def.md#8-workflow-triggers) — WorkflowTrigger declarations
- [Glossary](../GLOSSARY.md) — Workflow, Activity, Saga Pattern, Outbox Pattern, Temporal, Task Queue, Workflow ID
