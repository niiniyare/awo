# Domain Events Reference

**Classification:** Reference — Tier 2
**Owner:** `09-events/DOMAIN_EVENTS_REFERENCE.md`
**Status:** Living document — updated when new modules define events

---

## Purpose

This document catalogs all domain event topics defined by the framework's built-in modules. Third-party modules define their own topics following the same convention.

---

## Topic Naming Convention

```
{module}.{entity_local_name}.{past_tense_verb}
```

All lowercase, dot-separated. Always use past tense (event already occurred).

---

## Finance Module Events

| Topic | Trigger | Key Payload Fields |
|-------|---------|-------------------|
| `finance.invoice.created` | Invoice record created | `invoice_id`, `customer_id`, `total`, `tenant_id` |
| `finance.invoice.submitted` | Invoice submit action | `invoice_id`, `customer_id`, `total`, `submitted_by` |
| `finance.invoice.approved` | Invoice approve action | `invoice_id`, `approved_by`, `approved_at` |
| `finance.invoice.paid` | Payment status changed to Paid | `invoice_id`, `payment_id`, `amount_paid` |
| `finance.invoice.cancelled` | Invoice cancel action | `invoice_id`, `cancelled_by`, `reason` |
| `finance.payment.created` | Payment record created | `payment_id`, `invoice_id`, `amount`, `method` |
| `finance.journal_entry.posted` | Journal entry posted | `entry_id`, `reference`, `total_debit` |

---

## IAM Module Events

| Topic | Trigger | Key Payload Fields |
|-------|---------|-------------------|
| `iam.user.created` | User account created | `user_id`, `email`, `tenant_id` |
| `iam.user.activated` | User account activated | `user_id`, `activated_by` |
| `iam.user.suspended` | User account suspended | `user_id`, `suspended_by`, `reason` |
| `iam.user.role_assigned` | Role assigned to user | `user_id`, `role`, `assigned_by` |
| `iam.user.password_changed` | Password changed | `user_id`, `changed_at` |

---

## Tenant Module Events

| Topic | Trigger | Key Payload Fields |
|-------|---------|-------------------|
| `platform.tenant.created` | Tenant record created | `tenant_id`, `name`, `slug` |
| `platform.tenant.activated` | Tenant status → ACTIVE | `tenant_id`, `activated_at` |
| `platform.tenant.suspended` | Tenant status → SUSPENDED | `tenant_id`, `reason` |
| `platform.tenant.archived` | Tenant status → ARCHIVED | `tenant_id`, `archived_at` |

---

## Subscribing to Events

Consumers subscribe to topics via the `EventBroker` implementation. For Kafka:

```go
// Subscribe to all finance.invoice.* events
broker.Subscribe("finance.invoice.*", handleInvoiceEvent)
```

For NATS with subject-based routing:
```go
nc.Subscribe("finance.invoice.submitted", handleInvoiceSubmitted)
```

Consumers MUST be idempotent — event delivery is at-least-once.

---

## References

- [`09-events/EVENT_OUTBOX_SPEC.md`](EVENT_OUTBOX_SPEC.md) — Event storage and delivery
