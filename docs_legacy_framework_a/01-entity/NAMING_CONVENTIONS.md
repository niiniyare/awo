> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Naming Conventions

**Classification:** Reference — Tier 1
**Owner:** `01-entity/NAMING_CONVENTIONS.md`
**Status:** Frozen at v1.0

---

## Purpose

This document specifies all naming conventions for the Awo Framework. Consistent naming is enforced at compile time where possible and by code review where not.

---

## Entity Names

**Format:** `{module}_{noun}` — both parts in snake_case.

| Pattern | Valid | Invalid |
|---------|-------|---------|
| `finance_invoice` | ✓ | |
| `inventory_stock_move` | ✓ | |
| `iam_user` | ✓ | |
| `platform_organization` | ✓ | |
| `Invoice` | | ✗ — not snake_case, no module |
| `financeInvoice` | | ✗ — camelCase |
| `finance-invoice` | | ✗ — hyphen not allowed |
| `invoice` | | ✗ — no module prefix |

The noun MUST be singular: `invoice` not `invoices`, `user` not `users`.

Compound nouns are allowed: `stock_move`, `org_assignment`, `journal_entry`.

**Stability guarantee:** Entity names are embedded in migration filenames, Temporal workflow IDs (retained for years), and Redis keys. They MUST NEVER be renamed after production data exists.

---

## Field Names

**Format:** snake_case within the entity context.

| Pattern | Valid | Invalid |
|---------|-------|---------|
| `customer_id` | ✓ | |
| `total_amount` | ✓ | |
| `is_active` | ✓ | |
| `customerID` | | ✗ — camelCase |
| `CustomerID` | | ✗ — PascalCase |

**FK field convention:** Link fields to other entities MUST be named `{target_local_name}_id`. Example: a link to `finance_invoice` MUST be named `invoice_id`. The `_id` suffix makes the foreign key relationship immediately apparent.

**Reserved field names:** The following names are reserved by the framework and MUST NOT be declared in `Fields`:

| Field | Reserved by |
|-------|------------|
| `id` | Primary key (UUID) |
| `tenant_id` | Tenant isolation |
| `created_at` | Record creation timestamp |
| `updated_at` | Last modification timestamp |
| `custom_fields` | Runtime custom field JSONB column |

---

## Module Names

**Format:** lowercase ASCII letters only. No underscores, hyphens, or digits.

| Pattern | Valid | Invalid |
|---------|-------|---------|
| `finance` | ✓ | |
| `inventory` | ✓ | |
| `iam` | ✓ | |
| `platform` | ✓ | |
| `hr` | ✓ | |
| `Finance` | | ✗ — uppercase |
| `finance_module` | | ✗ — underscores |
| `finance-crm` | | ✗ — hyphens |

---

## Go Type Names

| Construct | Convention | Example |
|-----------|-----------|---------|
| Entity definition variables | `PascalCase + Definition` | `InvoiceDefinition` |
| Hook structs | `{Entity}{Purpose}Hook` | `InvoiceSubmitGuard`, `CustomerValidator` |
| Activity functions | `{Verb}{Noun}Activity` | `SendWelcomeEmailActivity`, `PostJournalEntryActivity` |
| Workflow functions | `{Entity}{Event}Workflow` | `InvoiceApprovalWorkflow`, `OnboardingWorkflow` |
| Action handler functions | `{Verb}{Entity}Action` | `SubmitInvoiceAction`, `CancelOrderAction` |
| Service types | `{Entity}Service` | `InvoiceService` |

---

## HTTP Action Names

**Format:** lowercase hyphen-separated. MUST be a verb or verb-noun combination.

| Pattern | Valid | Invalid |
|---------|-------|---------|
| `submit` | ✓ | |
| `send-reminder` | ✓ | |
| `approve` | ✓ | |
| `cancel` | ✓ | |
| `Submit` | | ✗ — uppercase |
| `sendReminder` | | ✗ — camelCase |
| `submitted` | | ✗ — past tense |

Actions are appended to the entity URL: `POST /api/v1/finance/invoices/{id}/submit`.

---

## Role Names

**Format:** `role:{domain}.{name}`

| Pattern | Valid | Examples |
|---------|-------|---------|
| `role:{module}.{role_name}` | ✓ | `role:finance.accounts_payable`, `role:inventory.manager` |
| `role:tenant.{role_name}` | ✓ | `role:tenant.admin`, `role:tenant.user` |
| `role:platform-admin` | ✓ | Platform administrator |

Module role names use the module prefix. Tenant-scoped roles use the `tenant.` prefix. The platform admin role is the only hyphenated role name.

---

## Temporal Workflow Names

**Format:** `{Entity}{Event}Workflow` in Go (registered name); `{tenant}.{entity}.{record_id}.{event}` for workflow IDs.

**Workflow ID convention:** `{tenant-uuid}.{qualified-entity-name}.{record-id}.{event-name}`

Example: `abc123-def4-5678-9012-abcdefabcdef.finance_invoice.inv456-abc1-2345-6789-0123456789ab.on_submit`

This convention guarantees globally unique workflow IDs and enables workflow deduplication.

See [`08-workflow/WORKFLOW_ID_CONVENTION.md`](../08-workflow/WORKFLOW_ID_CONVENTION.md).

---

## Redis Key Patterns

| Key | Pattern |
|-----|---------|
| Session | `session:{token}` |
| Feature flag | `eval:{sha256(flag_name+tenant_id+user_id)}` |
| SDUI page schema | `page:{entity_name}:{version}:{tenant_id}` |
| Rate limit window | `rl:{tenant_id}:{user_id}:{window_start_unix}` |
| Naming series counter | `naming:{pattern_hash}:{tenant_id}:{period}` |
| Idempotency response | `idempotency_cache:{tenant_id}:{key}` |

---

## Database Object Names

| Object | Convention | Example |
|--------|-----------|---------|
| Table | `{qualified_entity_name}` | `finance_invoice`, `iam_user` |
| FK column | `{local_entity_name}_id` | `invoice_id`, `customer_id` |
| Index | `{table}_{columns}[_idx]` | `finance_invoice_customer_id_idx` |
| Concurrent index | `{table}_{columns}_idx` | Same; created with `CONCURRENTLY` |
| RLS policy | `tenant_isolation` on each table | `tenant_isolation` |
| Migration file | `{unix_timestamp}_{description}.up.sql` | `20241215143022_create_finance_invoice.up.sql` |

---

## Migration Timestamps

Migration files MUST use 14-digit Unix timestamps with zero-padding: `YYYYMMDDHHMMSS`.

```
20241215143022_create_finance_invoice.up.sql
20241215143022_create_finance_invoice.down.sql
```

Timestamps MUST be unique and monotonically increasing within the migration sequence. Use actual wall-clock time at time of writing.

---

## References

- [`01-entity/ENTITY_DEFINITION_SPEC.md`](ENTITY_DEFINITION_SPEC.md)
- [`08-workflow/WORKFLOW_ID_CONVENTION.md`](../08-workflow/WORKFLOW_ID_CONVENTION.md)
- [`15-migrations/MIGRATION_GUIDE.md`](../15-migrations/MIGRATION_GUIDE.md)
