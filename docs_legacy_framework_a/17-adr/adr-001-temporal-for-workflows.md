> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-001: Temporal for Workflow Orchestration"
id: adr-001
status: accepted
category: ADR
stability: FROZEN
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[Temporal Integration](../09-workflow/temporal-integration.md)"
  - "[Activities](../09-workflow/activities.md)"
  - "[Sagas](../09-workflow/sagas.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
---

# ADR-001: Temporal for Workflow Orchestration

**Status:** Accepted
**Date:** 2024-01-15
**Authors:** Framework Team

---

## Context

Awo's ERP domain is full of long-running, multi-step processes: invoice approval chains, payroll runs, vendor onboarding, inventory reconciliation. These processes:

- Span multiple systems (database, email, external APIs, accounting engine)
- Take minutes to days to complete
- Must survive process crashes and deployments
- Require audit trails for compliance
- Need compensation (rollback) when steps fail mid-sequence
- Must handle human approval gates (wait for signal before proceeding)

The question was how to implement these processes reliably.

---

## Options Considered

### Option A: Database-Backed Job Queue (e.g., pgmq, River)

Pros: Simple, no additional infrastructure, familiar SQL semantics.

Cons:
- No built-in event history (audit trail requires custom implementation)
- No native saga pattern support (compensation must be custom)
- No signal/query API for human gates
- Long-running workflows (days) require custom heartbeat/timeout logic
- Replay on crash is manual — requires custom checkpointing

### Option B: Message Queue (Kafka, RabbitMQ) with Saga State Machine

Pros: Battle-tested for event-driven systems.

Cons:
- Saga state machine must be implemented and stored explicitly
- Compensation logic is bespoke
- No event replay out of the box
- Significant boilerplate per workflow type
- Requires external state store for workflow position

### Option C: Temporal

Pros:
- Durable execution: workflow state auto-persisted after each activity
- Auto-replay on crash: workflow resumes from last checkpoint after worker restart
- Built-in event history: complete audit trail visible in Temporal Web UI
- Native signal/query API: human approval gates built-in
- Saga pattern trivially implemented via compensation functions
- Go SDK with strong type checking

Cons:
- Additional infrastructure component to operate
- Temporal-specific Go patterns (determinism requirements for workflow functions)
- Vendor dependency (Temporal Inc.)

---

## Decision

**Use Temporal for all workflow orchestration.**

The correctness guarantees (durable execution, auto-replay, at-least-once activity execution) are essential for financial and operational ERP processes where missed or duplicate steps have real consequences. The development productivity benefits (clean saga pattern, built-in signal/query) justify the operational overhead of running Temporal.

---

## Consequences

**Positive:**
- Workflow state survives process crashes — no data loss on deploy or crash
- Complete event history for every workflow — compliance and debugging
- Saga compensation trivially implemented
- Human approval gates via Temporal signals — no custom state machine

**Negative:**
- Workflow functions must be deterministic — developers must learn and follow the determinism rules
- Additional operational surface: Temporal server requires monitoring, storage (PostgreSQL or Cassandra), and the Temporal Web UI
- Worker processes consume resources even when no workflows are running

**Architecture Laws generated:**
- LAW-016: Workflow ID format is canonical
- Determinism rules in WF-001 (temporal-integration.md)

---

## Revisit Trigger

If Temporal's operational overhead becomes prohibitive or the Go SDK is abandoned, reconsider Option A (database-backed job queue) with explicit saga state machine. The outbox pattern (ADR-006) already decouples the dispatch mechanism from the orchestration engine — switching engines requires only changing the relay, not business logic.
