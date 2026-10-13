# Chapter 20 — Action System

> **Volume:** IV — Rendering Architecture
> **Phase:** 2
> **Audience:** Platform Engineers, Backend Engineers, Mobile Engineers, Web Engineers
> **Prerequisites:** Chapters 01–19

---

## Table of Contents

- [20.1 Action System Overview](#201-action-system-overview)
- [20.2 Action Definition Structure](#202-action-definition-structure)
- [20.3 Action Types](#203-action-types)
- [20.4 Action Chaining](#204-action-chaining)
- [20.5 Optimistic Updates](#205-optimistic-updates)
- [20.6 Action Context and Payload Binding](#206-action-context-and-payload-binding)
- [20.7 Error Handling in Actions](#207-error-handling-in-actions)
- [20.8 Action Permissions](#208-action-permissions)
- [20.9 Offline Action Queue](#209-offline-action-queue)
- [20.10 Action Telemetry](#2010-action-telemetry)

---

## 20.1 Action System Overview

An **action** is a unit of user-initiated work declared in the UI definition and executed by the rendering engine. Actions represent everything a user can *do* — submit a form, navigate to another surface, call a backend API, show a notification, set a state variable, open a modal, trigger a workflow signal.

The action system has three design goals:

**Declarative, not scripted.** Actions are data structures in the JSON surface definition — not client-side script. There is no JavaScript in a surface definition. All behavior is expressed as action node types with props, and the rendering engine's action executor provides the implementation.

**Composable through chaining.** Complex multi-step flows are expressed as ordered chains of individual action steps. Each step can depend on the result of the previous step. Error handling at each step redirects to an error chain.

**Auditable.** Every action execution produces a telemetry event. HTTP request actions produce a server-side audit log entry. The full audit trail of user actions within a session is recoverable for compliance purposes.

---

## 20.2 Action Definition Structure

Actions are defined in the surface definition's `actions` map, referenced by ID from component event handlers.

```json
{
  "actions": {
    "submit_invoice": {
      "id": "submit_invoice",
      "label": "Submit Invoice",
      "chain": [
        {
          "id": "validate_step",
          "type": "action.validate_form",
          "props": { "form_id": "invoice_form" }
        },
        {
          "id": "save_step",
          "type": "action.http_request",
          "props": {
            "method": "POST",
            "endpoint": "/api/invoices",
            "body": "$form.invoice_form"
          },
          "result_var": "saved_invoice"
        },
        {
          "id": "navigate_step",
          "type": "action.navigate",
          "props": {
            "route": {
              "surface": "invoice_detail",
              "params": { "invoice_id": "$action.saved_invoice.id" }
            }
          }
        },
        {
          "id": "toast_step",
          "type": "action.show_toast",
          "props": {
            "message": "Invoice $action.saved_invoice.invoice_number saved",
            "variant": "success"
          }
        }
      ]
    }
  }
}
```

### 20.2.1 ActionDefinition Schema

```typescript
interface ActionDefinition {
  id: string;                    // Unique within the surface
  label?: string;                // Human-readable label (for aria, audit logs)
  chain: ActionStep[];           // Ordered steps to execute
  debounce_ms?: number;          // Debounce repeated triggers (default: 0)
  throttle_ms?: number;          // Throttle trigger rate (default: 0)
  confirm?: ConfirmSpec;         // Require user confirmation before executing
  loading_state_var?: string;    // State variable to set to `true` while executing
}

interface ActionStep {
  id: string;                    // Unique within the chain
  type: ActionType;              // Step type (see Section 20.3)
  props: Record<string, unknown>; // Step-specific props (may include bindings)
  result_var?: string;           // Store step result in $action.<result_var>
  error_chain?: ActionStep[];    // Steps to execute on failure
  skip_if?: Binding;             // Skip this step if binding evaluates to true
}
```

### 20.2.2 Confirmation Before Execution

When an action declares a `confirm` spec, the rendering engine shows a platform-appropriate confirmation dialog before executing the chain. The user must confirm to proceed; dismissing cancels execution.

```json
{
  "id": "delete_invoice",
  "label": "Delete Invoice",
  "confirm": {
    "title": "Delete Invoice?",
    "message": "This action cannot be undone. The invoice will be permanently deleted.",
    "confirm_label": "Delete",
    "cancel_label": "Cancel",
    "variant": "destructive"
  },
  "chain": [
    {
      "type": "action.http_request",
      "props": {
        "method": "DELETE",
        "endpoint": "/api/invoices/$route.params.invoice_id"
      }
    }
  ]
}
```

---

## 20.3 Action Types

### 20.3.1 action.http_request

Calls a backend API endpoint. The most fundamental action type — it is the bridge between user interactions and backend mutations.

```typescript
interface HttpRequestActionProps {
  method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  endpoint: string;              // May include $route.params, $state, $row bindings
  body?: Record<string, unknown> | Binding;  // Request body (POST/PUT/PATCH)
  headers?: Record<string, string>;         // Additional request headers
  query_params?: Record<string, unknown>;   // URL query parameters
  timeout_ms?: number;           // Request timeout (default: 30000)
  retry?: RetrySpec;             // Retry policy on failure
  optimistic?: OptimisticSpec;   // Optimistic update spec (see Section 20.5)
}
```

**Example — PATCH request with form body:**
```json
{
  "type": "action.http_request",
  "props": {
    "method": "PATCH",
    "endpoint": "/api/purchase-orders/$route.params.po_id/approve",
    "body": {
      "comment": "$form.approval_form.comment",
      "approved_at": "$expr{ new Date().toISOString() }"
    }
  },
  "result_var": "approval_result"
}
```

The HTTP request action automatically:
- Includes the session authentication token as a Bearer token header
- Includes tenant and locale headers (`X-Tenant-ID`, `Accept-Language`)
- Includes CSRF protection headers on state-mutating requests (web)
- Records a server-side audit log entry (the backend processes this via the Audit middleware)

### 20.3.2 action.navigate

Triggers navigation to another surface. Covered in detail in Chapter 19.

```json
{
  "type": "action.navigate",
  "props": {
    "route": {
      "surface": "po_detail",
      "params": { "po_id": "$action.approval_result.po_id" }
    },
    "presentation": "replace"
  }
}
```

### 20.3.3 action.set_state

Sets one or more surface state variables.

```json
{
  "type": "action.set_state",
  "props": {
    "assignments": [
      { "var": "selected_tab", "value": "line_items" },
      { "var": "is_editing", "value": true },
      { "var": "active_filters", "value": { "status": "pending" } }
    ]
  }
}
```

This action is synchronous and executes immediately. It is commonly used in conjunction with `$if` and `$show` bindings to show/hide panels, switch tabs, or toggle editing mode.

### 20.3.4 action.show_toast

Displays a toast notification.

```typescript
interface ShowToastProps {
  message: string | Binding;
  variant: 'info' | 'success' | 'warning' | 'error';
  duration_ms?: number;          // Default: 4000
  action_label?: string;         // Optional action label (e.g., "Undo")
  action_id?: string;            // Action to execute when action_label is tapped
}
```

```json
{
  "type": "action.show_toast",
  "props": {
    "message": "Invoice #$action.saved_invoice.invoice_number created",
    "variant": "success",
    "duration_ms": 5000,
    "action_label": "View",
    "action_id": "view_created_invoice"
  }
}
```

### 20.3.5 action.show_modal

Opens a surface as a modal overlay.

```json
{
  "type": "action.show_modal",
  "props": {
    "surface": "vendor_picker",
    "params": { "category": "$state.selected_category" },
    "on_close_action_id": "handle_vendor_selected",
    "size": "lg"
  }
}
```

| Prop | Type | Description |
|------|------|-------------|
| `surface` | `string` | Surface ID to load in the modal |
| `params` | `Record<string, Binding>` | Route parameters for the modal surface |
| `on_close_action_id` | `string` | Action to execute when the modal closes; receives `$modal.result` |
| `size` | `"sm" \| "md" \| "lg" \| "full"` | Modal size |
| `prevent_dismiss` | `boolean` | Prevent closing by clicking outside (default: false) |

### 20.3.6 action.close_modal

Closes the current modal surface, optionally passing a result to the `on_close_action_id`.

```json
{
  "type": "action.close_modal",
  "props": {
    "result": {
      "vendor_id": "$row.id",
      "vendor_name": "$row.name"
    }
  }
}
```

### 20.3.7 action.validate_form

Triggers validation on a form and blocks chain execution if the form has errors.

```json
{
  "type": "action.validate_form",
  "props": {
    "form_id": "invoice_form",
    "focus_first_error": true
  }
}
```

If validation fails, the step's `error_chain` executes (if declared). If no error chain is declared, the action chain stops. The form's field-level error messages are displayed automatically.

### 20.3.8 action.invalidate_data_source

Invalidates one or more data source caches, triggering a refetch.

```json
{
  "type": "action.invalidate_data_source",
  "props": {
    "source_ids": ["invoices_list", "invoice_summary"]
  }
}
```

This action is typically the last step after a successful mutation — it causes the surfaces that display the mutated data to refresh.

### 20.3.9 action.workflow_signal

Sends a signal to a Temporal workflow. Used to advance approval workflows, escalate tasks, or trigger workflow events from the UI.

```typescript
interface WorkflowSignalProps {
  workflow_id: string | Binding;   // Temporal workflow ID
  signal_name: string;             // Signal name registered in the workflow
  payload?: Record<string, unknown>; // Signal payload (may include bindings)
}
```

```json
{
  "type": "action.workflow_signal",
  "props": {
    "workflow_id": "$data.approval.workflow_id",
    "signal_name": "approve",
    "payload": {
      "approved_by": "$user.id",
      "comment": "$form.approval_form.comment",
      "approved_at": "$expr{ new Date().toISOString() }"
    }
  }
}
```

The rendering engine sends this to the backend's workflow signal endpoint (`POST /api/workflows/:id/signal`). The backend routes the signal to Temporal. The action does not wait for the workflow to complete — it returns when the signal is delivered.

### 20.3.10 action.copy_to_clipboard

Copies a value to the system clipboard.

```json
{
  "type": "action.copy_to_clipboard",
  "props": {
    "value": "$data.invoice.invoice_number",
    "success_message": "Invoice number copied"
  }
}
```

### 20.3.11 action.download_file

Initiates a file download.

```json
{
  "type": "action.download_file",
  "props": {
    "url": "/api/invoices/$route.params.invoice_id/pdf",
    "filename": "invoice-$data.invoice.invoice_number.pdf",
    "mime_type": "application/pdf"
  }
}
```

On mobile, this opens the platform's share sheet. On web, it triggers a browser download.

### 20.3.12 action.open_url

Opens a URL in the platform browser.

```json
{
  "type": "action.open_url",
  "props": {
    "url": "$data.vendor.website_url",
    "target": "external"
  }
}
```

| `target` | Behaviour |
|---------|-----------|
| `"external"` (default) | Open in the system browser (new tab on web, Safari/Chrome on mobile) |
| `"in_app"` | Open in an in-app web view (mobile only) |

### 20.3.13 action.print

Triggers the platform print dialog for the current surface or a specific component.

```json
{
  "type": "action.print",
  "props": {
    "surface": "invoice_print_view",
    "params": { "invoice_id": "$route.params.invoice_id" }
  }
}
```

### 20.3.14 action.reset_form

Resets a form to its initial values, clearing all field values, dirty flags, and error messages.

```json
{
  "type": "action.reset_form",
  "props": {
    "form_id": "filter_form"
  }
}
```

### 20.3.15 action.batch

Executes a list of sub-actions in parallel. All sub-actions must succeed for the batch to be considered successful.

```json
{
  "type": "action.batch",
  "props": {
    "actions": [
      {
        "type": "action.invalidate_data_source",
        "props": { "source_ids": ["invoices_list"] }
      },
      {
        "type": "action.show_toast",
        "props": { "message": "3 invoices approved", "variant": "success" }
      }
    ]
  }
}
```

---

## 20.4 Action Chaining

### 20.4.1 Sequential Execution

Steps in a chain execute sequentially. Each step waits for the previous step to complete before starting. A step failure (unless caught by an `error_chain`) stops execution of the remaining steps.

```
step 1 → step 2 → step 3 → step 4
            ↓ (error)
         error step 2a → error step 2b → stop
```

### 20.4.2 Accessing Previous Step Results

The `result_var` field on a step stores that step's response in the `$action` scope. Subsequent steps can access it via `$action.<result_var>`.

```json
{
  "chain": [
    {
      "type": "action.http_request",
      "props": { "method": "POST", "endpoint": "/api/invoices", "body": "$form.invoice_form" },
      "result_var": "created_invoice"
    },
    {
      "type": "action.navigate",
      "props": {
        "route": {
          "surface": "invoice_detail",
          "params": { "invoice_id": "$action.created_invoice.id" }
        }
      }
    }
  ]
}
```

The `$action` scope is local to the current action chain execution. It is not visible to other action chains or surface-level state bindings.

### 20.4.3 Conditional Steps

The `skip_if` field on a step skips that step when the binding evaluates to truthy. This enables conditional branching within a chain.

```json
{
  "chain": [
    {
      "type": "action.http_request",
      "props": { ... },
      "result_var": "save_result"
    },
    {
      "type": "action.workflow_signal",
      "props": {
        "workflow_id": "$action.save_result.workflow_id",
        "signal_name": "approve"
      },
      "skip_if": "$expr{ !$action.save_result.requires_approval }"
    },
    {
      "type": "action.show_toast",
      "props": { "message": "Saved", "variant": "success" }
    }
  ]
}
```

---

## 20.5 Optimistic Updates

Optimistic updates allow the UI to reflect the expected result of an action immediately, before the server confirms the result. If the server returns an error, the optimistic update is rolled back.

```json
{
  "type": "action.http_request",
  "props": {
    "method": "PATCH",
    "endpoint": "/api/invoices/$row.id",
    "body": { "status": "approved" },
    "optimistic": {
      "data_source_id": "invoices_list",
      "row_key": "$row.id",
      "patch": { "status": "approved" }
    }
  }
}
```

### 20.5.1 Optimistic Update Mechanics

1. Before the HTTP request is sent, the rendering engine applies the `patch` to the specified data source's cached data
2. The UI re-renders immediately with the optimistic values (the row shows `status: "approved"`)
3. The HTTP request is sent in the background
4. **On success:** The data source is invalidated and refetched; the server's authoritative data replaces the optimistic values
5. **On failure:** The optimistic patch is rolled back; the original values are restored; the error chain executes

```typescript
interface OptimisticSpec {
  data_source_id: string;    // Data source cache to update
  row_key?: string | Binding; // Specific row to patch (for list data sources)
  patch: Record<string, unknown>; // Values to apply (may include bindings from $row scope)
}
```

### 20.5.2 Optimistic State Variables

For state-variable-based optimism (showing a loading state on a button while an action runs), the `loading_state_var` field on the action definition sets the named state variable to `true` at the start of execution and `false` at the end.

```json
{
  "id": "approve_action",
  "loading_state_var": "is_approving",
  "chain": [ ... ]
}
```

The button that triggers this action can bind its `loading` prop to `$state.is_approving`.

---

## 20.6 Action Context and Payload Binding

### 20.6.1 Binding Scopes in Actions

Action step props can reference all binding scopes plus the action-specific scope:

| Scope | Source |
|-------|--------|
| `$state.*` | Surface state variables |
| `$form.*` | Form field values |
| `$data.*` | Data source query results |
| `$row.*` | Current row context (within data table actions) |
| `$route.params.*` | Current route parameters |
| `$user.*` | Session user context |
| `$tenant.*` | Tenant context |
| `$action.*` | Results from previous steps in the current chain |
| `$selection.*` | Currently selected rows (bulk action context) |

### 20.6.2 Bulk Action Context

When an action is triggered from a bulk action button, the `$selection` scope contains the selected rows:

```json
{
  "type": "action.http_request",
  "props": {
    "method": "POST",
    "endpoint": "/api/invoices/bulk-approve",
    "body": {
      "invoice_ids": "$selection.ids",
      "comment": "$form.bulk_approval_form.comment"
    }
  }
}
```

`$selection.ids` is an array of the `row_key` values for all selected rows. `$selection.rows` provides the full row data objects for all selected rows.

---

## 20.7 Error Handling in Actions

### 20.7.1 Error Chain

Each step can declare an `error_chain` — a list of steps to execute when that step fails. The error chain receives the error in the `$error` scope.

```json
{
  "chain": [
    {
      "type": "action.http_request",
      "props": { "method": "POST", "endpoint": "/api/invoices", "body": "$form.invoice_form" },
      "result_var": "created_invoice",
      "error_chain": [
        {
          "type": "action.show_toast",
          "props": {
            "message": "$expr{ $error.status === 422 ? $error.detail : 'Failed to save invoice. Please try again.' }",
            "variant": "error"
          }
        }
      ]
    }
  ]
}
```

### 20.7.2 Error Context ($error scope)

When an error chain executes, the `$error` scope is populated:

| Variable | Type | Value |
|----------|------|-------|
| `$error.code` | `string` | Application error code (e.g., `"invoice.invalid_line_items"`) |
| `$error.message` | `string` | Human-readable error message |
| `$error.detail` | `string \| null` | Additional detail (validation summary, etc.) |
| `$error.status` | `number` | HTTP status code (for HTTP request errors) |
| `$error.field_errors` | `Record<string, string>` | Field-level validation errors (for 422 responses) |
| `$error.step_id` | `string` | ID of the step that failed |

### 20.7.3 Global Error Handler

The surface root can declare a `default_error_action_id` that is executed when a step fails and has no `error_chain`. This provides a catch-all handler for unexpected errors.

```json
{
  "surface": {
    "default_error_action_id": "show_generic_error"
  },
  "actions": {
    "show_generic_error": {
      "chain": [
        {
          "type": "action.show_toast",
          "props": { "message": "Something went wrong. Please try again.", "variant": "error" }
        }
      ]
    }
  }
}
```

### 20.7.4 Validation Errors (422 Responses)

When an HTTP request action receives a `422 Unprocessable Entity` response, the rendering engine automatically distributes `field_errors` from the response body to the corresponding form fields. Field error messages appear inline below each field without requiring explicit error_chain handling.

The 422 response body format is:

```json
{
  "error": "validation_failed",
  "message": "The form contains errors",
  "field_errors": {
    "amount": "Amount must be greater than zero",
    "due_date": "Due date cannot be in the past",
    "line_items[0].quantity": "Quantity is required"
  }
}
```

---

## 20.8 Action Permissions

### 20.8.1 Component-Level Permission Gates

Action triggers (buttons, links) can be conditionally rendered based on permissions using `$if` with `$user.can()`:

```json
{
  "type": "action.button",
  "props": {
    "label": "Approve",
    "action_id": "approve_po"
  },
  "$if": "$user.can('purchase_order:approve')"
}
```

This is a rendering-level check — it controls whether the trigger is visible. The backend re-validates the permission when the action's HTTP request is processed.

### 20.8.2 Server-Side Permission Enforcement

The backend enforces permissions at the API endpoint level. The rendering engine never assumes that a client-side permission check is sufficient — `$if` and `$show` on action triggers are purely for UX, not security.

### 20.8.3 Dangerous Actions

Actions that are destructive or high-risk should combine both:
1. A `$if` binding that renders the trigger only for users with the required permission
2. A `confirm` spec on the action definition that requires explicit user confirmation

This combination ensures destructive actions are not taken accidentally and are not available to users without the required permissions.

---

## 20.9 Offline Action Queue

When an `action.http_request` step executes while the device is offline (mobile) or network connectivity is unavailable, the step is queued for later execution rather than failing immediately.

### 20.9.1 Queueable Actions

Only actions explicitly marked `offline_queueable: true` are queued when offline. This is an opt-in behavior because not all mutations are safe to queue — a stale optimistic read might produce an invalid mutation when network is restored.

```json
{
  "type": "action.http_request",
  "props": {
    "method": "POST",
    "endpoint": "/api/expenses",
    "body": "$form.expense_form",
    "offline_queueable": true
  }
}
```

### 20.9.2 Queue Semantics

- Queued actions execute in the order they were queued, maintaining causal ordering
- On network restore, the queue replays from the oldest queued action
- If a queued action fails on replay, the user is notified with a toast and the action remains in the queue for manual retry or dismissal
- The offline queue is persisted to encrypted local storage; it survives app termination

### 20.9.3 Queue Status Indicator

The shell displays an indicator when there are pending offline actions. Tapping the indicator opens the offline queue management surface, where users can see queued actions, retry failed ones, or discard them.

---

## 20.10 Action Telemetry

Every action execution produces a telemetry event. Events are emitted regardless of success or failure.

| Event | Properties | Trigger |
|-------|-----------|---------|
| `action_triggered` | action_id, trigger_component_id, surface_id | User initiates an action |
| `action_step_start` | action_id, step_id, step_type | Each step begins |
| `action_step_success` | action_id, step_id, step_type, duration_ms | Step succeeds |
| `action_step_error` | action_id, step_id, step_type, error_code, http_status | Step fails |
| `action_chain_complete` | action_id, total_duration_ms, step_count | Chain finishes (all steps) |
| `action_chain_aborted` | action_id, failed_step_id, error_code | Chain stops on error |
| `action_confirmed` | action_id | User confirms a destructive action |
| `action_confirm_dismissed` | action_id | User dismisses confirmation dialog |
| `optimistic_update_applied` | action_id, data_source_id | Optimistic update applied |
| `optimistic_update_rolled_back` | action_id, data_source_id | Optimistic update rolled back |
| `offline_action_queued` | action_id, action_type | Action queued for offline replay |
| `offline_action_replayed` | action_id, queue_duration_ms | Queued action executed |

---

*End of Chapter 20*

**Previous:** [Chapter 19 — Navigation Framework](./19-navigation-framework.md)
**Next:** [Chapter 21 — Event System](./21-event-system.md)
