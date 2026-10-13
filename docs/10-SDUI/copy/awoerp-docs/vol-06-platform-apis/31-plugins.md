# Chapter 31 — Plugins

> **Volume:** VI — Platform APIs & Configuration
> **Phase:** 3
> **Audience:** Platform Engineers, Plugin Developers, Integration Partners
> **Prerequisites:** Chapters 05–12, 17–21, 25–30

---

## Table of Contents

- [31.1 Plugin Model Overview](#311-plugin-model-overview)
- [31.2 Plugin SDK](#312-plugin-sdk)
- [31.3 Plugin Manifest](#313-plugin-manifest)
- [31.4 Component Extensions](#314-component-extensions)
- [31.5 API Extensions](#315-api-extensions)
- [31.6 Workflow Activity Extensions](#316-workflow-activity-extensions)
- [31.7 Plugin Lifecycle](#317-plugin-lifecycle)
- [31.8 Plugin Distribution](#318-plugin-distribution)
- [31.9 Plugin Security Model](#319-plugin-security-model)

---

## 31.1 Plugin Model Overview

Plugins extend AwoERP with capabilities that go beyond tenant configuration. Where Chapter 30 covers what tenant admins can do without code, Chapter 31 covers what developers can build with the Plugin SDK.

**A plugin can:**
- Register new SDUI component types (custom UI widgets)
- Add new REST API endpoints (custom data fetches and mutations)
- Register new Temporal workflow activity implementations
- Provide new surface definitions (complete new screens)
- Add new navigation items
- Extend entity schemas with strongly-typed field groups

**A plugin cannot:**
- Override or replace existing platform components
- Access another tenant's data
- Bypass the authorization model
- Modify the surface definition schema
- Execute arbitrary SQL on the platform database

Plugins run as separate services in isolated containers. The platform orchestrates them; they cannot directly invoke internal platform services.

---

## 31.2 Plugin SDK

### 31.2.1 SDK Structure

The Plugin SDK is a Go library that provides the scaffolding for building an AwoERP plugin:

```
plugin-sdk/
├── plugin.go          ← Plugin interface and registration
├── component.go       ← Component factory extension API
├── api.go             ← HTTP handler registration
├── workflow.go        ← Temporal activity registration
├── surface.go         ← Surface definition provider API
├── store.go           ← Plugin-scoped data store (PostgreSQL)
├── auth.go            ← Auth context extraction helpers
└── events.go          ← SSE event emission API
```

### 31.2.2 Plugin Interface

```go
// Every plugin implements this interface
type Plugin interface {
    // Metadata returns the plugin's manifest
    Metadata() Manifest

    // Register is called when the plugin is loaded.
    // It receives a Registry where the plugin declares its capabilities.
    Register(ctx context.Context, r *Registry) error

    // Health is polled by the platform to check plugin status.
    Health(ctx context.Context) HealthStatus

    // Shutdown is called before the plugin process is terminated.
    Shutdown(ctx context.Context) error
}

type Registry struct {
    Components func(factories ...ComponentFactory)
    Routes     func(routes ...Route)
    Activities func(activities ...WorkflowActivity)
    Surfaces   func(surfaces ...SurfaceProvider)
}
```

### 31.2.3 Example Plugin Bootstrap

```go
package main

import (
    "context"
    sdk "github.com/awoerp/plugin-sdk"
)

type EInvoicePlugin struct{}

func (p *EInvoicePlugin) Metadata() sdk.Manifest {
    return sdk.Manifest{
        ID:          "einvoice-ke",
        Name:        "Kenya e-Invoice",
        Version:     "1.2.0",
        Description: "Kenya Revenue Authority e-invoice integration",
        Author:      "Acme Integrations Ltd",
    }
}

func (p *EInvoicePlugin) Register(ctx context.Context, r *sdk.Registry) error {
    // Register custom components
    r.Components(
        &KRAQRCodeFactory{},
        &EInvoiceStatusBadgeFactory{},
    )

    // Register API routes
    r.Routes(
        sdk.Route{Method: "POST", Path: "/plugin/einvoice-ke/submit", Handler: handleSubmit},
        sdk.Route{Method: "GET",  Path: "/plugin/einvoice-ke/status/{id}", Handler: handleStatus},
    )

    // Register Temporal activities
    r.Activities(
        &SubmitToKRAActivity{},
        &PollKRAStatusActivity{},
    )

    return nil
}

func main() {
    sdk.Run(&EInvoicePlugin{})
}
```

---

## 31.3 Plugin Manifest

The plugin manifest is the contract between the plugin and the platform:

```json
{
  "id": "einvoice-ke",
  "name": "Kenya e-Invoice",
  "version": "1.2.0",
  "schema_version": "1.0",
  "description": "Kenya Revenue Authority e-invoice integration for automatic invoice submission",
  "author": {
    "name": "Acme Integrations Ltd",
    "email": "plugins@acme.co.ke",
    "url": "https://acme.co.ke"
  },
  "min_platform_version": "2.0.0",
  "capabilities": {
    "components": ["plugin.kra_qr_code", "plugin.einvoice_status_badge"],
    "routes": [
      "POST /plugin/einvoice-ke/submit",
      "GET /plugin/einvoice-ke/status/{id}"
    ],
    "temporal_activities": ["SubmitToKRAActivity", "PollKRAStatusActivity"],
    "surfaces": ["einvoice_submission"],
    "permissions_required": [
      "invoice.read",
      "plugin.einvoice-ke.submit"
    ]
  },
  "config_schema": {
    "kra_pin": { "type": "string", "required": true, "label": "KRA PIN" },
    "environment": { "type": "string", "enum": ["sandbox", "production"], "default": "sandbox" }
  },
  "data_store": {
    "tables": ["einvoice_submissions", "einvoice_errors"]
  }
}
```

### 31.3.1 Permissions Required

Plugins declare which platform permissions they require. The platform enforces these at install time — a plugin can only be installed if the tenant admin has been informed of and consented to the permission requirements.

Plugin-specific permissions (e.g., `plugin.einvoice-ke.submit`) are registered with the platform's authorization model as new permission types. Tenant admins can grant or revoke plugin permissions to roles through the standard role management surface.

---

## 31.4 Component Extensions

### 31.4.1 Registering a Component

Plugin components follow the same `ComponentFactory` protocol used by the platform's built-in components (Chapter 09):

```go
type KRAQRCodeFactory struct{}

func (f *KRAQRCodeFactory) ComponentType() string {
    return "plugin.kra_qr_code"  // All plugin types are prefixed 'plugin.'
}

func (f *KRAQRCodeFactory) PropsSchema() JSONSchema {
    return JSONSchema{
        Properties: map[string]JSONSchemaProperty{
            "invoice_id": {Type: "string", Required: true},
            "size":       {Type: "number", Default: 200},
        },
    }
}

// RenderWeb returns the React component definition as JSON
func (f *KRAQRCodeFactory) RenderWeb(props map[string]any) (ComponentDefinition, error) {
    return ComponentDefinition{
        ComponentType: "plugin.kra_qr_code",
        Props:         props,
        // Web renderer calls the plugin's /plugin/einvoice-ke/qr-code endpoint
        // to fetch the QR code image URL
        DataSources: []DataSourceDeclaration{{
            ID:       "kra_qr_code",
            Endpoint: fmt.Sprintf("/plugin/einvoice-ke/qr-code/%s", props["invoice_id"]),
            Strategy: "eager",
        }},
    }, nil
}
```

### 31.4.2 Plugin Component Rendering

When the SDUI rendering engine encounters a `plugin.*` component type, it looks up the registered plugin factory. On web:

```typescript
class PluginComponentRegistry {
    private factories = new Map<string, PluginComponentFactory>();

    register(factory: PluginComponentFactory): void {
        if (!factory.type.startsWith('plugin.')) {
            throw new Error('Plugin component types must start with plugin.');
        }
        this.factories.set(factory.type, factory);
    }

    render(node: ComponentNode): React.ReactElement | null {
        const factory = this.factories.get(node.type);
        if (!factory) {
            console.warn(`Unknown plugin component type: ${node.type}`);
            return <PluginComponentFallback type={node.type} />;
        }
        return factory.render(node);
    }
}
```

Plugin component JavaScript bundles are loaded lazily when the rendering engine first encounters their type. The plugin manifest declares the bundle URL.

---

## 31.5 API Extensions

### 31.5.1 Plugin Route Registration

Plugin API routes are served through the platform's API gateway under the `/plugin/{plugin-id}/` prefix. The platform's auth middleware applies to all plugin routes:

```go
// Plugin routes receive the same auth context as platform routes
func handleSubmit(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    tenantID := sdk.TenantID(ctx)   // From platform auth middleware
    userID := sdk.UserID(ctx)       // From platform auth middleware

    // Check plugin-specific permission
    if !sdk.HasPermission(ctx, "plugin.einvoice-ke.submit") {
        sdk.WriteError(w, 403, "forbidden", "You do not have permission to submit e-invoices")
        return
    }

    var req SubmitRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        sdk.WriteError(w, 400, "bad_request", "Invalid request")
        return
    }

    // Access plugin data store (tenant-scoped)
    store := sdk.Store(ctx)
    submission := EInvoiceSubmission{
        TenantID:  tenantID,
        InvoiceID: req.InvoiceID,
        Status:    "pending",
    }
    if err := store.Create(ctx, &submission); err != nil {
        sdk.WriteError(w, 500, "internal_error", "Failed to create submission")
        return
    }

    // Start Temporal workflow
    sdk.StartWorkflow(ctx, "KRASubmissionWorkflow", KRASubmissionInput{
        SubmissionID: submission.ID,
        TenantID:     tenantID,
    })

    sdk.WriteJSON(w, 202, map[string]string{"submission_id": submission.ID})
}
```

### 31.5.2 Plugin Data Store

Each plugin gets a dedicated PostgreSQL schema (`plugin_{plugin_id}`) with tables declared in its manifest. The SDK's data store API is tenant-scoped — all queries automatically filter by `tenant_id`:

```go
// Store operations are always tenant-scoped
store := sdk.Store(ctx)  // Automatically has tenantID in scope

// Equivalent to: SELECT * FROM plugin_einvoice_ke.einvoice_submissions WHERE tenant_id = $tenantID
submissions, err := store.List(ctx, "einvoice_submissions", sdk.ListParams{
    Filters: sdk.Filters{"status": "pending"},
    Limit:   100,
})
```

The plugin schema is created by running the plugin's declared migrations when the plugin is installed for a tenant.

---

## 31.6 Workflow Activity Extensions

### 31.6.1 Registering Activities

Plugins can register Temporal workflow activities that can be invoked from both platform workflows and plugin-provided workflow definitions:

```go
type SubmitToKRAActivity struct {
    httpClient *http.Client
}

// ActivityName returns the name used to register this activity with Temporal
func (a *SubmitToKRAActivity) ActivityName() string {
    return "SubmitToKRAActivity"
}

func (a *SubmitToKRAActivity) Execute(ctx context.Context,
    input SubmitToKRAInput) (SubmitToKRAOutput, error) {

    // Fetch plugin config for this tenant
    config, err := sdk.PluginConfig[KRAConfig](ctx)
    if err != nil {
        return SubmitToKRAOutput{}, err
    }

    // Call KRA API
    resp, err := a.httpClient.Post(
        kraEndpoint(config.Environment)+"/submit",
        "application/json",
        buildKRAPayload(input),
    )
    if err != nil {
        return SubmitToKRAOutput{}, err
    }

    return SubmitToKRAOutput{
        KRAID:     resp.KRAID,
        Status:    resp.Status,
        Timestamp: time.Now(),
    }, nil
}
```

Activities are registered with the Temporal worker that runs in the plugin's container. The platform's workflow orchestration service routes activity tasks to the appropriate plugin worker.

---

## 31.7 Plugin Lifecycle

### 31.7.1 Plugin States

```
available → installing → active → updating → suspended → uninstalling → uninstalled
```

### 31.7.2 Installation Flow

```
Tenant admin: Install plugin from marketplace
    ↓
Platform: Review permission requirements with admin
    ↓
Admin: Consent to permissions
    ↓
Platform: 
  1. Create plugin_installations record
  2. Run plugin database migrations (create plugin schema)
  3. Seed initial plugin config
  4. Register plugin's component types in surface compiler
  5. Register plugin's routes in API gateway
  6. Start plugin container (if not already running for another tenant)
  7. Status → 'active'
    ↓
Admin: Configure plugin (KRA PIN, environment, etc.)
```

### 31.7.3 Plugin Configuration

Plugin configuration is stored per-tenant and encrypted at rest. The plugin SDK provides type-safe access to configuration:

```go
// Plugin config is fetched from the platform, decrypted, and provided to activities
config, err := sdk.PluginConfig[KRAConfig](ctx)

type KRAConfig struct {
    KRAPIN      string `json:"kra_pin"`
    Environment string `json:"environment"`
}
```

Tenant admins configure plugins through a generated configuration surface built from the plugin's `config_schema` in its manifest.

---

## 31.8 Plugin Distribution

### 31.8.1 Plugin Marketplace

Plugins are distributed through the AwoERP Plugin Marketplace. To publish a plugin:

1. Implement the Plugin SDK interface and write the manifest
2. Build and package the plugin container image
3. Submit to the marketplace review process
4. Platform team reviews for security, API usage, and performance
5. Approved plugins are listed in the marketplace

### 31.8.2 Private Plugins

Organizations can develop private plugins for internal use. Private plugins:
- Are installed directly via container registry URL (no marketplace)
- Are visible only to the installing tenant
- Skip the marketplace review process (at the tenant's own risk)
- Must still conform to the plugin manifest schema

### 31.8.3 Plugin Versioning

Plugins follow semantic versioning. The platform supports running multiple versions of a plugin simultaneously during upgrades:

- Tenant A: `einvoice-ke@1.1.0`
- Tenant B: `einvoice-ke@1.2.0` (just updated)

Version migrations are handled by the plugin's own migration scripts. The platform coordinates the upgrade, pausing active workflows if necessary.

---

## 31.9 Plugin Security Model

### 31.9.1 Isolation

Plugins run in isolated containers with:
- No access to the platform's internal database (only their own plugin schema via the SDK's store API)
- No access to other plugins' data or APIs
- No access to cross-tenant data
- Network egress only to declared external domains (declared in manifest: `allowed_egress_domains`)

### 31.9.2 API Gateway Enforcement

All plugin API calls pass through the platform's API gateway, which:
- Validates the auth token (same as platform routes)
- Enforces tenant isolation (injects tenant context)
- Rate limits plugin routes (same rate limiting as platform routes)
- Logs all requests to the audit system
- Rejects requests to plugin routes for tenants that haven't installed the plugin

### 31.9.3 Data Access Audit

Plugin data store access is logged. Tenant admins can view all data access events for installed plugins in the audit log surface, filtered by `source: plugin.{plugin_id}`.

### 31.9.4 Plugin Suspension

If a plugin is suspected of security or data privacy violations, the platform team can immediately:
- Suspend the plugin (reject all API calls to plugin routes)
- Terminate running plugin workflow activities
- Notify affected tenant admins

Suspension is reversible; uninstallation is required for data removal.

---

*End of Chapter 31*

---

*End of Volume VI — Platform APIs & Configuration*

**Previous:** [Chapter 30 — Customization](./30-customization.md)
**Next:** [Chapter 32 — Performance Architecture](../vol-07-performance/32-performance-architecture.md)
