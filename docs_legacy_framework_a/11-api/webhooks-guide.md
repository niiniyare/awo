> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Webhooks — Consumer Guide"
id: api-009
status: accepted
category: GUIDE
stability: STABLE
audience: [api-consumers]
since: "1.0"
normative-level: normative
related:
  - "[Webhooks](webhooks.md)"
  - "[API Authentication](authentication.md)"
  - "[Entity API Reference](entity-api-reference.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Webhooks — Consumer Guide

**API-009 | Status: Accepted | Stability: Stable**

How to subscribe to, receive, verify, and handle Awo webhook events from an external application.

---

## 1. What Are Webhooks

Awo sends HTTP POST requests to your configured endpoint when entity lifecycle events occur. Use webhooks to:

- Sync ERP data to external systems (CRM, e-commerce, BI)
- Trigger automations when invoices are paid
- Send notifications outside of Awo's workflow engine
- Audit ERP events in a separate system

---

## 2. Subscribe to a Webhook

```
POST /api/v1/entities/platform_webhook_subscription
Authorization: Bearer {api_client_token}
X-Tenant-ID: {tenant-uuid}
Content-Type: application/json

{
  "url": "https://your-app.com/webhooks/awo",
  "secret": "your-secret-for-signature-verification",
  "events": [
    "finance_invoice.submitted",
    "finance_invoice.paid",
    "finance_payment.created"
  ],
  "active": true
}
```

Response:

```json
{
  "data": {
    "id": "018e...",
    "url": "https://your-app.com/webhooks/awo",
    "events": ["finance_invoice.submitted", "finance_invoice.paid", "finance_payment.created"],
    "active": true,
    "created_at": "2024-12-15T09:00:00+03:00"
  }
}
```

The `secret` is write-only — store it securely. It is used to verify webhook signatures.

---

## 3. Event Naming Convention

```
{entity_type}.{lifecycle_event}
```

| Event | When fired |
|---|---|
| `finance_invoice.created` | Invoice created |
| `finance_invoice.submitted` | Status changed to Submitted |
| `finance_invoice.approved` | Status changed to Approved |
| `finance_invoice.paid` | Status changed to Paid |
| `finance_invoice.cancelled` | Status changed to Cancelled |
| `crm_customer.created` | New customer |
| `hr_leave_request.approved` | Leave request approved |

Subscribe to specific events — never use `*` wildcard (not supported; high-volume risk).

---

## 4. Webhook Payload

```json
POST https://your-app.com/webhooks/awo
Content-Type: application/json
X-Awo-Event: finance_invoice.submitted
X-Awo-Delivery: 018e1234-5678-7abc-...   (unique delivery ID)
X-Awo-Signature: sha256=a3f2b1c4...       (HMAC-SHA256)
X-Awo-Timestamp: 1702634400               (Unix seconds)

{
  "event": "finance_invoice.submitted",
  "tenant_id": "018e1234-...",
  "record_id": "018e5678-...",
  "timestamp": "2024-12-15T09:00:00+03:00",
  "data": {
    "id": "018e5678-...",
    "number": "INV-2024-00042",
    "customer": "018eaaaa-...",
    "status": "Submitted",
    "total_kes": "450000.0000",
    "submitted_at": "2024-12-15T09:00:00+03:00"
  },
  "previous_data": {
    "status": "Draft"
  }
}
```

`previous_data` contains fields that changed — only the changed fields, not the full previous record.

---

## 5. Signature Verification

Verify every webhook to ensure it came from Awo and was not tampered with:

```go
// Go example
func verifyAwoWebhook(r *http.Request, secret string) error {
    signature := r.Header.Get("X-Awo-Signature")    // "sha256=abc123..."
    timestamp  := r.Header.Get("X-Awo-Timestamp")

    body, err := io.ReadAll(r.Body)
    if err != nil {
        return err
    }
    r.Body = io.NopCloser(bytes.NewBuffer(body))    // restore body

    // Prevent replay attacks: reject if timestamp is >5 minutes old
    ts, _ := strconv.ParseInt(timestamp, 10, 64)
    if time.Now().Unix()-ts > 300 {
        return errors.New("webhook timestamp too old")
    }

    // Compute HMAC-SHA256 over timestamp + "." + body
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write([]byte(timestamp + "." + string(body)))
    expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))

    // Constant-time comparison
    if !hmac.Equal([]byte(expected), []byte(signature)) {
        return errors.New("invalid webhook signature")
    }
    return nil
}
```

```python
# Python example
import hmac, hashlib, time

def verify_awo_webhook(headers, body: bytes, secret: str) -> bool:
    signature  = headers.get("X-Awo-Signature", "")
    timestamp  = headers.get("X-Awo-Timestamp", "")

    # Replay protection
    if abs(time.time() - int(timestamp)) > 300:
        return False

    expected = "sha256=" + hmac.new(
        secret.encode(), (timestamp + "." + body.decode()).encode(), hashlib.sha256
    ).hexdigest()

    return hmac.compare_digest(expected, signature)
```

**Never skip signature verification** — it is the only guarantee the request came from Awo.

---

## 6. Responding to Webhooks

Your endpoint must:

1. Return `200 OK` within **30 seconds**
2. Process the event idempotently (Awo retries on failure)

```go
func handleAwoWebhook(w http.ResponseWriter, r *http.Request) {
    if err := verifyAwoWebhook(r, webhookSecret); err != nil {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    var event AwoWebhookEvent
    json.NewDecoder(r.Body).Decode(&event)

    // Enqueue for async processing — respond immediately
    queue.Enqueue(event)

    w.WriteHeader(http.StatusOK)
}
```

Process in a background queue. Never do slow work (DB writes, external API calls) synchronously in the webhook handler.

---

## 7. Retry Schedule

If your endpoint returns a non-2xx response or times out, Awo retries:

| Attempt | Delay |
|---|---|
| 1 | Immediate |
| 2 | 30 seconds |
| 3 | 5 minutes |
| 4 | 30 minutes |
| 5 | 2 hours |
| 6 | 8 hours |

After 6 failed attempts, the delivery is marked `failed` and no further retries occur. You can re-trigger failed deliveries via the admin API:

```
POST /api/v1/entities/platform_webhook_delivery/{id}/retry
```

---

## 8. Idempotency

Use `X-Awo-Delivery` (the unique delivery ID) to detect retries:

```go
// Check if this delivery was already processed
deliveryID := r.Header.Get("X-Awo-Delivery")
if processedDeliveries.Contains(deliveryID) {
    w.WriteHeader(http.StatusOK)  // Already processed — return 200 to stop retries
    return
}

// Process...
processedDeliveries.Add(deliveryID)
```

---

## 9. Webhook Delivery Log

```
GET /api/v1/entities/platform_webhook_delivery?subscription={id}
```

Returns delivery history: `pending`, `delivered`, `failed`. Each delivery includes the HTTP status returned by your endpoint, the response body (truncated), and the delivery timestamp.

---

## Related Documents

- [Webhooks](webhooks.md) — subscription entity schema, payload format spec
- [API Clients](../07-iam/api-clients.md) — machine-to-machine auth for webhook subscription API
- [API Authentication](authentication.md) — authenticating subscription management requests
