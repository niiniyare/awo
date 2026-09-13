# Workflow ID Convention

**Classification:** Reference — Tier 1
**Owner:** `08-workflow/WORKFLOW_ID_CONVENTION.md`
**Status:** Frozen at v1.0

---

## Format

```
{tenant_id}.{qualified_entity_name}.{record_id}.{event_name}
```

All components are lowercase, dot-separated.

## Example

```
abc12300-e29b-41d4-a716-446655440000.finance_invoice.inv45600-e29b-41d4-a716-446655440000.on_submit
```

## Component Definitions

| Component | Description | Example |
|-----------|-------------|---------|
| `tenant_id` | UUID of the owning tenant | `abc12300-e29b-41d4-a716-446655440000` |
| `qualified_entity_name` | Module + entity name | `finance_invoice` |
| `record_id` | UUID of the affected entity record | `inv45600-e29b-41d4-a716-446655440000` |
| `event_name` | Lifecycle event in snake_case | `on_submit`, `on_create`, `on_approve` |

## Purpose

The convention serves two goals:

1. **Global uniqueness:** No two workflow starts for the same record + event can have the same ID.
2. **Idempotent start:** Temporal deduplicates workflow starts by ID. If the outbox worker retries a dispatch, Temporal returns the existing workflow instead of starting a duplicate.

## Stability Requirement

Workflow IDs are stored permanently in Temporal's event history. They MUST NOT change format after v1.0. Changing the format would make historical workflow IDs uninterpretable.

## Custom Workflow IDs

`WorkflowTrigger.WorkflowID` allows overriding the generated ID. Use only when a workflow must be reusable across multiple record events (e.g., a long-running tenant onboarding workflow that spans multiple lifecycle events). In that case, omit `{event_name}` from the template.

## References

- [`08-workflow/OUTBOX_SPEC.md`](OUTBOX_SPEC.md) — Where workflow IDs are written
- [`08-workflow/TEMPORAL_INTEGRATION.md`](TEMPORAL_INTEGRATION.md) — WorkflowTrigger declaration
