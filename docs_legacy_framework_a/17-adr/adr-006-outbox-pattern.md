> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "ADR-006: Transactional Outbox for Workflow Dispatch"
id: adr-006
status: accepted
category: ADR
stability: FROZEN
audience: [framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[Outbox Pattern](../09-workflow/outbox-pattern.md)"
  - "[Temporal Integration](../09-workflow/temporal-integration.md)"
  - "[Architecture Laws](../02-architecture/laws.md)"
---

# ADR-006: Transactional Outbox for Workflow Dispatch

**Status:** Accepted
**Date:** 2024-02-01
**Authors:** Framework Team

---

## Context

When an entity is created or updated, a Temporal workflow often needs to start. Naive implementation — write entity to DB, then call `StartWorkflow` — has a failure window: if the process crashes between the DB commit and the Temporal call, the entity is saved but the workflow never starts.

This is incorrect for ERP operations: a submitted invoice must always trigger the approval workflow. A created employee must always trigger onboarding.

---

## Options Considered

### Option A: Temporal Call After DB Commit (Naive)

```go
tx.Commit()
temporal.StartWorkflow(...)  // can fail after commit
```

Cons: Failure window between commit and Temporal call. Workflow may never start.

### Option B: Temporal Call Inside DB Transaction (Wrong Direction)

```go
BEGIN TX
temporal.StartWorkflow(...)  // call while TX is open
tx.Commit()
```

Cons: If Temporal starts the workflow but the DB transaction rolls back, the workflow runs against a record that doesn't exist. Worse than Option A.

### Option C: Two-Phase Commit (XA)

Coordinate DB commit and Temporal call with distributed two-phase commit.

Cons: No XA support in Temporal. Complex to implement. Performance overhead.

### Option D: Transactional Outbox

Write the workflow dispatch request to a `workflow_outbox` table inside the same DB transaction as the entity write. A separate relay process reads pending outbox entries and calls Temporal. On success, marks entry as done.

Pros:
- Atomic: entity record and outbox entry commit together or neither commits
- Relay retries on Temporal failure — at-least-once delivery
- Temporal's `WorkflowIDReusePolicy = REJECT_DUPLICATE` handles relay duplicates (idempotent dispatch)
- Framework-private: module code doesn't know about the outbox

Cons:
- Additional relay process (runs as goroutine in the same process)
- Outbox table adds a small amount of write amplification
- At-least-once (not exactly-once) — workflows must handle idempotent start

---

## Decision

**Use the transactional outbox pattern for all workflow dispatch.**

The correctness guarantee (no workflow missing after entity commit) is worth the complexity. The relay pattern is well-understood and the at-least-once semantic is manageable with Temporal's built-in deduplication.

The outbox table is framework-private — module authors declare `WorkflowTriggers` in their `EntityDefinition`, and the framework handles all outbox operations.

---

## Consequences

**Positive:**
- Workflow dispatch is guaranteed after entity commit — no silent failures
- Module authors use a simple declarative API (`WorkflowTriggers`)
- Temporal's deduplication handles relay duplicates

**Negative:**
- At-least-once delivery: workflow may be dispatched more than once on Temporal failure + relay retry
- Relay polling adds a small dispatch latency (2-5 second poll interval)
- Outbox table grows unboundedly without periodic cleanup of DONE entries

**Architecture Laws generated:**
- LAW-006: Outbox entry and entity record MUST commit atomically; StartWorkflow MUST be outside the TX
- LAW-018: Outbox table is framework-private

---

## Revisit Trigger

If dispatch latency becomes critical (sub-second workflow start required), consider a dedicated workflow dispatch service with a push notification mechanism (listen/notify from PostgreSQL). The outbox table structure can remain; the relay would be event-driven rather than poll-based.
