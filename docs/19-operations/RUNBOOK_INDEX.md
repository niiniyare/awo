# Operations Runbook Index

**Classification:** Reference — Tier 2
**Owner:** `19-operations/RUNBOOK_INDEX.md`
**Status:** Living document

---

## Service Endpoints

| Endpoint | Port | Purpose |
|----------|------|---------|
| HTTP API | 8080 | All API traffic |
| Health probes | 8081 | Kubernetes liveness/readiness |
| Metrics | 9090 | Prometheus scrape |
| Temporal UI | 8088 | Workflow monitoring (internal) |

---

## Runbooks Index

| Runbook | When to Use |
|---------|------------|
| [`RUNBOOK_SERVICE_DOWN.md`](RUNBOOK_SERVICE_DOWN.md) | HTTP server not responding, pods crashing |
| [`RUNBOOK_DATABASE.md`](RUNBOOK_DATABASE.md) | DB connection errors, slow queries, pool exhaustion |
| [`RUNBOOK_HIGH_ERROR_RATE.md`](RUNBOOK_HIGH_ERROR_RATE.md) | 5xx rate above threshold |
| [`RUNBOOK_MIGRATION_FAILURE.md`](RUNBOOK_MIGRATION_FAILURE.md) | Migration job fails on deploy |
| [`RUNBOOK_TEMPORAL_WORKER.md`](RUNBOOK_TEMPORAL_WORKER.md) | Workflows not progressing |
| [`RUNBOOK_EVENT_OUTBOX.md`](RUNBOOK_EVENT_OUTBOX.md) | Event outbox backlog growing |

---

## Common Diagnostic Commands

### Check server health

```bash
curl http://{host}:8081/health/live
curl http://{host}:8081/health/ready
```

### View recent logs

```bash
kubectl logs -l app=awo-erp-server --tail=100 -n production
kubectl logs -l app=awo-erp-server --tail=100 -n production | grep '"level":"error"'
```

### Check database connections

```bash
kubectl exec -it deploy/awo-erp-server -- curl localhost:9090/metrics | grep db_connections
```

### Check event outbox backlog

```sql
-- Current pending count
SELECT COUNT(*) FROM event_outbox WHERE status = 'pending';

-- By topic with oldest message
SELECT topic, COUNT(*), MIN(created_at) AS oldest
FROM event_outbox
WHERE status = 'pending'
GROUP BY topic
ORDER BY oldest;
```

### Force session revoke (security incident)

```bash
# Revoke all sessions for a user
awoctl sessions revoke-all --user-id {uuid}

# Revoke all sessions for a tenant
awoctl sessions revoke-tenant --tenant-id {uuid}
```

---

## Alerting Thresholds

| Alert | Threshold | Severity |
|-------|-----------|---------|
| 5xx error rate | > 1% over 5 min | P2 |
| p99 latency | > 2s over 5 min | P2 |
| DB pool exhaustion | < 2 idle connections | P1 |
| Readiness probe failing | > 2 consecutive | P1 |
| Event outbox backlog | > 500 pending | P3 |
| Workflow outbox backlog | > 100 pending | P2 |
| Temporal workflow failures | > 0 for critical workflows | P2 |

---

## Escalation Path

1. On-call SRE (PagerDuty rotation)
2. Backend lead
3. Database admin (for data issues)
4. CTO (for data breach or extended P1)

---

## References

- [`17-observability/HEALTH_CHECKS.md`](../17-observability/HEALTH_CHECKS.md) — Health check contracts
- [`17-observability/METRICS_SPEC.md`](../17-observability/METRICS_SPEC.md) — Alerting thresholds
