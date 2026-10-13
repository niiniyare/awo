# Chapter 27 — Authorization

> **Volume:** VI — Platform APIs & Configuration
> **Phase:** 3
> **Audience:** Backend Engineers, Platform Engineers, Security Engineers, Product Engineers
> **Prerequisites:** Chapters 03, 25, 26

---

## Table of Contents

- [27.1 Authorization Model Overview](#271-authorization-model-overview)
- [27.2 Role-Based Access Control (RBAC)](#272-role-based-access-control-rbac)
- [27.3 OpenFGA Integration](#273-openfga-integration)
- [27.4 Permission Evaluation in the Backend](#274-permission-evaluation-in-the-backend)
- [27.5 Surface-Level Authorization](#275-surface-level-authorization)
- [27.6 Row-Level Security (PostgreSQL RLS)](#276-row-level-security-postgresql-rls)
- [27.7 Column-Level Authorization](#277-column-level-authorization)
- [27.8 Action Authorization](#278-action-authorization)
- [27.9 Tenant Admin Authorization](#279-tenant-admin-authorization)
- [27.10 Authorization Caching](#2710-authorization-caching)

---

## 27.1 Authorization Model Overview

AwoERP uses a layered authorization model:

```
Layer 1: Surface-level (can the user access this surface at all?)
    ↓
Layer 2: Action-level (can the user perform this specific action?)
    ↓
Layer 3: Resource-level (can the user access this specific record?)
    ↓
Layer 4: Field-level (can the user see/modify this specific field?)
    ↓
Layer 5: PostgreSQL RLS (database enforces row visibility)
```

Each layer operates independently. The surface compilation pipeline enforces Layers 1, 2, 3, and 4 by producing definitions scoped to what the user can see and do. Layer 5 is the backstop — it enforces data isolation even if application-level checks were somehow bypassed.

**OpenFGA** implements the relationship-based authorization model for Layers 1–4. **PostgreSQL RLS** implements Layer 5.

---

## 27.2 Role-Based Access Control (RBAC)

### 27.2.1 Built-in Roles

AwoERP ships with a set of built-in roles that cover common ERP use cases. Tenant admins can create custom roles that combine permissions from different built-in role templates.

| Role ID | Description |
|---------|-------------|
| `superadmin` | Platform-level administrator; cross-tenant access |
| `tenant_admin` | Full control within a tenant |
| `finance_manager` | Full finance module access including approvals |
| `finance_staff` | Finance data entry; no approval authority |
| `invoice_approver` | Invoice approval workflow only |
| `procurement_manager` | Procurement module manager |
| `procurement_staff` | Purchase requisition creation |
| `hr_manager` | HR module full access |
| `hr_staff` | Employee self-service |
| `report_viewer` | Read-only access to reports and dashboards |

### 27.2.2 Permission Taxonomy

Permissions follow the pattern `{resource}.{action}`:

```
invoice.read
invoice.create
invoice.update
invoice.delete
invoice.approve
invoice.reject
invoice.bulk_approve

vendor.read
vendor.create
vendor.update
vendor.delete

budget.read
budget.update
budget.close

report.{report_id}.read

admin.roles.read
admin.roles.update
admin.users.read
admin.users.update
```

---

## 27.3 OpenFGA Integration

### 27.3.1 Authorization Model

AwoERP uses OpenFGA's Zanzibar-style authorization model. The model defines object types, their relations, and how those relations are evaluated:

```python
model
  schema 1.1

type user

type tenant
  relations
    define admin: [user]
    define member: [user] or admin

type role
  relations
    define assignee: [user]
    define member: assignee

type permission
  relations
    define holder: [role#member, user]

type surface
  relations
    define viewer: [user, role#member]

type invoice
  relations
    define owner: [user]
    define reader: owner or viewer from tenant
    define approver: [user, role#member]
    define can_read: reader
    define can_approve: approver
```

### 27.3.2 Writing Tuples

Authorization relationships are stored as tuples in OpenFGA. When a tenant admin assigns a role:

```go
type FGAClient struct {
    client *openfga.APIClient
    storeID string
    modelID string
}

func (f *FGAClient) AssignRole(ctx context.Context, userID, roleID, tenantID string) error {
    _, err := f.client.WriteTuples(ctx, f.storeID).Body(
        openfga.WriteRequest{
            Writes: &openfga.WriteRequestWrites{
                TupleKeys: []openfga.TupleKey{
                    {
                        User:     fmt.Sprintf("user:%s", userID),
                        Relation: "assignee",
                        Object:   fmt.Sprintf("role:%s_%s", tenantID, roleID),
                    },
                },
            },
            AuthorizationModelId: &f.modelID,
        },
    ).Execute()
    return err
}

func (f *FGAClient) RevokeRole(ctx context.Context, userID, roleID, tenantID string) error {
    _, err := f.client.WriteTuples(ctx, f.storeID).Body(
        openfga.WriteRequest{
            Deletes: &openfga.WriteRequestDeletes{
                TupleKeys: []openfga.TupleKeyWithoutCondition{
                    {
                        User:     fmt.Sprintf("user:%s", userID),
                        Relation: "assignee",
                        Object:   fmt.Sprintf("role:%s_%s", tenantID, roleID),
                    },
                },
            },
            AuthorizationModelId: &f.modelID,
        },
    ).Execute()
    return err
}
```

### 27.3.3 Checking Authorization

```go
func (f *FGAClient) Check(ctx context.Context, userID, relation, objectType, objectID string) (bool, error) {
    resp, err := f.client.Check(ctx, f.storeID).Body(
        openfga.CheckRequest{
            TupleKey: openfga.CheckRequestTupleKey{
                User:     fmt.Sprintf("user:%s", userID),
                Relation: relation,
                Object:   fmt.Sprintf("%s:%s", objectType, objectID),
            },
            AuthorizationModelId: &f.modelID,
        },
    ).Execute()
    if err != nil {
        return false, err
    }
    return resp.GetAllowed(), nil
}

// Batch check for multiple objects
func (f *FGAClient) BatchCheck(ctx context.Context,
    userID string,
    checks []AuthCheck) (map[string]bool, error) {
    // Uses OpenFGA batch check API for efficiency
    // Returns map[check_key]allowed
}
```

---

## 27.4 Permission Evaluation in the Backend

### 27.4.1 Middleware Chain

Every handler that accesses a resource goes through permission checks in this order:

```go
// Route setup with authorization middleware
r.With(
    AuthMiddleware(jwtConfig),                       // Validate token
    TenantMiddleware(tenantStore),                   // Resolve tenant
    PermissionMiddleware(fgaClient, "invoice.read"), // Check permission
).Get("/api/invoices/{id}", handlers.GetInvoice)
```

### 27.4.2 Resource-Level Check

For resource-specific checks (can user read *this* invoice, not just any invoice):

```go
func (h *InvoiceHandler) GetInvoice(w http.ResponseWriter, r *http.Request) {
    invoiceID := chi.URLParam(r, "id")
    userID := middleware.UserID(r.Context())

    allowed, err := h.fga.Check(r.Context(), userID, "can_read", "invoice", invoiceID)
    if err != nil {
        writeError(w, 500, "internal_error", "Authorization check failed")
        return
    }
    if !allowed {
        writeError(w, 403, "forbidden", "You do not have permission to view this invoice")
        return
    }
    // ... proceed
}
```

### 27.4.3 Listing with Filtered Authorization

For list endpoints, filtering by authorization is done at the database level via RLS rather than post-filtering in Go (which would break pagination):

```sql
-- RLS policy on invoices table
CREATE POLICY invoice_access ON invoices
    FOR SELECT
    USING (
        tenant_id = current_setting('app.tenant_id')::uuid
        AND (
            -- User is directly the owner
            owner_id = current_setting('app.user_id')::uuid
            -- Or user has invoice.read permission via role
            OR EXISTS (
                SELECT 1 FROM user_role_permissions
                WHERE user_id = current_setting('app.user_id')::uuid
                  AND permission = 'invoice.read'
                  AND tenant_id = current_setting('app.tenant_id')::uuid
            )
        )
    );
```

---

## 27.5 Surface-Level Authorization

### 27.5.1 Compilation-Time Filtering

When the SDUI compiler builds a surface definition for a user, it evaluates which components, actions, and navigation items the user is authorized to see. Components with `requires_permission` declarations are removed from the compiled definition if the user lacks the required permission.

```json
{
  "type": "button",
  "label": "Approve Invoice",
  "on_click_action_id": "action_approve",
  "requires_permission": "invoice.approve"
}
```

If the requesting user does not have `invoice.approve`, this button is not present in the compiled surface definition sent to the client. There is no client-side permission hiding — the component simply does not exist.

### 27.5.2 Navigation Filtering

Navigation item visibility is also compiled:

```json
{
  "type": "nav.sidebar",
  "items": [
    {
      "label": "Invoices",
      "surface_id": "invoice_list",
      "requires_permission": "invoice.read"
    },
    {
      "label": "Admin",
      "surface_id": "admin_panel",
      "requires_permission": "admin.access"
    }
  ]
}
```

The compiled navigation definition sent to a `finance_staff` user omits the "Admin" item.

### 27.5.3 403 on Runtime Access Attempt

Even if a user somehow attempts to fetch a surface they lack permission to (via direct URL navigation or a crafted request), the surface definition endpoint returns 403 before compilation begins:

```go
func (h *SurfaceHandler) GetSurface(w http.ResponseWriter, r *http.Request) {
    surfaceID := chi.URLParam(r, "surface_id")
    userID := middleware.UserID(r.Context())

    // Check surface-level access
    allowed, err := h.fga.Check(r.Context(), userID, "viewer", "surface", surfaceID)
    if !allowed {
        writeError(w, 403, "forbidden", "Surface not available")
        return
    }
    // ... compile and return definition
}
```

---

## 27.6 Row-Level Security (PostgreSQL RLS)

### 27.6.1 RLS Setup

RLS is enabled on all tenant data tables. The Go application sets session-level settings before executing any query:

```go
func (s *Store) withRLSContext(ctx context.Context, tenantID, userID string,
    fn func(*sql.Tx) error) error {

    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // Set session variables for RLS policies
    if _, err := tx.ExecContext(ctx, "SET LOCAL app.tenant_id = $1", tenantID); err != nil {
        return fmt.Errorf("set tenant context: %w", err)
    }
    if _, err := tx.ExecContext(ctx, "SET LOCAL app.user_id = $1", userID); err != nil {
        return fmt.Errorf("set user context: %w", err)
    }

    if err := fn(tx); err != nil {
        return err
    }

    return tx.Commit()
}
```

The PostgreSQL application role used by AwoERP is not a superuser and does not bypass RLS. Bypassing RLS requires a separate `rls_bypass` role that is granted only to migration scripts and DBA sessions.

### 27.6.2 Cross-Tenant Query Prevention

RLS policies on every table include a `tenant_id = current_setting('app.tenant_id')` predicate. This means:

- Joining across tenants is impossible — the policy filters to the current tenant before the join
- Even a SQL injection attack that escaped parameterization cannot access another tenant's data
- Aggregation functions (COUNT, SUM) operate only on the current tenant's visible rows

### 27.6.3 Soft Delete and RLS

Soft-deleted records have `deleted_at IS NOT NULL`. RLS policies on most tables exclude soft-deleted records from standard read policies:

```sql
CREATE POLICY invoices_select ON invoices
    FOR SELECT
    USING (
        tenant_id = current_setting('app.tenant_id')::uuid
        AND deleted_at IS NULL
    );
```

A separate policy for admins (using `app.include_deleted = 'true'`) allows audit access to deleted records.

---

## 27.7 Column-Level Authorization

Certain fields are sensitive and require an additional permission check before being included in responses. The backend uses field-level permission declarations in the resource mapping layer:

```go
type FieldPolicy struct {
    Field      string
    Permission string
}

var invoiceFieldPolicies = []FieldPolicy{
    {Field: "vendor_bank_account", Permission: "invoice.view_banking"},
    {Field: "internal_notes",      Permission: "invoice.view_notes"},
    {Field: "cost_center",         Permission: "finance.view_cost_center"},
}

func mapInvoiceToResponse(ctx context.Context, fga FGAClient, invoice Invoice) InvoiceResponse {
    resp := InvoiceResponse{
        ID:            invoice.ID,
        InvoiceNumber: invoice.InvoiceNumber,
        Amount:        invoice.Amount,
        Status:        invoice.Status,
    }

    userID := middleware.UserID(ctx)
    for _, policy := range invoiceFieldPolicies {
        allowed, _ := fga.CheckPermission(ctx, userID, policy.Permission)
        if allowed {
            switch policy.Field {
            case "vendor_bank_account":
                resp.VendorBankAccount = invoice.VendorBankAccount
            case "internal_notes":
                resp.InternalNotes = invoice.InternalNotes
            case "cost_center":
                resp.CostCenter = invoice.CostCenter
            }
        }
    }
    return resp
}
```

Fields the user lacks permission to see are omitted from the response (not nulled — omitted entirely). The SDUI rendering engine treats missing fields as blank, per Chapter 24.

---

## 27.8 Action Authorization

The action system (Chapter 20) declares `requires_permission` on action definitions. The compiler removes unauthorized actions, and the backend enforces the same check at execution time:

```go
// On POST /api/invoices/{id}/approve
func (h *InvoiceHandler) ApproveInvoice(w http.ResponseWriter, r *http.Request) {
    invoiceID := chi.URLParam(r, "id")
    userID := middleware.UserID(r.Context())

    // Check action permission
    allowed, err := h.fga.Check(r.Context(), userID, "can_approve", "invoice", invoiceID)
    if !allowed {
        writeError(w, 403, "forbidden", "You do not have permission to approve this invoice")
        return
    }

    // Validate business rules (e.g., segregation of duties)
    invoice, err := h.store.GetInvoice(r.Context(), invoiceID)
    if invoice.CreatedBy == userID {
        writeError(w, 422, "validation_failed", "Cannot approve an invoice you created")
        return
    }

    // ... proceed with approval
}
```

Backend enforcement is not optional. The surface definition may not contain the approval button for unauthorized users, but the API enforces the permission regardless.

---

## 27.9 Tenant Admin Authorization

### 27.9.1 Tenant-Scoped Superuser

The `tenant_admin` role grants full control within a tenant but no cross-tenant access. Platform superadmin access is a separate role granted only to Anthropic operations staff and is not tenant-assignable.

### 27.9.2 Role Management Surface

Tenant admins manage roles through the `admin_roles` surface. Changes made in this surface write OpenFGA tuples directly:

```go
func (h *AdminHandler) AssignRole(w http.ResponseWriter, r *http.Request) {
    tenantID := middleware.TenantID(r.Context())
    adminUserID := middleware.UserID(r.Context())

    // Confirm the admin can manage roles
    allowed, _ := h.fga.CheckPermission(r.Context(), adminUserID, "admin.roles.update")
    if !allowed {
        writeError(w, 403, "forbidden", "Insufficient permissions")
        return
    }

    var body struct {
        UserID string `json:"user_id"`
        RoleID string `json:"role_id"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        writeError(w, 400, "bad_request", "Invalid request body")
        return
    }

    if err := h.fga.AssignRole(r.Context(), body.UserID, body.RoleID, tenantID); err != nil {
        writeError(w, 500, "internal_error", "Failed to assign role")
        return
    }

    // Invalidate authorization cache for affected user
    h.authCache.InvalidateUser(r.Context(), body.UserID, tenantID)

    // Emit permission_changed SSE event for active sessions
    h.eventBus.Publish(r.Context(), fmt.Sprintf("user:%s:tenant:%s", body.UserID, tenantID),
        SSEEvent{Type: "permission_changed"})

    writeJSON(w, 200, map[string]bool{"assigned": true})
}
```

---

## 27.10 Authorization Caching

### 27.10.1 Permission Cache

FGA checks are cached in Redis to avoid the latency of a network call on every request. The cache uses a short TTL so that permission changes propagate quickly.

```go
type CachedFGAClient struct {
    fga   FGAClient
    redis *redis.Client
    ttl   time.Duration // Default: 30 seconds
}

func (c *CachedFGAClient) Check(ctx context.Context,
    userID, relation, objectType, objectID string) (bool, error) {

    key := fmt.Sprintf("authz:%s:%s:%s:%s", userID, relation, objectType, objectID)

    cached, err := c.redis.Get(ctx, key).Result()
    if err == nil {
        return cached == "1", nil
    }

    allowed, err := c.fga.Check(ctx, userID, relation, objectType, objectID)
    if err != nil {
        return false, err
    }

    val := "0"
    if allowed {
        val = "1"
    }
    c.redis.Set(ctx, key, val, c.ttl)

    return allowed, nil
}

func (c *CachedFGAClient) InvalidateUser(ctx context.Context, userID, tenantID string) {
    pattern := fmt.Sprintf("authz:%s:*", userID)
    keys, _ := c.redis.Keys(ctx, pattern).Result()
    if len(keys) > 0 {
        c.redis.Del(ctx, keys...)
    }
}
```

### 27.10.2 Compiled Surface Caching

The compiled surface definition cache key (Chapter 8) includes the `role_set_hash` and `flag_set_hash`. When a user's roles change, their role set hash changes, which invalidates their cached surface definition. The next surface fetch produces a freshly compiled definition reflecting the new permissions.

This means permission changes take effect at most on the user's next navigation (next surface definition fetch). For active sessions, the `permission_changed` SSE event triggers an immediate re-fetch and reload.

---

*End of Chapter 27*

**Previous:** [Chapter 26 — Security Architecture](./26-security.md)
**Next:** [Chapter 28 — Multi-Tenancy](./28-multi-tenancy.md)
