# High Error Rate Runbook

**Classification:** Runbook — Tier 2
**Owner:** `19-operations/RUNBOOK_HIGH_ERROR_RATE.md`
**Trigger:** 5xx error rate > 1% over 5 minutes. PagerDuty alert: `awo-erp-high-error-rate`.

---

## 1. Quantify the Problem

```bash
# Count errors in last 15 minutes
kubectl logs -l app=awo-erp-server -n production --since=15m \
  | grep '"level":"error"' | wc -l

# Get error breakdown by path
kubectl logs -l app=awo-erp-server -n production --since=15m \
  | grep '"status":5' \
  | jq -r '"\(.status) \(.path)"' \
  | sort | uniq -c | sort -rn | head -20
```

---

## 2. Identify Error Type

```bash
# Show full error messages with request_id for tracing
kubectl logs -l app=awo-erp-server -n production --since=15m \
  | grep '"level":"error"' \
  | jq '{time: .time, path: .path, err: .err, request_id: .request_id}'
```

### 500 Internal Server Error

Application panic or unhandled error. Look for:
- `"err":"pq: ..."` → database error
- `"err":"context deadline exceeded"` → timeout
- `"err":"connection refused"` → upstream service down (Redis, Temporal)
- Stack trace with `goroutine` → panic recovered by middleware

### 503 Service Unavailable

Overload or upstream unavailable:
- Check Redis connectivity (session lookups failing → all authenticated requests fail)
- Check Temporal worker connectivity

### 504 Gateway Timeout

Request exceeded timeout:
- Check p99 latency in Prometheus
- Look for slow queries (see [`RUNBOOK_DATABASE.md`](RUNBOOK_DATABASE.md))

---

## 3. Check Specific Errors

### Database Errors

```bash
kubectl logs -l app=awo-erp-server -n production --since=15m \
  | grep -E '"err":".*pq:|pgx:|context deadline'
```

If DB errors → follow [`RUNBOOK_DATABASE.md`](RUNBOOK_DATABASE.md).

### Redis / Session Errors

```bash
kubectl logs -l app=awo-erp-server -n production --since=15m \
  | grep -E '"err":".*redis|session'

# Test Redis from pod
kubectl exec -it deploy/awo-erp-server -n production -- \
  redis-cli -u "$REDIS_URL" ping
```

### Validation Error Spike (422)

422s are not 5xx but a spike indicates a client-side issue (bad deploy of frontend, API schema change):

```bash
kubectl logs -l app=awo-erp-server -n production --since=15m \
  | grep '"status":422' \
  | jq -r '.path' | sort | uniq -c | sort -rn | head -10
```

---

## 4. Check Recent Deploys

```bash
kubectl rollout history deployment/awo-erp-server -n production

# Check when the error rate started vs deploy time
kubectl describe deployment awo-erp-server -n production \
  | grep -A5 "Events:"
```

If errors started immediately after a deploy → rollback is likely the right action.

---

## 5. Rollback Decision

Rollback if:
- Error rate > 5% and still increasing
- Errors started with a specific deploy
- Root cause not identifiable within 10 minutes

```bash
kubectl rollout undo deployment/awo-erp-server -n production
kubectl rollout status deployment/awo-erp-server -n production
```

Do **not** rollback if:
- Errors are from an external dependency (DB, Redis) — fix the dependency
- Error rate is steady and < 2% — investigate without rollback

---

## 6. Resolution Criteria

- 5xx error rate < 0.1% over a 5-minute window
- No new panics in logs
- Liveness and readiness probes passing
- p99 latency back to baseline (< 500ms)

---

## References

- [`19-operations/RUNBOOK_SERVICE_DOWN.md`](RUNBOOK_SERVICE_DOWN.md) — Pod-level issues
- [`19-operations/RUNBOOK_DATABASE.md`](RUNBOOK_DATABASE.md) — DB-level issues
- [`17-observability/METRICS_SPEC.md`](../17-observability/METRICS_SPEC.md) — Alerting thresholds
