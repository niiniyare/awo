# Event Outbox Backlog Runbook

**Classification:** Runbook — Tier 2
**Owner:** `19-operations/RUNBOOK_EVENT_OUTBOX.md`
**Trigger:** `event_outbox_pending > 500` alert, or consumers not receiving events.

---

## 1. Check Current Backlog

```sql
-- Total pending
SELECT COUNT(*) AS pending
FROM event_outbox
WHERE status = 'pending';

-- Breakdown by topic with oldest message
SELECT topic,
       COUNT(*) AS count,
       MIN(created_at) AS oldest,
       MAX(created_at) AS newest
FROM event_outbox
WHERE status = 'pending'
GROUP BY topic
ORDER BY oldest;

-- Failed events (exhausted retries)
SELECT topic, COUNT(*), MIN(created_at) AS oldest
FROM event_outbox
WHERE status = 'failed'
GROUP BY topic
ORDER BY oldest;
```

A backlog growing steadily means the outbox worker has stopped or the EventBroker is failing.

---

## 2. Check Outbox Worker Logs

```bash
kubectl logs -l app=awo-erp-server -n production --since=30m \
  | grep '"module":"outbox"'
```

Look for:
- `"msg":"outbox: publish error"` → EventBroker hitting errors
- No outbox logs at all → outbox goroutine may have panicked
- Exponential backoff messages → worker is retrying but failing

---

## 3. Check EventBroker Connectivity

The `EventBroker` implementation (Kafka, NATS, Redis Pub/Sub, or Noop) connects to an external broker. Verify it is reachable:

```bash
# If using Redis Pub/Sub broker
kubectl exec -it deploy/awo-erp-server -n production -- \
  redis-cli -u "$REDIS_URL" ping

# If using Kafka broker
kubectl exec -it deploy/awo-erp-server -n production -- \
  nc -zv <kafka-host> 9092
```

If the broker is down → fix the broker first, then the backlog will drain automatically once the worker reconnects.

---

## 4. Check Retry Status

The outbox worker uses exponential backoff: 1s, 2s, 4s... capped at 5 minutes.

```sql
-- Events with high retry counts
SELECT topic, attempts, last_error, deliver_at, created_at
FROM event_outbox
WHERE status = 'pending'
  AND attempts > 3
ORDER BY attempts DESC, deliver_at
LIMIT 20;
```

Events with `attempts > 10` and `deliver_at` far in the future → broker connection issue, not transient error.

---

## 5. Check Failed Events (24h timeout)

Events that exceeded the 24-hour delivery window have `status = 'failed'`:

```sql
-- Failed events by topic
SELECT topic, COUNT(*), MAX(last_error) AS sample_error
FROM event_outbox
WHERE status = 'failed'
GROUP BY topic;
```

Failed events require manual intervention — they are NOT automatically retried. See §7 for replay.

---

## 6. Restart Outbox Worker

The outbox worker runs inside the API server process. Restart the pods:

```bash
kubectl rollout restart deployment/awo-erp-server -n production
kubectl rollout status deployment/awo-erp-server -n production
```

Monitor backlog drain:

```sql
-- Run every 30 seconds to watch drain rate
SELECT COUNT(*) AS pending, now() AS checked_at
FROM event_outbox
WHERE status = 'pending';
```

---

## 7. Replay Failed Events

To retry events with `status = 'failed'`, reset them to `pending`:

```sql
-- Preview what will be replayed
SELECT id, topic, payload, attempts, last_error, created_at
FROM event_outbox
WHERE status = 'failed'
  AND topic = 'finance.invoice.submitted'
  AND created_at >= now() - interval '24 hours'
LIMIT 10;

-- Reset to pending for retry (get DBA sign-off before running)
UPDATE event_outbox
SET status = 'pending',
    attempts = 0,
    last_error = NULL,
    deliver_at = now()
WHERE status = 'failed'
  AND topic = 'finance.invoice.submitted'
  AND created_at >= now() - interval '24 hours';
```

**Get DBA sign-off before running UPDATE on `event_outbox`.** Verify the consumer is idempotent before replaying — consumers MUST be idempotent (at-least-once delivery guarantee).

---

## 8. Prevent Accumulation

Large backlogs indicate the worker is consistently behind. Long-term fixes:
- Increase `OUTBOX_RELAY_BATCH_SIZE` config (default: 100)
- Increase `OUTBOX_RELAY_INTERVAL` to poll more frequently
- If volume is extremely high, consider switching high-volume topics to a more scalable EventBroker (Kafka vs Redis Pub/Sub)

---

## Resolution Criteria

- `SELECT COUNT(*) FROM event_outbox WHERE status = 'pending'` < 100
- No new messages older than 60 seconds in the pending backlog
- Outbox worker logs show successful deliveries
- Consumer-side lag (Kafka consumer group, NATS subscription) is draining

---

## References

- [`09-events/EVENT_OUTBOX_SPEC.md`](../09-events/EVENT_OUTBOX_SPEC.md) — Outbox schema and worker spec
- [`09-events/DOMAIN_EVENTS_REFERENCE.md`](../09-events/DOMAIN_EVENTS_REFERENCE.md) — Topic catalog
