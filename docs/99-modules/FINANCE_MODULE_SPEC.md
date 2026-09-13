# Finance Module Specification

**Classification:** Reference — Tier 2
**Owner:** `99-modules/FINANCE_MODULE_SPEC.md`
**Status:** Living document

---

## Purpose

This document specifies the Finance module — the canonical reference module for the Awo Framework. Every pattern demonstrated in the Finance module is the exemplar for all other modules.

---

## Module Identity

| Property | Value |
|----------|-------|
| Module name | `finance` |
| Go package | `internal/core/finance` |
| Entity prefix | `finance_` |
| Migration path | `internal/core/finance/migrations/` |
| Canonical roles | `role:finance.viewer`, `role:finance.accounts_payable`, `role:finance.manager` |

---

## Entity Inventory

| Entity (qualified name) | Type | Description |
|------------------------|------|-------------|
| `finance_customer` | System | Billing customer. Name, email, address, KRA PIN. |
| `finance_invoice` | System | Invoice header. Links to customer. Has invoice lines. |
| `finance_invoice_line` | System | Line item on an invoice. Qty × unit price. |
| `finance_payment` | System | Payment against an invoice. |
| `finance_journal_entry` | System | Double-entry journal entry header. |
| `finance_journal_line` | System | Debit or credit line on a journal entry. |
| `finance_ledger_account` | System | Chart of accounts. Asset/Liability/Equity/Revenue/Expense. |
| `finance_ledger_entry` | System | Posted general ledger entry. Immutable after posting. |
| `finance_credit_note` | System | Credit note against a cancelled invoice. |
| `finance_tax_entry` | System | KRA eTIMS tax record. Links to invoice. |

---

## finance_invoice Entity

### Fields

| Name | Type | Notes |
|------|------|-------|
| `number` | `NamingSeries` | Pattern: `INV-{YYYY}-{SEQ:5}`. Immutable after create. |
| `customer_id` | `Link` → `finance_customer` | Required. |
| `status` | `Select` | `Draft` / `Submitted` / `Approved` / `Paid` / `Cancelled` |
| `due_date` | `Date` | |
| `total` | `Currency` | Sum of line totals. Recomputed by `AfterSave` hook on invoice lines. |
| `tax_total` | `Currency` | VAT/tax total. |
| `notes` | `LongText` | |
| `submitted_by` | `Link` → `iam_user` | |
| `submitted_at` | `DateTime` | |
| `approved_by` | `Link` → `iam_user` | |
| `approved_at` | `DateTime` | |
| `cancelled_by` | `Link` → `iam_user` | |
| `cancelled_at` | `DateTime` | |
| `cancel_reason` | `SmallText` | |

### Custom Actions

| Action | Method | Permission Identifier | Trigger |
|--------|--------|----------------------|---------|
| `submit` | POST | `finance.invoice.submit` | Status: Draft → Submitted. Emits `finance.invoice.submitted`. Starts `InvoiceApprovalWorkflow`. |
| `approve` | POST | `finance.invoice.approve` | Status: Submitted → Approved. Emits `finance.invoice.approved`. Four-eyes: approver ≠ submitter. |
| `cancel` | POST | `finance.invoice.cancel` | Status: any except Paid → Cancelled. Body: `{reason}`. Emits `finance.invoice.cancelled`. |
| `record_payment` | POST | `finance.invoice.record_payment` | Creates `finance_payment`. Status: Approved → Paid. |

### Workflow Triggers

| Event | Workflow | Task Queue |
|-------|----------|-----------|
| `on_submit` | `InvoiceApprovalWorkflow` | `finance.invoice.approval` |

### Domain Events

| Topic | When |
|-------|------|
| `finance.invoice.created` | After create |
| `finance.invoice.submitted` | Submit action |
| `finance.invoice.approved` | Approve action |
| `finance.invoice.paid` | Record payment action |
| `finance.invoice.cancelled` | Cancel action |

---

## finance_journal_entry Entity

Double-entry accounting header. Linked to `finance_journal_line` (OneToMany).

| Field | Type | Notes |
|-------|------|-------|
| `reference` | `NamingSeries` | `JE-{YYYY}-{SEQ:6}` |
| `entry_date` | `Date` | |
| `memo` | `SmallText` | |
| `status` | `Select` | `Draft` / `Posted` |
| `total_debit` | `Currency` | Recomputed from lines |
| `total_credit` | `Currency` | Must equal `total_debit` on post |
| `posted_by` | `Link` → `iam_user` | |
| `posted_at` | `DateTime` | |

### Post Action

Custom action `post`:
- Validates `total_debit == total_credit` (double-entry balance)
- Sets `status = 'Posted'`
- Creates `finance_ledger_entry` records from journal lines
- Emits `finance.journal_entry.posted`
- `LedgerEntry` records are immutable once posted

---

## finance_ledger_entry Entity

Immutable general ledger. Append-only by design.

| Field | Type | Notes |
|-------|------|-------|
| `account_id` | `Link` → `finance_ledger_account` | Required |
| `journal_entry_id` | `Link` → `finance_journal_entry` | Source journal entry |
| `debit` | `Currency` | Exactly one of debit or credit is non-zero |
| `credit` | `Currency` | |
| `entry_date` | `Date` | |
| `reference` | `Data` | Human-readable reference |

`Immutable: true` on all fields — no updates allowed after creation.

---

## finance_tax_entry Entity

KRA eTIMS compliance record. Created automatically when an invoice is paid.

| Field | Type | Notes |
|-------|------|-------|
| `invoice_id` | `Link` → `finance_invoice` | |
| `etims_cu_serial` | `Data` | Control Unit serial (from eTIMS API) |
| `etims_invoice_number` | `Data` | eTIMS-assigned invoice number |
| `tax_amount` | `Currency` | |
| `submission_status` | `Select` | `Pending` / `Submitted` / `Failed` |
| `submitted_at` | `DateTime` | |

---

## Permission Identifiers

The Finance module declares these permission identifiers on `PermissionSet`. The IAM module maps roles to these identifiers separately.

| Permission Identifier | Operation |
|-----------------------|-----------|
| `finance.invoice.create` | Create invoices |
| `finance.invoice.read` | Read invoices and invoice lines |
| `finance.invoice.update` | Update draft invoice fields |
| `finance.invoice.delete` | Delete invoices |
| `finance.invoice.submit` | Submit invoice for approval |
| `finance.invoice.approve` | Approve submitted invoice |
| `finance.invoice.cancel` | Cancel an invoice |
| `finance.invoice.record_payment` | Record payment against invoice |
| `finance.customer.create` | Create customers |
| `finance.customer.read` | Read customers |
| `finance.customer.update` | Update customers |
| `finance.journal_entry.create` | Create journal entries |
| `finance.journal_entry.read` | Read journal entries and lines |
| `finance.journal_entry.post` | Post a journal entry |
| `finance.ledger_entry.read` | Read ledger entries (immutable — no create/update/delete) |
| `finance.ledger_account.create` | Create ledger accounts |
| `finance.ledger_account.read` | Read ledger accounts |
| `finance.ledger_account.update` | Update ledger accounts |
| `finance.payment.create` | Create payment records |
| `finance.payment.read` | Read payment records |

**Role-to-permission mapping** (managed by IAM module, not EntityDefinition):

| Role | Permissions granted |
|------|---------------------|
| `role:finance.viewer` | All `finance.*.read` permissions |
| `role:finance.accounts_payable` | `finance.invoice.*`, `finance.payment.*`, `finance.customer.*` |
| `role:finance.manager` | All finance permissions including `approve`, `post`, `journal_entry.*` |
| `role:tenant.admin` | All finance permissions including delete |

---

## Migration Files

```
internal/core/finance/migrations/
  20241201000001_create_finance_customer.up.sql
  20241201000001_create_finance_customer.down.sql
  20241201000002_create_finance_invoice.up.sql
  20241201000002_create_finance_invoice.down.sql
  20241201000003_create_finance_invoice_line.up.sql
  20241201000003_create_finance_invoice_line.down.sql
  20241201000004_create_finance_payment.up.sql
  20241201000004_create_finance_payment.down.sql
  20241201000005_create_finance_ledger_account.up.sql
  20241201000005_create_finance_ledger_account.down.sql
  20241201000006_create_finance_journal_entry.up.sql
  20241201000006_create_finance_journal_entry.down.sql
  20241201000007_create_finance_journal_line.up.sql
  20241201000007_create_finance_journal_line.down.sql
  20241201000008_create_finance_ledger_entry.up.sql
  20241201000008_create_finance_ledger_entry.down.sql
  20241201000009_create_finance_credit_note.up.sql
  20241201000009_create_finance_credit_note.down.sql
  20241201000010_create_finance_tax_entry.up.sql
  20241201000010_create_finance_tax_entry.down.sql
```

---

## References

- [`13-actions/CUSTOM_ACTIONS_EXAMPLES.md`](../13-actions/CUSTOM_ACTIONS_EXAMPLES.md) — Finance action implementations
- [`09-events/DOMAIN_EVENTS_REFERENCE.md`](../09-events/DOMAIN_EVENTS_REFERENCE.md) — Finance events
- [`08-workflow/TEMPORAL_INTEGRATION.md`](../08-workflow/TEMPORAL_INTEGRATION.md) — InvoiceApprovalWorkflow
