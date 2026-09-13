> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Quick Start"
id: intro-005
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors]
since: "1.0"
normative-level: informative
related:
  - "[Philosophy](philosophy.md)"
  - "[Module Developer Guide](../16-module-dev-guide/README.md)"
  - "[Getting Started](../16-module-dev-guide/01-getting-started.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Quick Start

**INTRO-005 | Status: Accepted | Stability: Stable**

Get from zero to a running entity definition in 15 minutes.

---

## Prerequisites

- Go 1.22+
- Docker (for local PostgreSQL + Redis + Temporal)
- Access to the `erp` repository

---

## 1. Start Local Infrastructure

```bash
docker compose up -d postgres redis temporal
```

The `docker-compose.yml` at the repo root starts:
- PostgreSQL 16 (port 5432) with PgBouncer (port 5433)
- Redis 7 (port 6379)
- Temporal server + UI (port 7233 / 8080)

---

## 2. Run Migrations

```bash
go run ./cmd/migrate up
```

This applies all `.up.sql` files in `db/migration/` in timestamp order.

---

## 3. Start the Server

```bash
go run ./cmd/server
```

The server starts at `http://localhost:8080`. Temporal worker starts concurrently.

---

## 4. Create Your First Entity

Create the file `internal/core/crm/entity_contact.go`:

```go
package crm

import (
    "awo.so/awo/def"
)

var ContactDefinition = def.SystemDefinition{
    Name:        "crm_contact",
    Module:      "crm",
    Label:       "Contact",
    LabelPlural: "Contacts",
    Fields: []def.FieldDef{
        {Name: "name",  Type: def.FieldData, Required: true, Searchable: true},
        {Name: "email", Type: def.FieldData,
            Validators: []def.FieldValidator{def.EmailValidator{}}},
        {Name: "phone", Type: def.FieldData},
        {Name: "status", Type: def.FieldSelect,
            Options: []string{"Lead", "Prospect", "Customer"}, Default: "Lead"},
    },
    Permissions: def.PermissionSet{
        Create: []string{"role:crm.sales_rep", "role:tenant.admin"},
        Read:   []string{"role:crm.sales_rep", "role:tenant.admin"},
        Write:  []string{"role:crm.sales_rep", "role:tenant.admin"},
        Delete: []string{"role:tenant.admin"},
    },
}

func init() {
    def.Register(&ContactDefinition)
}
```

---

## 5. Write the Migration

Create `db/migration/20241215120000_create_crm_contact.up.sql`:

```sql
CREATE TABLE crm_contact (
    id           uuid PRIMARY KEY,
    tenant_id    uuid NOT NULL REFERENCES tenants(id),
    name         varchar(255) NOT NULL,
    email        varchar(255),
    phone        varchar(50),
    status       varchar(50) NOT NULL DEFAULT 'Lead'
                 CHECK (status IN ('Lead', 'Prospect', 'Customer')),
    custom_fields jsonb DEFAULT '{}',
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz
);

ALTER TABLE crm_contact ENABLE ROW LEVEL SECURITY;
ALTER TABLE crm_contact FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON crm_contact
    USING (tenant_id = current_tenant_id());

CREATE INDEX ON crm_contact(tenant_id);
CREATE INDEX ON crm_contact USING gin(name gin_trgm_ops);  -- Searchable field
CREATE INDEX ON crm_contact USING gin(custom_fields);
```

Create `db/migration/20241215120000_create_crm_contact.down.sql`:

```sql
DROP TABLE IF EXISTS crm_contact;
```

---

## 6. Run the New Migration

```bash
go run ./cmd/migrate up
```

---

## 7. Import the Module

In `cmd/server/main.go` (or your module registration file), add a blank import:

```go
import (
    _ "awo.so/internal/core/crm"  // triggers init() → registers ContactDefinition
)
```

---

## 8. Restart and Test

```bash
go run ./cmd/server
```

The framework auto-generates these routes for your new entity:

```
GET    /api/v1/entities/crm_contact         → List contacts
GET    /api/v1/entities/crm_contact/:id     → Get contact
POST   /api/v1/entities/crm_contact         → Create contact
PATCH  /api/v1/entities/crm_contact/:id     → Update contact
DELETE /api/v1/entities/crm_contact/:id     → Delete contact
```

Test with curl (after obtaining a session token):

```bash
curl -X POST http://localhost:8080/api/v1/entities/crm_contact \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $SESSION_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -d '{"name": "Amina Ochieng", "email": "amina@example.com", "status": "Lead"}'
```

Response:

```json
{
  "data": {
    "id": "01933b2c-a1b2-7000-c3d4-e5f6a7b8c9d0",
    "name": "Amina Ochieng",
    "email": "amina@example.com",
    "phone": null,
    "status": "Lead",
    "created_at": "2024-12-15T12:00:00+03:00",
    "updated_at": "2024-12-15T12:00:00+03:00"
  }
}
```

---

## 9. View the Auto-Generated UI

The SDUI layer generates a list view and create form automatically. Access at:

```
http://localhost:8080/crm/contact
```

---

## What's Next

| Topic | Document |
|---|---|
| Add hooks for business logic | [Add Hooks](../16-module-dev-guide/05-add-hooks.md) |
| Add row-level filtering | [Add Policies](../16-module-dev-guide/06-add-policies.md) |
| Add custom actions | [Add Actions](../16-module-dev-guide/07-add-actions.md) |
| Add async workflows | [Add Workflows](../16-module-dev-guide/08-add-workflows.md) |
| Customize the UI | [Add SDUI](../16-module-dev-guide/09-add-sdui.md) |
| Complete module checklist | [Module Checklist](../16-module-dev-guide/16-module-checklist.md) |
