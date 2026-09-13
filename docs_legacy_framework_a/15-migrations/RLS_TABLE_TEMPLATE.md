> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# RLS Table Template

**Classification:** Reference — Tier 1
**Owner:** `15-migrations/RLS_TABLE_TEMPLATE.md`
**Status:** Frozen at v1.0

---

## Purpose

This is the mandatory SQL template for every new tenant-scoped table. Copy this template into your `.up.sql` file and replace the placeholder names. Every table that stores tenant data MUST include all components of this template.

---

## Template

```sql
-- ============================================================
-- Template: Tenant-Scoped Table with RLS
-- Replace: {table_name}, {column_definitions}, {index_columns}
-- ============================================================

CREATE TABLE {table_name} (
    -- Primary key (always UUID)
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Tenant isolation (required on every tenant-scoped table)
    tenant_id   uuid        NOT NULL REFERENCES tenants(id),

    -- === Your columns here ===
    {column_definitions}

    -- Standard timestamps (always last)
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- ============================================================
-- Row Level Security (mandatory for every tenant-scoped table)
-- ============================================================

-- Enable RLS (reject any query that doesn't match a policy)
ALTER TABLE {table_name} ENABLE ROW LEVEL SECURITY;

-- FORCE: applies even to table owner / superuser within app sessions
ALTER TABLE {table_name} FORCE ROW LEVEL SECURITY;

-- Isolation policy: all queries see only current tenant's rows
CREATE POLICY tenant_isolation ON {table_name}
    USING (tenant_id = current_tenant_id());

-- ============================================================
-- Required Indexes
-- ============================================================

-- tenant_id index (essential for cross-tenant admin queries and RLS performance)
CREATE INDEX {table_name}_tenant_id ON {table_name} (tenant_id);

-- === Your domain indexes here ===
-- Examples:
-- CREATE INDEX {table_name}_status ON {table_name} (tenant_id, status);
-- CREATE INDEX {table_name}_customer ON {table_name} (tenant_id, customer_id);
```

---

## Worked Example

```sql
CREATE TABLE finance_invoice (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid        NOT NULL REFERENCES tenants(id),

    number          varchar(64) NOT NULL UNIQUE,
    customer_id     uuid        NOT NULL REFERENCES finance_customer(id),
    status          text        NOT NULL DEFAULT 'Draft'
                                CHECK (status IN ('Draft', 'Submitted', 'Approved', 'Paid', 'Cancelled')),
    total           numeric(20,4) NOT NULL DEFAULT 0,
    notes           text,
    custom_fields   jsonb       NOT NULL DEFAULT '{}',

    submitted_by    uuid,
    submitted_at    timestamptz,
    approved_by     uuid,
    approved_at     timestamptz,

    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE finance_invoice ENABLE ROW LEVEL SECURITY;
ALTER TABLE finance_invoice FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON finance_invoice
    USING (tenant_id = current_tenant_id());

CREATE INDEX finance_invoice_tenant_id  ON finance_invoice (tenant_id);
CREATE INDEX finance_invoice_customer   ON finance_invoice (tenant_id, customer_id);
CREATE INDEX finance_invoice_status     ON finance_invoice (tenant_id, status);
CREATE INDEX finance_invoice_number     ON finance_invoice (tenant_id, number);
CREATE INDEX finance_invoice_custom     ON finance_invoice USING GIN (custom_fields);
```

---

## Down Migration Template

```sql
DROP TABLE IF EXISTS {table_name};
```

No need to drop indexes or policies — they are automatically dropped with the table.

---

## Checklist Before Submitting

- [ ] `id uuid PRIMARY KEY DEFAULT gen_random_uuid()` present
- [ ] `tenant_id uuid NOT NULL REFERENCES tenants(id)` present
- [ ] `ENABLE ROW LEVEL SECURITY` present
- [ ] `FORCE ROW LEVEL SECURITY` present
- [ ] `tenant_isolation` policy present with `current_tenant_id()`
- [ ] Index on `tenant_id` present
- [ ] Currency columns use `numeric(20,4)` — not `float`, not `decimal(10,2)`
- [ ] Timestamp columns use `timestamptz` — not `timestamp` (timezone-unaware)
- [ ] `custom_fields jsonb NOT NULL DEFAULT '{}'` present (for system entities)
- [ ] `created_at` and `updated_at` present
- [ ] Down migration drops the table

---

## What NOT to Include

- Do NOT add `WHERE tenant_id = ?` to application code — RLS handles this.
- Do NOT use `SET LOCAL app.tenant_id` directly — use `set_tenant_context()` stored procedure.
- Do NOT use `numeric(20,6)` for money — always `numeric(20,4)`.
- Do NOT use `float` or `double precision` for money — ever.
- Do NOT skip `FORCE ROW LEVEL SECURITY` — without it, table owner bypasses RLS.

---

## References

- [`04-multitenancy/RLS_SPEC.md`](../04-multitenancy/RLS_SPEC.md) — Full RLS specification
- [`15-migrations/MIGRATION_GUIDE.md`](MIGRATION_GUIDE.md) — Migration governance
- [`15-migrations/MIGRATION_CHECKLIST.md`](MIGRATION_CHECKLIST.md) — Pre-merge checklist
