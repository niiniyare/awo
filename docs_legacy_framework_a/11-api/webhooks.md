> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Webhooks"
id: api-004
status: accepted
category: SPEC
stability: STABLE
audience: [module-authors, operators]
since: "1.0"
normative-level: normative
related:
  - "[API Conventions](conventions.md)"
  - "[Hooks](../04-domain/hooks.md)"
  - "[Outbox Pattern](../09-workflow/outbox-pattern.md)"
  - "[Security Model](../15-security/security-model.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Webhooks

**API-004 | Status: Accepted | Stability: Stable**

This document specifies the Awo webhook system: subscription management, payload format, delivery guarantees, signature verification, and retry behavior.

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, RECOMMENDED, MAY, and OPTIONAL are interpreted per RFC 2119.

---

## 1. Webhook Subscriptions

Tenants subscribe to entity lifecycle events via the Settings module or the webhook subscription API:

```
POST /api/v1/webhooks
Authorization: Bearer {session_token}
Content-Type: application/json

{
  "url":    "https://partner.example.com/hooks/awo",
  "events": ["finance_invoice.created", "finance_invoice.submitted", "finance_payment.created"],
  "secret": "whsec_..."
}
```

Response:

```json
{
  "data": {
    "id":      "018e1b2c-...",
    "url":     "https://partner.example.com/hooks/awo",
    "events":  ["finance_invoice.created", "finance_invoice.submitted", "finance_payment.created"],
    "active":  true,
    "created_at": "2024-03-15T09:30:00+03:00"
  }
}
```

### Event Name Format

`{entity-type}.{lifecycle-event}`

| Lifecycle event | Triggered when |
|---|---|
| `created` | Entity record persisted for the first time |
| `updated` | Entity record updated |
| `deleted` | Entity record deleted |
| `submitted` | Custom action `submit` executed |
| `cancelled` | Custom action `cancel` executed |
| `{action-name}` | Any custom action declared on the entity |

---

## 2. Payload Format

```json
{
  "id":        "wh_018e1b2c-3d4e-7f89-abcd-ef0123456789",
  "tenant_id": "a1b2c3d4-...",
  "event":     "finance_invoice.submitted",
  "timestamp": "2024-03-15T09:30:00Z",
  "data": {
    "id":         "018e1b2c-...",
    "number":     "INV-2024-00042",
    "customer":   "customer-uuid",
    "total_kes":  "45000.0000",
    "status":     "Submitted",
    "created_at": "2024-03-14T14:22:00Z",
    "updated_at": "2024-03-15T09:30:00Z"
  }
}
```

Payload rules:
- `id` is unique per delivery attempt — use for idempotency on receiver side
- `timestamp` is always UTC ISO 8601
- `data` is the full entity record at the time of the event (same structure as the entity GET response)
- `Sensitive` fields are **excluded** from webhook payloads
- Custom fields are included under a `custom_fields` key if present

---

## 3. Delivery Guarantee

Webhooks use the transactional outbox pattern (WF-004) for at-least-once delivery:

1. When an entity event fires, the webhook dispatch task is written to `webhook_outbox` in the same PostgreSQL transaction as the entity mutation
2. The webhook relay reads `webhook_outbox` and dispatches HTTP requests to subscriber URLs
3. On HTTP 2xx: delivery confirmed; outbox row deleted
4. On HTTP non-2xx or timeout: exponential backoff retry (see §4)
5. If the subscriber URL returns 2xx but receives the payload more than once: the receiver MUST be idempotent using `id`

**Exactly-once delivery is not guaranteed.** Subscribers MUST handle duplicate deliveries by checking the `id` field.

---

## 4. Retry Schedule

| Attempt | Delay |
|---|---|
| 1 (initial) | Immediately |
| 2 | 30 seconds |
| 3 | 5 minutes |
| 4 | 30 minutes |
| 5 | 2 hours |
| 6 | 12 hours |
| 7 | 24 hours |

After 7 failed attempts, the delivery is marked `failed` and the webhook subscription is automatically **suspended**. The tenant is notified by email. The subscription can be re-activated via the Settings module.

Retry clock: if the target server is unreachable (connection refused, DNS failure, TLS error), the delivery counts as a failed attempt.

Successful delivery: any HTTP 2xx response code.

---

## 5. Signature Verification

Every webhook delivery includes a signature in the `Awo-Signature` header:

```
Awo-Signature: t=1710498600,v1=hmac-sha256-hex-value
```

- `t` = Unix timestamp (seconds) of delivery attempt
- `v1` = `HMAC-SHA256(secret, "{t}.{raw-body}")` encoded as lowercase hex

Receiver MUST:
1. Extract `t` from the header
2. Reject deliveries where `t` is more than 300 seconds in the past (replay attack prevention)
3. Compute `HMAC-SHA256(secret, "{t}.{raw-body}")` using the subscription's `secret`
4. Compare computed value with `v1` using constant-time comparison

```go
// Example receiver verification
func verifyWebhook(secret, rawBody string, header string) (bool, error) {
    parts := strings.SplitN(header, ",", 2)
    // parse t and v1 from parts
    t, _ := strconv.ParseInt(strings.TrimPrefix(parts[0], "t="), 10, 64)
    v1 := strings.TrimPrefix(parts[1], "v1=")

    // Replay guard
    if time.Now().Unix()-t > 300 {
        return false, errors.New("webhook timestamp too old")
    }

    // Compute expected signature
    payload := fmt.Sprintf("%d.%s", t, rawBody)
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write([]byte(payload))
    expected := hex.EncodeToString(mac.Sum(nil))

    // Constant-time compare
    return hmac.Equal([]byte(expected), []byte(v1)), nil
}
```

---

## 6. Subscription Management

```
GET    /api/v1/webhooks              → list subscriptions
GET    /api/v1/webhooks/{id}         → get subscription details + recent delivery log
PATCH  /api/v1/webhooks/{id}         → update URL, events, or secret
DELETE /api/v1/webhooks/{id}         → remove subscription
POST   /api/v1/webhooks/{id}/test    → send test payload to verify URL is reachable
POST   /api/v1/webhooks/{id}/enable  → re-activate a suspended subscription
```

### Test Delivery

```
POST /api/v1/webhooks/{id}/test
```

Sends a synthetic payload with `event: "test"` and `data: {"message": "Test delivery from Awo."}`. Returns HTTP 200 if the subscriber URL responded with 2xx, HTTP 400 with details if not.

---

## 7. Delivery Log

The last 100 delivery attempts are retained per subscription:

```
GET /api/v1/webhooks/{id}/deliveries
```

```json
{
  "data": [
    {
      "id":           "del_018e1b2c-...",
      "event":        "finance_invoice.submitted",
      "attempt":      1,
      "status":       "succeeded",
      "status_code":  200,
      "duration_ms":  145,
      "delivered_at": "2024-03-15T09:30:05Z"
    },
    {
      "id":          "del_018e1b2c-...",
      "event":       "finance_invoice.created",
      "attempt":     4,
      "status":      "failed",
      "status_code": 503,
      "error":       "Service Unavailable",
      "next_retry":  "2024-03-15T12:30:00Z"
    }
  ]
}
```

---

## 8. Security Constraints

- Webhook URLs MUST use HTTPS. HTTP URLs are rejected at subscription creation time.
- Webhook URLs MUST NOT be internal network addresses (RFC 1918, localhost, link-local). SSRF prevention is enforced by the relay before each delivery attempt.
- The `secret` is write-only — it is never returned in GET responses. It can be rotated via PATCH.
- Platform administrators can view all tenant webhook subscriptions; tenants can only see their own.

---

## Related Documents

- [Outbox Pattern](../09-workflow/outbox-pattern.md) — delivery mechanism
- [Hooks](../04-domain/hooks.md) — entity lifecycle events that trigger webhooks
- [Security Model](../15-security/security-model.md) — SSRF prevention
- [API Conventions](conventions.md) — response envelope
- [Glossary](../GLOSSARY.md) — Webhook, Outbox Pattern, HMAC
