> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Add Migrations"
id: mdg-10
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Add SDUI](09-add-sdui.md)"
  - "[Testing](11-testing.md)"
  - "[Migrations](../14-operations/migrations.md)"
  - "[System Entities](../05-persistence/system-entities.md)"
  - "[RLS](../06-tenancy/rls.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Add Migrations

**MDG-10 | Module Developer Guide**

This document writes the SQL migration files for `crm_contact` and `crm_interaction`. Because these are custom entities (JSONB storage), the migration is simpler than for system entities.

---

## 1. Custom Entity — No Table Migration Needed

For custom entities (`StorageModel: def.StorageCustom`), the framework uses the shared `custom_entity_records` table:

```sql
-- Already exists (created by the framework's initial migration):
CREATE TABLE custom_entity_records (
    id           uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    uuid         NOT NULL REFERENCES tenants(id),
    entity_type  text         NOT NULL,
    data         jsonb        NOT NULL DEFAULT '{}',
    created_at   timestamptz  NOT NULL DEFAULT now(),
    updated_at   timestamptz  NOT NULL DEFAULT now(),
    created_by   uuid,
    updated_by   uuid
);
```

No migration is needed to "create" a custom entity type — the `entity_type` column distinguishes records. The framework writes `entity_type = 'crm_contact'` for contact records.

However, **migrations may still be needed** for:
- GIN indexes on specific JSONB fields (for `Searchable: true` fields)
- Unique constraints (emulated via unique index on JSONB expression)
- Seeding initial data

---

## 2. Custom Entity Migration — Indexes

For `crm_contact`, the `email` field is declared `Unique: true` and `full_name` is declared `Searchable: true`. These require migration-managed indexes:

```sql
-- db/migration/20241215143022_crm_contact_indexes.up.sql

-- GIN index on full_name for trigram search
CREATE INDEX CONCURRENTLY crm_contact_full_name_gin_idx
    ON custom_entity_records
    USING GIN ((data->>'full_name') gin_trgm_ops)
    WHERE entity_type = 'crm_contact';

-- GIN index on email for trigram search
CREATE INDEX CONCURRENTLY crm_contact_email_gin_idx
    ON custom_entity_records
    USING GIN ((data->>'email') gin_trgm_ops)
    WHERE entity_type = 'crm_contact';

-- Unique index on email per tenant (enforces Unique: true for custom entity)
CREATE UNIQUE INDEX CONCURRENTLY crm_contact_email_unique_idx
    ON custom_entity_records (tenant_id, (data->>'email'))
    WHERE entity_type = 'crm_contact';

-- Index on assigned_to for PolicyFunc filter performance
CREATE INDEX CONCURRENTLY crm_contact_assigned_to_idx
    ON custom_entity_records ((data->>'assigned_to'))
    WHERE entity_type = 'crm_contact';
```

```sql
-- db/migration/20241215143022_crm_contact_indexes.down.sql
DROP INDEX IF EXISTS crm_contact_full_name_gin_idx;
DROP INDEX IF EXISTS crm_contact_email_gin_idx;
DROP INDEX IF EXISTS crm_contact_email_unique_idx;
DROP INDEX IF EXISTS crm_contact_assigned_to_idx;
```

---

## 3. System Entity Migration

If `crm_contact` were a system entity, a full table migration would be required:

```sql
-- db/migration/20241215143022_create_crm_contact.up.sql
CREATE TABLE crm_contact (
    id                  uuid            PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           uuid            NOT NULL REFERENCES tenants(id),
    full_name           varchar(128)    NOT NULL,
    email               varchar(256)    NOT NULL,
    phone               varchar(32),
    status              varchar(32)     NOT NULL DEFAULT 'Lead'
                        CHECK (status IN ('Lead','Prospect','Active','Inactive','Lost')),
    source              varchar(32)     NOT NULL DEFAULT 'Other'
                        CHECK (source IN ('Referral','Website','Event','Cold Outreach','Social Media','Other')),
    assigned_to         uuid            REFERENCES users(id),
    notes               text,
    first_contact_date  date,
    custom_fields       jsonb           NOT NULL DEFAULT '{}',
    created_at          timestamptz     NOT NULL DEFAULT now(),
    updated_at          timestamptz     NOT NULL DEFAULT now(),
    created_by          uuid,
    updated_by          uuid
);

-- RLS (required for every tenant-scoped table)
ALTER TABLE crm_contact ENABLE ROW LEVEL SECURITY;
ALTER TABLE crm_contact FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON crm_contact
    USING (tenant_id = current_tenant_id());

-- Unique constraint
CREATE UNIQUE INDEX CONCURRENTLY crm_contact_email_unique_idx
    ON crm_contact (tenant_id, email);

-- GIN trigram index for search
CREATE INDEX CONCURRENTLY crm_contact_full_name_gin_idx
    ON crm_contact USING GIN (full_name gin_trgm_ops);

-- Performance indexes
CREATE INDEX crm_contact_tenant_status_idx ON crm_contact (tenant_id, status);
CREATE INDEX crm_contact_assigned_to_idx ON crm_contact (tenant_id, assigned_to);
CREATE INDEX crm_contact_custom_fields_idx ON crm_contact USING GIN (custom_fields);
```

```sql
-- db/migration/20241215143022_create_crm_contact.down.sql
DROP TABLE IF EXISTS crm_contact;
```

---

## 4. Migration Checklist

Before submitting a migration for review:

- [ ] Timestamp in filename is correct and unique (14 digits, `YYYYMMDDHHMMSS`)
- [ ] Both `.up.sql` and `.down.sql` present
- [ ] `.down.sql` cleanly reverses `.up.sql`
- [ ] All new tenant-scoped tables have `ENABLE ROW LEVEL SECURITY`, `FORCE ROW LEVEL SECURITY`, and the `tenant_isolation` policy
- [ ] All production indexes use `CREATE INDEX CONCURRENTLY`
- [ ] No `ALTER TABLE ... RENAME` on columns with existing data
- [ ] No `ALTER TABLE ... SET NOT NULL` without a default on a table with existing rows
- [ ] Migration tested in local environment (apply up, verify, apply down, verify)

---

## 5. Running the Migration

```bash
# Never auto-migrate — run the migration runner manually
./awo-migrate up

# Verify current version
./awo-migrate version
```

Do not embed migration execution in application startup. The migration runner (`cmd/migrate`) is a separate process run as a pre-deploy step.

---

## Next: [Testing →](11-testing.md)
