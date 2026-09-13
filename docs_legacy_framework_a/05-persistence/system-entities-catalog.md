> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "System Entity Catalog"
id: pers-008
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, framework-authors]
since: "1.0"
normative-level: normative
related:
  - "[System Entities](system-entities.md)"
  - "[Entity Repository](entity-repository.md)"
  - "[Business Module Catalog](../10-modules/business-modules.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# System Entity Catalog

**PERS-008 | Status: Accepted | Stability: Stable**

Complete list of all system entities shipped with Awo, their mandatory classification, and the reason each requires SQL-backed storage.

---

## Platform System Entities (always required)

These entities exist before any business module is activated:

| Entity | Table | Mandatory system entity reason |
|---|---|---|
| `tenant` | `tenants` | Platform identity; accessible before per-tenant schemas load; global table |
| `iam_user` | `iam_users` | IAM data; JSONB corruption risk for auth operations |
| `iam_role` | `iam_roles` | Role definitions; Casbin reads these directly |
| `iam_permission` | `iam_permissions` | Permission assignments; Casbin-consumed |
| `iam_session` | Stored in Redis only | Session tokens are Redis-native; not in PostgreSQL |
| `iam_audit_log` | `iam_audit_log` | Tamper-evident; SQL immutability constraints enforced at DB level |
| `naming_series_counter` | `naming_series_counters` | Atomic counter requires `UPDATE ... RETURNING`; JSONB cannot provide this |
| `custom_field_def` | `custom_field_definitions` | Schema metadata; must persist before custom entity data |
| `platform_setting` | `platform_settings` | Settings hierarchy; read at every request, must be fast |
| `feature_flag` | `feature_flags` | Flag definitions; evaluated at every request |
| `module_registration` | `module_registrations` | Module activation per tenant |

---

## Finance System Entities

| Entity | Table | Mandatory system entity reason |
|---|---|---|
| `finance_invoice` | `finance_invoices` | High-frequency writes; AR aging queries require SQL indexes on `status` + `due_date` |
| `finance_ledger_entry` | `finance_ledger_entries` | Double-entry integrity; `debit = credit` CHECK constraint required at DB level |
| `finance_journal` | `finance_journals` | Groups ledger entries; FK constraint from ledger_entry required |
| `finance_payment` | `finance_payments` | Financial transaction; SQL constraints + immutability enforcement |
| `finance_tax_entry` | `finance_tax_entries` | KRA regulatory compliance; immutability enforced at DB role level |
| `finance_account` | `finance_accounts` | Chart of accounts; referenced by FK from ledger_entry |
| `finance_purchase_order` | `finance_purchase_orders` | AP matching; requires SQL indexes on `status` + `supplier_id` |

---

## Inventory System Entities

| Entity | Table | Mandatory system entity reason |
|---|---|---|
| `inventory_product` | `inventory_products` | High-frequency reads; FK target from stock moves |
| `inventory_stock_move` | `inventory_stock_moves` | Inventory accuracy; quantity constraints at SQL level; high write volume |
| `inventory_location` | `inventory_locations` | FK target from stock moves; tree structure requires SQL adjacency list |
| `inventory_valuation` | `inventory_valuations` | FIFO costing accuracy; numeric precision required; FK constraints |

---

## HR System Entities

| Entity | Table | Mandatory system entity reason |
|---|---|---|
| `hr_employee` | `hr_employees` | Links to `iam_user`; FK constraint required |
| `hr_department` | `hr_departments` | Tree structure (adjacency list); FK targets from employee |
| `hr_contract` | `hr_contracts` | Payroll basis; date range queries require SQL indexes |
| `hr_leave_request` | `hr_leave_requests` | Approval workflow; high-frequency status updates |
| `hr_attendance` | `hr_attendance` | High write volume (clock-in/out); time-range queries require SQL indexes |

---

## Payroll System Entities

| Entity | Table | Mandatory system entity reason |
|---|---|---|
| `payroll_run` | `payroll_runs` | Immutable after posting; SQL-level immutability guard |
| `payroll_entry` | `payroll_entries` | Per-employee amounts; Currency type required; FK to payroll_run |
| `payroll_statutory` | `payroll_statutory` | Regulatory deduction records; KRA audit requirement |

---

## Forecourt System Entities

| Entity | Table | Mandatory system entity reason |
|---|---|---|
| `forecourt_pump` | `forecourt_pumps` | Real-time reading updates; high write frequency |
| `forecourt_shift` | `forecourt_shifts` | Shift reconciliation; SQL date/time range queries |
| `forecourt_dip` | `forecourt_dips` | Dip readings; time-series queries require SQL index on `dip_time` |
| `forecourt_price` | `forecourt_prices` | Price history; effective date queries require `date` index |

---

## Global Tables (no RLS, no tenant scope)

These tables are not tenant-scoped:

| Table | Purpose |
|---|---|
| `tenants` | Platform tenant registry |
| `iam_audit_log` | Cross-tenant tamper-evident log |
| `timezones` | IANA timezone catalog |
| `currencies` | ISO 4217 currency codes |
| `countries` | ISO 3166 country codes |
| `paye_bands` | KRA tax bands (updated per Finance Act) |
| `platform_admins` | Platform administrator accounts |

Global tables have no RLS policies. The app role has `SELECT` but not `INSERT`/`UPDATE`/`DELETE` on most global tables — they are updated via migrations or a dedicated admin interface.

---

## Entity Classification Decision Guide

Use this table to decide whether a new entity should be a system entity:

| Question | If Yes |
|---|---|
| Does it participate in double-entry accounting? | System entity |
| Does it require FK constraints to other system entities? | System entity |
| Does it require DB-level immutability? | System entity |
| Is it read by Casbin, RLS policies, or session validation? | System entity |
| Does it have >100 writes/second expected? | System entity |
| Does it store monetary amounts used in financial calculations? | System entity |
| Is it referenced as a FK target from another system entity? | System entity |
| None of the above | Custom entity (JSONB) |

---

## Related Documents

- [System Entities](system-entities.md) — schema conventions, RLS migration requirements
- [Custom Entities](custom-entities.md) — JSONB storage, escalation criteria
- [Business Module Catalog](../10-modules/business-modules.md) — module-level entity grouping
