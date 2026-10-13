# Chapter 28 — Multi-Tenancy

> **Volume:** VI — Platform APIs & Configuration
> **Phase:** 3
> **Audience:** Backend Engineers, Platform Engineers, Database Engineers
> **Prerequisites:** Chapters 03, 25, 26, 27

---

## Table of Contents

- [28.1 Tenancy Model](#281-tenancy-model)
- [28.2 Tenant Data Isolation](#282-tenant-data-isolation)
- [28.3 Tenant Resolution](#283-tenant-resolution)
- [28.4 Tenant Provisioning](#284-tenant-provisioning)
- [28.5 Tenant Configuration](#285-tenant-configuration)
- [28.6 Tenant-Scoped SDUI Compilation](#286-tenant-scoped-sdui-compilation)
- [28.7 Cross-Tenant Operations (Superadmin Only)](#287-cross-tenant-operations-superadmin-only)
- [28.8 Tenant Data Export and Deletion](#288-tenant-data-export-and-deletion)

---

## 28.1 Tenancy Model

AwoERP uses a **shared database, shared schema** multi-tenancy model with PostgreSQL Row Level Security as the isolation enforcement boundary.

**Why shared schema?** It provides the best operational efficiency — one schema to migrate, one set of indexes, one connection pool — while RLS provides security guarantees that are enforced at the database level, not the application level. This model scales to thousands of tenants with no schema management overhead.

**Tenant boundaries are absolute.** Data belonging to Tenant A is never visible to Tenant B, even to superadmin users (who access specific tenants explicitly, not all tenants simultaneously). The only data that crosses tenant boundaries is:
- Platform-level configuration (feature flag definitions, component schemas)
- Aggregated, anonymized analytics (no individual record data)

---

## 28.2 Tenant Data Isolation

### 28.2.1 Tenant ID Column

Every tenant-scoped table includes a `tenant_id` column of type `uuid`. This column is:
- NOT NULL
- Part of every index on the table (leading column, or included where appropriate)
- Covered by a RLS policy that restricts all queries to the current tenant

```sql
CREATE TABLE invoices (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    invoice_number TEXT NOT NULL,
    amount      NUMERIC(15, 2) NOT NULL,
    status      TEXT NOT NULL DEFAULT 'draft',
    vendor_id   UUID NOT NULL,
    created_by  UUID NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX idx_invoices_tenant ON invoices(tenant_id, status, created_at DESC);
CREATE INDEX idx_invoices_tenant_vendor ON invoices(tenant_id, vendor_id);
```

### 28.2.2 RLS Policies

RLS is enabled on all tenant-scoped tables:

```sql
-- Enable RLS
ALTER TABLE invoices ENABLE ROW LEVEL SECURITY;
ALTER TABLE invoices FORCE ROW LEVEL SECURITY; -- Applies even to table owner

-- SELECT policy
CREATE POLICY invoices_tenant_isolation_select ON invoices
    FOR SELECT
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- INSERT policy
CREATE POLICY invoices_tenant_isolation_insert ON invoices
    FOR INSERT
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- UPDATE policy
CREATE POLICY invoices_tenant_isolation_update ON invoices
    FOR UPDATE
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- DELETE policy
CREATE POLICY invoices_tenant_isolation_delete ON invoices
    FOR DELETE
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
```

The `true` parameter in `current_setting('app.tenant_id', true)` means the function returns NULL (rather than raising an error) if the setting is not set. The `= NULL` predicate in the USING clause evaluates to false, so no rows are visible if the tenant context is not set. This is a safe default.

### 28.2.3 Application Role

The PostgreSQL role used by the AwoERP application (`awoerp_app`) does not have `BYPASSRLS` privilege. Migrating schemas uses a separate `awoerp_migrations` role that also does not bypass RLS for data-touching migrations. Only `awoerp_dba` has `BYPASSRLS`, and it is never used by the application.

---

## 28.3 Tenant Resolution

### 28.3.1 From Token to Tenant

The tenant context is established on every request through the following chain:

```go
func TenantMiddleware(tenantStore TenantStore) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Primary: from JWT claim
            claims := middleware.Claims(r.Context())
            tenantID := claims.TenantID

            // Secondary: from X-Tenant-ID header (validated against token)
            if headerTenantID := r.Header.Get("X-Tenant-ID"); headerTenantID != "" {
                if headerTenantID != tenantID {
                    writeError(w, 403, "forbidden", "Tenant mismatch")
                    return
                }
            }

            // Resolve tenant config (cached)
            tenant, err := tenantStore.Get(r.Context(), tenantID)
            if err != nil || !tenant.Active {
                writeError(w, 403, "forbidden", "Tenant not found or inactive")
                return
            }

            ctx := context.WithValue(r.Context(), contextKeyTenantID, tenantID)
            ctx = context.WithValue(ctx, contextKeyTenant, tenant)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### 28.3.2 Database Context Injection

Before any database query, the tenant and user context is set in the database session:

```go
// Called at the start of every database transaction
func (s *Store) setSessionContext(ctx context.Context, tx *sql.Tx) error {
    tenantID := middleware.TenantID(ctx)
    userID := middleware.UserID(ctx)

    queries := []string{
        fmt.Sprintf("SET LOCAL app.tenant_id = '%s'", tenantID),
        fmt.Sprintf("SET LOCAL app.user_id = '%s'", userID),
    }

    for _, q := range queries {
        if _, err := tx.ExecContext(ctx, q); err != nil {
            return fmt.Errorf("set session context: %w", err)
        }
    }

    return nil
}
```

These are session-local settings (`SET LOCAL`) that reset when the transaction ends, preventing context leakage between requests in the same connection pool connection.

---

## 28.4 Tenant Provisioning

### 28.4.1 Provisioning Flow

New tenant provisioning is handled by the platform admin service, not by the main application. The flow:

```
1. Platform admin creates tenant record
2. Provisioning job runs (Temporal workflow):
   a. Create tenant row in tenants table
   b. Seed default roles and permission assignments
   c. Create OpenFGA tuples for default roles
   d. Create tenant admin user account
   e. Configure default feature flags
   f. Initialize tenant preferences (currency, locale, timezone)
   g. Send welcome email to tenant admin
3. Tenant status set to 'active'
```

### 28.4.2 Tenant Record

```sql
CREATE TABLE tenants (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL UNIQUE,     -- e.g. 'acme-ltd' → app/acme-ltd/
    status          TEXT NOT NULL DEFAULT 'provisioning',  -- provisioning | active | suspended | deleted
    plan            TEXT NOT NULL DEFAULT 'starter',
    currency        TEXT NOT NULL DEFAULT 'USD',
    locale          TEXT NOT NULL DEFAULT 'en',
    timezone        TEXT NOT NULL DEFAULT 'UTC',
    logo_url        TEXT,
    custom_domain   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    suspended_at    TIMESTAMPTZ,
    deleted_at      TIMESTAMPTZ
);
```

### 28.4.3 Tenant Subdomain vs Custom Domain

Tenants access AwoERP at:
- Default: `{slug}.awoerp.com`
- Custom domain: `app.{customer-domain}.com` (configured in tenant settings)

The web shell resolves the tenant from the hostname:

```go
func resolveTenantFromHost(host string) (string, error) {
    if strings.HasSuffix(host, ".awoerp.com") {
        slug := strings.TrimSuffix(host, ".awoerp.com")
        return slug, nil
    }
    // Custom domain — lookup in tenant_custom_domains table
    return tenantStore.GetByCustomDomain(host)
}
```

---

## 28.5 Tenant Configuration

### 28.5.1 Tenant Preferences

Tenant-wide preferences affect how AwoERP behaves for all users in the tenant:

```json
{
  "tenant_id": "ten-456",
  "currency": "KES",
  "currency_display": "KSh",
  "decimal_places": 2,
  "locale": "en-KE",
  "timezone": "Africa/Nairobi",
  "date_format": "DD/MM/YYYY",
  "fiscal_year_start": "01-01",
  "approval_threshold_amount": 500000,
  "require_mfa": true,
  "session_timeout_minutes": 480,
  "allowed_ip_ranges": ["196.201.0.0/16"]
}
```

### 28.5.2 Tenant Theme

Each tenant can customize the visual appearance of the platform:

```json
{
  "tenant_id": "ten-456",
  "theme": {
    "primary_color": "#1A5276",
    "secondary_color": "#D4AC0D",
    "logo_url": "https://cdn.awoerp.com/tenants/ten-456/logo.png",
    "favicon_url": "https://cdn.awoerp.com/tenants/ten-456/favicon.ico",
    "font_family": "Inter",
    "border_radius": "4px"
  }
}
```

Theme tokens override the platform's default design tokens for surfaces compiled for that tenant. The SDUI compiler injects tenant theme tokens into the surface definition's design token layer.

### 28.5.3 Tenant Settings Surface

Tenant admins manage these settings through the `tenant_settings` surface. Changes are written to the `tenant_config` table and cached in Redis with a 60-second TTL. The cache invalidation fires a `tenant_config_changed` event that causes the app shell to reload its tenant context.

---

## 28.6 Tenant-Scoped SDUI Compilation

### 28.6.1 Tenant in the Cache Key

The surface definition cache key (Chapter 8) includes the `tenant_id`:

```
cache_key = surface_id + tenant_id + role_set_hash + flag_set_hash + locale + platform + client_version + capability_hash
```

Two users in different tenants viewing the same surface ID get different compiled definitions, because:
- The tenant's feature flags may differ
- The tenant's custom field extensions may differ
- The tenant's theme tokens differ
- The tenant's locale and currency formatting differs

### 28.6.2 Tenant-Specific Field Extensions

Tenants can add custom fields to standard entities (Chapter 30). The compiler includes tenant-specific custom fields in the compiled surface definition for form surfaces:

```json
{
  "type": "form.section",
  "title": "Custom Fields",
  "fields": [
    {
      "type": "field.text",
      "id": "custom_po_reference",
      "label": "PO Reference",
      "extension": true,
      "tenant_id": "ten-456"
    }
  ]
}
```

---

## 28.7 Cross-Tenant Operations (Superadmin Only)

Platform superadmins can access any tenant's data for support purposes. Cross-tenant access is:
- Always explicit (an admin must declare which tenant they are operating as)
- Always audit-logged with the superadmin's identity and the impersonated tenant
- Restricted to read operations by default; write access requires additional elevation

```go
func (h *SuperAdminHandler) ImpersonateTenant(w http.ResponseWriter, r *http.Request) {
    adminUserID := middleware.UserID(r.Context())
    targetTenantID := r.URL.Query().Get("tenant_id")

    // Confirm superadmin role
    allowed, _ := h.fga.CheckPermission(r.Context(), adminUserID, "superadmin")
    if !allowed {
        writeError(w, 403, "forbidden", "Superadmin access required")
        return
    }

    // Issue a short-lived tenant-scoped token
    token, err := h.auth.IssueImpersonationToken(r.Context(), ImpersonationClaims{
        SuperAdminID: adminUserID,
        TenantID:     targetTenantID,
        ExpiresIn:    15 * time.Minute,
        ReadOnly:     true,
    })

    // Audit log the impersonation
    h.audit.Write(r.Context(), AuditEvent{
        Action:    "superadmin.impersonate_tenant",
        UserID:    adminUserID,
        TenantID:  targetTenantID,
        Result:    "success",
    })

    writeJSON(w, 200, map[string]string{"impersonation_token": token})
}
```

All API calls made with an impersonation token are audit-logged with both the superadmin ID and the target tenant ID.

---

## 28.8 Tenant Data Export and Deletion

### 28.8.1 Data Export

Tenants can request a full export of their data via the `tenant_settings` surface. The export:
1. Queues a Temporal workflow: `TenantDataExportWorkflow`
2. Exports all tenant tables using `COPY ... TO` with `tenant_id = $1` filter
3. Packages as a compressed archive
4. Uploads to secure blob storage with a 24-hour expiry link
5. Notifies the tenant admin by email and in-app notification

### 28.8.2 Tenant Deletion (GDPR / Contract End)

Tenant deletion is a multi-step process managed by the platform admin service:

```
1. Tenant status → 'deleting'
2. Revoke all active sessions (invalidate all user tokens)
3. Cancel all running Temporal workflows for tenant
4. Export data (creates final archive)
5. Delete data (run DELETE FROM each table WHERE tenant_id = $1)
6. Delete tenant record
7. Delete blob storage objects
8. Delete OpenFGA tuples for tenant
9. Tenant status → 'deleted' (or remove row)
10. Notify tenant admin of completion
```

Hard deletion is performed after a 30-day soft-delete period (status: `suspended`) during which the data can be restored if the deletion was in error.

---

*End of Chapter 28*

**Previous:** [Chapter 27 — Authorization](./27-authorization.md)
**Next:** [Chapter 29 — Feature Flags](./29-feature-flags.md)
