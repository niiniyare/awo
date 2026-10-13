# Chapter 49 — End-to-End Examples

> **Volume:** X — Reference
> **Phase:** 4
> **Audience:** All Engineers
> **Prerequisites:** All previous chapters

---

## Table of Contents

- [49.1 Example: Invoice Approval Workflow](#491-example-invoice-approval-workflow)
- [49.2 Example: New Tenant Onboarding](#492-example-new-tenant-onboarding)
- [49.3 Example: Offline Invoice Creation](#493-example-offline-invoice-creation)
- [49.4 Example: Surface Definition Authoring](#494-example-surface-definition-authoring)
- [49.5 Example: Adding a New Permission](#495-example-adding-a-new-permission)

---

## 49.1 Example: Invoice Approval Workflow

This example traces the complete path of an invoice from creation through approval by a different user, showing every system that participates.

### Step 1: Clerk creates a draft invoice

**Web client → API:**
```http
POST /api/v1/invoices
Authorization: Bearer <clerk_jwt>
X-Tenant-ID: tenant-acme
Idempotency-Key: 550e8400-e29b-41d4-a716-446655440000

{
  "vendor_id": "vendor-001",
  "line_items": [
    { "description": "Consulting", "quantity": 1, "unit_price": 150000, "currency_code": "KES" }
  ]
}
```

**API handler:**
1. Validates request against OpenAPI spec
2. Sets `SET LOCAL app.tenant_id = 'tenant-acme'` — RLS is now active
3. Checks FGA: `user:clerk-1 can create invoice:*`? Yes
4. Inserts invoice with `status = "draft"`, `created_by = "user:clerk-1"`
5. Writes audit log entry: `{ action: "invoice.created", actor: "clerk-1", resource: "invoice:inv-001" }`
6. Publishes SSE invalidation to `surface:invoice_list:tenant:tenant-acme`
7. Returns `201 Created` with invoice body

### Step 2: Clerk submits invoice for approval

```http
POST /api/v1/invoices/inv-001/submit
Authorization: Bearer <clerk_jwt>
```

1. FGA: `user:clerk-1 can submit invoice:inv-001`? Yes (clerk owns it, status is draft)
2. Updates `status = "pending_approval"`
3. Starts Temporal workflow: `InvoiceApprovalNotificationWorkflow`
   - Workflow finds all users with `approver` relation to the invoice's department
   - Sends notification (email + in-app SSE)
4. Returns `200 OK`

### Step 3: Approver receives notification, opens invoice

**SSE notification received by approver's client:**
```json
{
  "type": "notification",
  "payload": {
    "title_key": "notification.invoice.pending_approval",
    "action": { "type": "navigate", "surface_id": "invoice_detail", "params": { "invoice_id": "inv-001" } }
  }
}
```

**Approver taps notification → client loads surface:**
```http
GET /api/v1/surfaces/invoice_detail?invoice_id=inv-001
Authorization: Bearer <approver_jwt>
```

Surface compiler:
1. Loads `invoice_detail` surface definition
2. Compiles for role `approver`:
   - `amount` field: visible (approver can see amounts)
   - `approve_action`: visible (approver can approve)
   - `delete_action`: hidden (approver cannot delete)
3. Returns compiled surface

### Step 4: Approver approves the invoice

```http
POST /api/v1/invoices/inv-001/approve
Authorization: Bearer <approver_jwt>
Idempotency-Key: 660e8400-e29b-41d4-a716-446655440001
```

1. FGA: `user:approver-1 can approve invoice:inv-001`? Yes
2. Business rule: `status == "pending_approval"`? Yes
3. Updates `status = "approved"`, `approved_by = "user:approver-1"`, `approved_at = now()`
4. Writes audit log entry
5. Publishes SSE invalidation → both clerk's and approver's `invoice_list` surfaces update live

**Clerk's web app — SSE patch received:**
```json
{
  "version": 42,
  "base_version": 41,
  "operations": [
    {
      "op": "set_props",
      "path": "$.components[?(@.id=='inv-001-row')].children[?(@.id=='status-badge')]",
      "props": {
        "status": "approved",
        "label_key": "invoice.status.approved"
      }
    }
  ]
}
```

Clerk's invoice list updates in real time, without a page reload.

---

## 49.2 Example: New Tenant Onboarding

This example shows the complete path when a new organization signs up for AwoERP.

### Step 1: Tenant provisioning request

```http
POST /api/v1/tenants
Authorization: Bearer <platform_admin_jwt>

{
  "name": "Sunrise Logistics Ltd",
  "subdomain": "sunrise-logistics",
  "plan": "business",
  "admin_email": "admin@sunrise.co.ke",
  "locale": "en-KE",
  "timezone": "Africa/Nairobi"
}
```

### Step 2: TenantProvisioningWorkflow (Temporal)

The API returns `202 Accepted` immediately and starts the workflow:

```go
// Activities run in order; each is retryable
Activity: CreateTenantRecord(params)
    // Inserts into tenants table
    // Creates tenant schema in PostgreSQL
    // Applies RLS policies to all tables for new tenant

Activity: SeedDefaultData(tenantID)
    // Creates default currency (KES), fiscal year, chart of accounts
    // Creates default permission groups (admin, approver, clerk, viewer)
    // Publishes default surfaces to tenant's surface_definitions

Activity: CreateAdminUser(tenantID, adminEmail)
    // Creates user record
    // Sets FGA tuple: user:{user_id} is admin of tenant:{tenant_id}
    // Sends onboarding email with login link

Activity: VerifyProvisioning(tenantID)
    // Runs smoke test: can a simulated admin user load the dashboard surface?
    // Checks all expected tables exist and RLS policies are active
    // Marks tenant as "active" in tenants table
```

### Step 3: Admin configures tenant

Admin receives the onboarding email, sets a password, and logs in. The onboarding surface guides them through:
1. Uploading their logo (stored in object storage, served via CDN)
2. Setting their fiscal year
3. Inviting their first users

Each step is an SDUI surface; no custom code is required for standard onboarding.

---

## 49.3 Example: Offline Invoice Creation

This example traces an invoice created while the user's device has no network connection.

### Step 1: Device goes offline

The mobile client detects loss of connectivity (network reachability API). It:
1. Transitions sync state machine: `online` → `offline_writing`
2. Displays a banner: "Working offline — changes will sync when you reconnect"

### Step 2: User creates an invoice offline

The user fills in an invoice form and taps "Save". The surface action handler:
1. Generates a local UUID for the invoice: `inv-local-550e8400`
2. Writes the invoice to the local SQLite database (offline store)
3. Appends to the offline queue:
```json
{
  "queue_id": "q-001",
  "action_id": "invoice.create",
  "payload": {
    "local_id": "inv-local-550e8400",
    "vendor_id": "vendor-001",
    "line_items": [...]
  },
  "created_at": "2024-03-15T10:00:00Z",
  "retry_count": 0
}
```
4. Shows the invoice in the list with a "⏳ Pending sync" indicator

### Step 3: Device reconnects

Network becomes available. The sync service:
1. Transitions: `offline_writing` → `syncing`
2. Replays the offline queue in order:
```http
POST /api/v1/invoices
X-Offline-Queue-ID: q-001
X-Local-ID: inv-local-550e8400
Idempotency-Key: q-001

{ ...invoice payload... }
```
3. Server responds with the real server-side ID: `inv-server-001`
4. Client updates local SQLite: replaces `inv-local-550e8400` with `inv-server-001`
5. Queue entry is marked `replayed`

### Step 4: Conflict handling

If the server rejects the queued action (e.g., the vendor was deleted while offline):
```json
{ "error": { "code": "vendor_not_found", "message": "Vendor no longer exists" } }
```

The client:
1. Marks the queue entry as `failed`
2. Surfaces a conflict resolution screen to the user
3. Offers options: "Update vendor" or "Discard draft"

---

## 49.4 Example: Surface Definition Authoring

This example shows the full lifecycle of adding a new field to the invoice list surface.

### Step 1: Add the field to the surface definition

```json
// surfaces/invoice-list.json (excerpt)
{
  "type": "data.table",
  "id": "invoice-table",
  "columns": [
    { "id": "col-id", "label_key": "invoice.field.id", "value_binding": "$row.id" },
    { "id": "col-vendor", "label_key": "invoice.field.vendor", "value_binding": "$row.vendor_name" },
    { "id": "col-amount", "label_key": "invoice.field.amount", "value_binding": "$row.amount",
      "display": { "type": "currency", "currency_code_binding": "$row.currency_code" },
      "visibility": { "roles": ["approver", "admin"] }
    },
    // New field being added:
    { "id": "col-due-date", "label_key": "invoice.field.due_date", "value_binding": "$row.due_date",
      "display": { "type": "date" }
    }
  ]
}
```

### Step 2: Add translation keys

```json
// translations/en-KE/invoice.json
{
  "invoice.field.due_date": "Due Date"
}

// translations/sw-KE/invoice.json
{
  "invoice.field.due_date": "Tarehe ya Malipo"
}
// ... repeat for all 8 supported locales
```

### Step 3: Run the linter locally

```bash
awoerp-lint surfaces/invoice-list.json
# ✓ All label_key values found in translations
# ✓ All action_id references resolved
# ✓ Accessibility labels present on all interactive components
# ✓ No binding loops detected
```

### Step 4: Preview locally

```bash
awoerp-preview --surface invoice_list --role approver --locale sw-KE
# Opens http://localhost:3001
# "Tarehe ya Malipo" column visible (approver sees all columns)
```

### Step 5: Open PR, CI runs

CI runs:
- Surface lint
- Snapshot test update (golden file updated with new column)
- Role permutation tests (compile for all roles; verify output)
- Visual regression screenshots

### Step 6: Publish to staging, then production

After PR merges:
1. Auto-deploy to staging
2. Staging smoke test: load invoice_list surface as each role
3. Manual production deploy: two-engineer approval gate (Chapter 45)

---

## 49.5 Example: Adding a New Permission

This example shows how to add a permission for a new capability: the ability for a `supervisor` role to re-open closed invoices.

### Step 1: Define the new FGA relation

```fga
// authorization/model.fga
type invoice {
  relations {
    define creator: [user]
    define approver: [user]
    define can_approve: approver
    // New relation:
    define can_reopen: [user] or supervisor from tenant
  }
}
```

### Step 2: Write the FGA unit test

```go
func TestInvoiceReopenPermission(t *testing.T) {
    store := newTestFGAStore(t)

    // Setup: alice is a supervisor
    store.WriteTuples([]Tuple{
        {User: "user:alice", Relation: "supervisor", Object: "tenant:acme"},
    })

    // Alice can reopen
    assertAllowed(t, store, "user:alice", "can_reopen", "invoice:inv-001")

    // Bob (not a supervisor) cannot
    store.WriteTuples([]Tuple{
        {User: "user:bob", Relation: "clerk", Object: "tenant:acme"},
    })
    assertDenied(t, store, "user:bob", "can_reopen", "invoice:inv-001")
}
```

### Step 3: Add the API authorization check

```go
// In the invoice handler
func (h *InvoiceHandler) ReopenInvoice(ctx context.Context, req *ReopenInvoiceRequest) (*Invoice, error) {
    userID := auth.UserFromContext(ctx)
    invoiceID := req.InvoiceID

    allowed, err := h.fga.Check(ctx, &fgaClient.ClientCheckRequest{
        User:     "user:" + userID,
        Relation: "can_reopen",
        Object:   "invoice:" + invoiceID,
    })
    if err != nil || !allowed.GetAllowed() {
        return nil, ErrForbidden
    }
    // ... execute reopen
}
```

### Step 4: Add surface visibility rule

In the invoice detail surface, the reopen action is gated on the `can_reopen` FGA relation:

```json
{
  "type": "action.button",
  "id": "reopen-action",
  "label_key": "invoice.action.reopen",
  "action_id": "invoice.reopen",
  "visibility": {
    "fga_check": {
      "relation": "can_reopen",
      "object_binding": "$params.invoice_id"
    }
  }
}
```

The surface compiler evaluates the FGA check for the current user and includes or excludes the action button accordingly. No client-side role inspection required.

### Step 5: Add the integration test

```go
func TestInvoiceReopen_SupervisorAllowed(t *testing.T) {
    // Create tenant, create supervisor user, create closed invoice
    // Attempt reopen as supervisor → 200 OK
    // Attempt reopen as clerk → 403 Forbidden
    // Verify audit log entry written
}
```

---

*End of Chapter 49*

**Previous:** [Chapter 48 — Reference Architecture](./48-reference-architecture.md)
**Next:** [Chapter 50 — SDK](./50-sdk.md)
