# Service Down Runbook

**Classification:** Runbook — Tier 2
**Owner:** `19-operations/RUNBOOK_SERVICE_DOWN.md`
**Use when:** Pods are crashing, health checks failing, service unreachable, or sustained 5xx rate.

---

## Step 1 — Assess Scope

```bash
# Check pod status
kubectl get pods -n awo-erp

# Check recent events
kubectl get events -n awo-erp --sort-by='.lastTimestamp' | tail -20

# Check service endpoints
kubectl get endpoints -n awo-erp awo-erp-svc
```

Determine: is this one pod, all pods, or a specific deployment?

---

## Step 2 — Check Logs

```bash
# Logs from crashing pod
kubectl logs -n awo-erp <pod-name> --previous

# Logs from all pods (last 5 min)
kubectl logs -n awo-erp -l app=awo-erp --since=5m

# If pod keeps restarting, look for OOM
kubectl describe pod -n awo-erp <pod-name> | grep -A5 "Last State"
```

Common crash patterns:

| Log Pattern | Likely Cause |
|-------------|-------------|
| `failed to connect to database` | DB unreachable or credentials wrong |
| `failed to connect to Redis` | Redis down or network issue |
| `OOMKilled` | Memory limit too low — increase or find leak |
| `panic:` | Application bug — check stack trace |
| `address already in use` | Port conflict in pod |
| `EntityRegistry: fatal: duplicate entity name` | Module registration bug |

---

## Step 3 — Check Dependencies

```bash
# Check if DB is reachable
kubectl exec -n awo-erp <pod-name> -- nc -zv <db-host> 5432

# Check Redis
kubectl exec -n awo-erp <pod-name> -- nc -zv <redis-host> 6379

# Check Temporal server
kubectl exec -n awo-erp <pod-name> -- nc -zv <temporal-host> 7233
```

---

## Step 4 — Check Health Endpoints

```bash
# Liveness
curl -sf https://<host>/health/live && echo "OK"

# Readiness (checks DB + Redis + EntityRegistry)
curl -sf https://<host>/health/ready && echo "OK"
curl https://<host>/health/ready | jq .
```

If `/health/ready` fails but `/health/live` passes: dependency issue (DB, Redis, or registry not populated).
If both fail: application startup failed.

---

## Step 5 — Check Recent Deployments

```bash
# Recent rollout history
kubectl rollout history deployment/awo-erp -n awo-erp

# If recent deploy broke it — rollback
kubectl rollout undo deployment/awo-erp -n awo-erp

# Verify rollback complete
kubectl rollout status deployment/awo-erp -n awo-erp
```

---

## Step 6 — Scale / Restart

If pods are stuck in `CrashLoopBackOff` after fixing root cause:

```bash
# Force restart all pods
kubectl rollout restart deployment/awo-erp -n awo-erp

# Or manually delete stuck pod (Kubernetes recreates it)
kubectl delete pod -n awo-erp <pod-name>
```

---

## Step 7 — Escalation

If not resolved in 30 minutes:

1. **P0/P1**: Page on-call backend lead
2. Check [`RUNBOOK_TEMPORAL_WORKER.md`](RUNBOOK_TEMPORAL_WORKER.md) if workflows are affected
3. Check [`RUNBOOK_DATABASE.md`](RUNBOOK_DATABASE.md) if DB connectivity is suspect
4. Pull recent application metrics from Grafana (`awo-erp` dashboard)

---

## Post-Incident

After service restored:
1. Write incident timeline in incident tracker
2. Identify root cause
3. Open follow-up ticket if code/config change needed
4. Update this runbook if a new pattern was discovered
