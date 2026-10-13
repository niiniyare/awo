# Chapter 50 — SDK

> **Volume:** X — Reference
> **Phase:** 4
> **Audience:** Plugin Developers, Tenant Integration Engineers
> **Prerequisites:** Chapters 12, 13, 25

---

## Table of Contents

- [50.1 SDK Overview](#501-sdk-overview)
- [50.2 Authentication](#502-authentication)
- [50.3 Go SDK](#503-go-sdk)
- [50.4 TypeScript SDK](#504-typescript-sdk)
- [50.5 Webhook Integration](#505-webhook-integration)
- [50.6 Plugin SDK](#506-plugin-sdk)
- [50.7 SDK Versioning and Compatibility](#507-sdk-versioning-and-compatibility)

---

## 50.1 SDK Overview

The AwoERP SDK provides clients for external developers building:
- Tenant integrations (sync AwoERP data with other systems)
- Custom automations (trigger workflows via API)
- Plugins (extend the platform with custom surfaces and actions)

The SDK is a thin wrapper over the HTTP API. It handles authentication, request signing, pagination, and error deserialization. It does not add business logic.

**Available SDKs:**
| Language | Package | Use Case |
|----------|---------|---------|
| Go | `github.com/awoerp/sdk-go` | Backend integrations, workers |
| TypeScript/Node.js | `@awoerp/sdk` | Frontend integrations, automation |
| Python | `awoerp-sdk` | Data pipelines, scripting |

---

## 50.2 Authentication

External SDK clients authenticate with API keys, not JWTs. API keys are created in the platform admin surface under Settings → API Keys.

API keys are scoped to a tenant and carry a set of permissions:
```
read:invoices
write:invoices
read:vendors
read:employees
webhooks:subscribe
```

**API Key format:**
```
awo_live_<32-character-random-string>
awo_test_<32-character-random-string>
```

Test keys work against staging data; live keys work against production data.

---

## 50.3 Go SDK

### Installation

```bash
go get github.com/awoerp/sdk-go
```

### Client Initialization

```go
import "github.com/awoerp/sdk-go/awoerp"

client, err := awoerp.NewClient(
    awoerp.WithAPIKey(os.Getenv("AWOERP_API_KEY")),
    awoerp.WithBaseURL("https://api.awoerp.dev"),
    // Optional: configure HTTP timeout
    awoerp.WithTimeout(30 * time.Second),
)
if err != nil {
    log.Fatal(err)
}
```

### Listing Invoices

```go
// ListInvoices automatically handles pagination
ctx := context.Background()

invoices, err := client.Invoices.List(ctx, &awoerp.ListInvoicesParams{
    Status:   awoerp.InvoiceStatusPendingApproval,
    PageSize: 50,
})
if err != nil {
    return fmt.Errorf("list invoices: %w", err)
}

for _, invoice := range invoices.Data {
    fmt.Printf("Invoice %s: %s %d\n", invoice.ID, invoice.CurrencyCode, invoice.Amount)
}

// Paginate with cursor
for invoices.Meta.HasMore {
    invoices, err = client.Invoices.List(ctx, &awoerp.ListInvoicesParams{
        Cursor: invoices.Meta.NextCursor,
    })
    if err != nil {
        return fmt.Errorf("list next page: %w", err)
    }
    // process invoices.Data
}
```

### Creating an Invoice

```go
invoice, err := client.Invoices.Create(ctx, &awoerp.CreateInvoiceRequest{
    VendorID: "vendor-001",
    LineItems: []awoerp.LineItem{
        {
            Description: "Professional Services",
            Quantity:    1,
            UnitPrice:   250000, // in minor currency units (cents/fils)
            CurrencyCode: "KES",
        },
    },
})
if err != nil {
    var apiErr *awoerp.APIError
    if errors.As(err, &apiErr) {
        fmt.Printf("API error %s: %s\n", apiErr.Code, apiErr.Message)
    }
    return err
}

fmt.Printf("Created invoice: %s\n", invoice.ID)
```

### Error Handling

```go
_, err := client.Invoices.Approve(ctx, invoiceID)
if err != nil {
    var apiErr *awoerp.APIError
    if errors.As(err, &apiErr) {
        switch apiErr.Code {
        case "not_found":
            // 404: invoice does not exist
        case "forbidden":
            // 403: API key does not have approve permission
        case "validation_failed":
            // 422: business rule violation (e.g., already approved)
            for _, detail := range apiErr.Details {
                fmt.Printf("  Field %s: %s\n", detail.Field, detail.Code)
            }
        }
    }
    return err
}
```

---

## 50.4 TypeScript SDK

### Installation

```bash
pnpm add @awoerp/sdk
```

### Client Initialization

```typescript
import { AwoERPClient } from '@awoerp/sdk';

const client = new AwoERPClient({
    apiKey: process.env.AWOERP_API_KEY!,
    baseURL: 'https://api.awoerp.dev',
});
```

### Listing and Paginating

```typescript
// Auto-paginate: returns an async iterator
for await (const invoice of client.invoices.listAll({ status: 'pending_approval' })) {
    console.log(`${invoice.id}: ${invoice.currencyCode} ${invoice.amount}`);
}

// Manual pagination
const page = await client.invoices.list({ pageSize: 25 });
if (page.meta.hasMore) {
    const nextPage = await client.invoices.list({ cursor: page.meta.nextCursor });
}
```

### Webhook Event Types

```typescript
import { WebhookEvent, InvoiceApprovedEvent } from '@awoerp/sdk/webhooks';

function handleWebhook(event: WebhookEvent) {
    if (event.type === 'invoice.approved') {
        const invoiceEvent = event as InvoiceApprovedEvent;
        console.log(`Invoice ${invoiceEvent.data.invoiceId} approved by ${invoiceEvent.data.approvedBy}`);
    }
}
```

---

## 50.5 Webhook Integration

### Registering a Webhook

```http
POST /api/v1/webhooks
Authorization: Bearer <api_key>

{
  "url": "https://my-integration.example.com/awoerp-events",
  "events": ["invoice.created", "invoice.approved", "invoice.paid"],
  "secret": "whsec_my-webhook-secret"
}
```

Returns a webhook ID and the signing secret. Store the secret securely.

### Webhook Payload

```json
{
  "id": "evt-550e8400-e29b-41d4-a716-446655440000",
  "type": "invoice.approved",
  "tenant_id": "tenant-acme",
  "created_at": "2024-03-15T14:22:01Z",
  "data": {
    "invoice_id": "inv-001",
    "approved_by": "user:approver-1",
    "approved_at": "2024-03-15T14:22:00Z",
    "amount": 150000,
    "currency_code": "KES"
  }
}
```

### Verifying Webhook Signatures

Every webhook request includes a `X-AwoERP-Signature` header. Verify it before processing:

```go
// Go
import "github.com/awoerp/sdk-go/awoerp/webhook"

func handleWebhook(w http.ResponseWriter, r *http.Request) {
    body, _ := io.ReadAll(r.Body)
    signature := r.Header.Get("X-AwoERP-Signature")
    secret := os.Getenv("AWOERP_WEBHOOK_SECRET")

    if !webhook.VerifySignature(body, signature, secret) {
        http.Error(w, "invalid signature", 401)
        return
    }

    // Process the event
    event, _ := webhook.ParseEvent(body)
    switch event.Type {
    case "invoice.approved":
        // handle approval
    }
    w.WriteHeader(204)
}
```

```typescript
// TypeScript
import { verifyWebhookSignature, parseWebhookEvent } from '@awoerp/sdk/webhooks';

app.post('/awoerp-events', (req, res) => {
    const signature = req.headers['x-awoerp-signature'] as string;
    const body = req.rawBody; // unparsed buffer

    if (!verifyWebhookSignature(body, signature, process.env.AWOERP_WEBHOOK_SECRET!)) {
        return res.status(401).send('Invalid signature');
    }

    const event = parseWebhookEvent(body);
    // handle event
    res.status(204).send();
});
```

### Webhook Retry Policy

Failed webhook deliveries (non-2xx response or timeout) are retried with exponential backoff:
- Retry 1: 30 seconds
- Retry 2: 5 minutes
- Retry 3: 30 minutes
- Retry 4: 2 hours
- Retry 5: 8 hours (final attempt)

After 5 failed attempts, the webhook is marked as failed and the platform admin is notified.

---

## 50.6 Plugin SDK

The plugin SDK allows developers to extend AwoERP with custom surfaces and server-side actions that run within the platform's security model.

### Plugin Structure

```
my-plugin/
├── plugin.yaml          # Plugin manifest
├── surfaces/            # Custom surface definitions
│   └── customs-docs.json
├── actions/             # Server-side action handlers (Go)
│   └── generate_customs_doc.go
├── translations/        # Plugin-specific translation keys
│   ├── en-KE/plugin.json
│   └── sw-KE/plugin.json
└── migrations/          # Plugin-specific DB migrations
    └── 00001_create_customs_docs_table.up.sql
```

### Plugin Manifest

```yaml
# plugin.yaml
id: customs-docs-plugin
name: Customs Documentation
version: 1.2.0
description: Generate customs documentation from AwoERP invoices
author: Customs Integrations Ltd

permissions:
  - read:invoices
  - read:vendors
  - write:plugin_data  # Plugin-scoped table

surfaces:
  - id: customs_doc_form
    path: surfaces/customs-docs.json

actions:
  - id: generate_customs_doc
    handler: GenerateCustomsDoc
    permissions_required:
      - read:invoices
```

### Plugin Action Handler

```go
// actions/generate_customs_doc.go
package actions

import (
    "context"
    "github.com/awoerp/plugin-sdk/pluginsdk"
)

type GenerateCustomsDocInput struct {
    InvoiceID string `json:"invoice_id"`
    Format    string `json:"format"` // "pdf" or "xml"
}

type GenerateCustomsDocOutput struct {
    DocumentURL string `json:"document_url"`
    Reference   string `json:"reference"`
}

func GenerateCustomsDoc(
    ctx context.Context,
    input GenerateCustomsDocInput,
    deps pluginsdk.Dependencies,
) (GenerateCustomsDocOutput, error) {
    // Fetch invoice via SDK (respects tenant + RLS)
    invoice, err := deps.Client.Invoices.Get(ctx, input.InvoiceID)
    if err != nil {
        return GenerateCustomsDocOutput{}, err
    }

    // Generate document using plugin logic
    doc := generateDocument(invoice, input.Format)

    // Store in object storage (plugin-scoped bucket)
    url, err := deps.Storage.Put(ctx, "customs-docs/"+invoice.ID+".pdf", doc)
    if err != nil {
        return GenerateCustomsDocOutput{}, err
    }

    return GenerateCustomsDocOutput{
        DocumentURL: url,
        Reference:   generateReference(invoice),
    }, nil
}
```

Plugin action handlers run in the platform's tenant context. They cannot access other tenants' data. They cannot call FGA with arbitrary tuples. They have the permissions declared in the manifest and no more.

---

## 50.7 SDK Versioning and Compatibility

### SDK version alignment

| SDK Version | API Version | Minimum Platform Version |
|-------------|-------------|------------------------|
| 2.x | v2 | 2.0.0 |
| 1.x | v1 (deprecated) | 1.0.0 |

SDK major versions align with API major versions. SDK minor versions add new convenience methods; they do not change the underlying API calls.

### Deprecation in the SDK

When an API endpoint is deprecated, the corresponding SDK method is marked deprecated in the same SDK release:

```go
// Deprecated: Use client.Invoices.Get(ctx, id) instead.
// This method will be removed in SDK v3.0.0.
func (c *InvoiceClient) FetchByID(ctx context.Context, id string) (*Invoice, error) {
    return c.Get(ctx, id)
}
```

SDK deprecations are announced in the changelog. The SDK major version that removes the deprecated method ships after the corresponding API endpoint's Sunset date.

---

*End of Chapter 50*

**Previous:** [Chapter 49 — End-to-End Examples](./49-e2e-examples.md)
**Next:** [Chapter 51 — Roadmap](./51-roadmap.md)
