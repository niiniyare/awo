# Showcase Schema Audit

**Source:** `erp/web/public/schemas/pages/`
**Destination:** `awo/web/showcase/schemas/pages/`
**Date:** 2026-08-07

## Classification Key

- **A — Keep (static/custom page):** Hand-authored content that cannot be generated from EntityDefinition metadata (e.g. marketing copy, navigation portals, help pages).
- **B — Temporary demo (showcase only):** Useful as a rendered preview but contains no live API wiring or placeholder/hardcoded data. Lives in showcase only; never promoted to production routing.
- **C — Replace with Awo SDUI (entity CRUD):** Manually authored CRUD that duplicates what the SDUI engine generates automatically from EntityDefinition. Must be removed from the schema directory once the real SDUI endpoint is confirmed working. These schemas exist only as a reference for comparing SDUI output against the hand-written version.

---

## Schema Inventory

### Root

| File | Title | Category | Reason |
|---|---|---|---|
| `dashboard.json` | Dashboard | B | Placeholder stat cards with hardcoded `—` values and no live API. Useful as a welcome-screen showcase. Replace with a real dashboard SDUI page once the dashboard engine is wired. |

### `finance/`

| File | Title | Category | Reason |
|---|---|---|---|
| `finance/accounts.json` | Chart of Accounts | C | Duplicate of `GET /api/v1/ui/finance/account` SDUI list view. Uses wrong API path (`/api/v1/account` instead of `/api/v1/finance/account`). Remove once SDUI finance entity is confirmed. |
| `finance/journal-entries.json` | Journal Entries | C | Duplicate of SDUI journal entry list. View-only (no create toolbar). Remove once SDUI journal entity is confirmed. |
| `finance/fiscal-years.json` | Fiscal Years | C | Duplicate of SDUI fiscal year list. Remove once SDUI fiscal year entity is confirmed. |

### `inventory/`

| File | Title | Category | Reason |
|---|---|---|---|
| `inventory/items.json` | Items | C | Duplicate of SDUI inventory item list. Remove once SDUI inventory entity is confirmed. |
| `inventory/warehouses.json` | Warehouses | C | Duplicate of SDUI warehouse list. Remove once SDUI warehouse entity is confirmed. |
| `inventory/stock-entries.json` | Stock Entries | C | Duplicate of SDUI stock entry list. Remove once SDUI stock entry entity is confirmed. |

### `hr/`

| File | Title | Category | Reason |
|---|---|---|---|
| `hr/employees.json` | Employees | C | Duplicate of SDUI employee list. Remove once SDUI HR entity is confirmed. |
| `hr/leave-requests.json` | Leave Requests | C | Duplicate of SDUI leave request list. Remove once SDUI HR entity is confirmed. |
| `hr/payroll-runs.json` | Payroll Runs | C | Duplicate of SDUI payroll run list. Remove once SDUI HR entity is confirmed. |

### `crm/`

| File | Title | Category | Reason |
|---|---|---|---|
| `crm/customers.json` | Customers | C | Duplicate of SDUI CRM customer list. Remove once SDUI CRM entity is confirmed. |
| `crm/contacts.json` | Contacts | C | Duplicate of SDUI CRM contact list. Remove once SDUI CRM entity is confirmed. |
| `crm/leads.json` | Leads | C | Duplicate of SDUI CRM lead list. Remove once SDUI CRM entity is confirmed. |
| `crm/opportunities.json` | Opportunities | C | Duplicate of SDUI CRM opportunity list. Remove once SDUI CRM entity is confirmed. |

### `forecourt/`

| File | Title | Category | Reason |
|---|---|---|---|
| `forecourt/sites.json` | Sites | C | Duplicate of SDUI forecourt site list. Remove once SDUI forecourt entity is confirmed. |
| `forecourt/tanks.json` | Tanks | C | Duplicate of SDUI forecourt tank list. Remove once SDUI forecourt entity is confirmed. |
| `forecourt/shifts.json` | Shift Close | C | Duplicate of SDUI forecourt shift list. Remove once SDUI forecourt entity is confirmed. |

### `platform/`

| File | Title | Category | Reason |
|---|---|---|---|
| `platform/users.json` | Users | C | Duplicate of SDUI platform user list. Remove once SDUI IAM entity is confirmed. |
| `platform/roles.json` | Roles | C | Duplicate of SDUI platform role list. Remove once SDUI IAM entity is confirmed. |
| `platform/tenants.json` | Tenants | C | Duplicate of SDUI platform tenant list. Remove once SDUI tenant entity is confirmed. |

---

## Summary

| Category | Count |
|---|---|
| A — Keep (static/custom) | 0 |
| B — Temporary demo | 1 (`dashboard.json`) |
| C — Replace with SDUI | 20 (all entity CRUD schemas) |

**Total:** 21 files

---

## Migration Path

1. For each Category C schema, confirm the corresponding SDUI entity is registered and the `GET /api/v1/ui/{module}/{resource}` endpoint returns a valid AMIS schema.
2. Verify rendered output matches or exceeds the hand-written schema.
3. Delete the static JSON from `showcase/schemas/` — the nav will point to the SDUI URL, not a static file.
4. The `dashboard.json` (Category B) should remain in showcase until a real dashboard SDUI page is wired (see `dashboard.Registry` in `awo/sdui/dashboard`).

---

## Important: These schemas are NOT the source of truth

Static JSON schemas in `showcase/schemas/` are reference material only. The source of truth for all entity pages is the EntityDefinition registered in `def.Register()`. The SDUI engine generates the AMIS schema at runtime from that definition.

Do not put `showcase/schemas/` files on production routes. The server routes them under `/showcase/*` only.
