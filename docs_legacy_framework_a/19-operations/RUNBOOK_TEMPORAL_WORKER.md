> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# Temporal Worker Stuck Runbook

**Classification:** Runbook — Tier 2
**Owner:** `19-operations/RUNBOOK_TEMPORAL_WORKER.md`
**Trigger:** Workflows not progressing, Temporal UI shows workflows stuck in `Running`, or alert `awo-erp-temporal-workflow-failures`.

---

## 1. Access Temporal UI

```bash
# Port-forward Temporal UI (if not exposed via VPN)
kubectl port-forward svc/temporal-ui 8088:8088 -n production
# Open http://localhost:8088
```

In the UI:
- Check **Running** workflows — are they stuck on an activity?
- Check **Failed** workflows — what error?
- Check **Timed Out** workflows — activity or workflow timeout?

---

## 2. Check Worker Logs

```bash
# Workers run in the same pod as the API server
kubectl logs -l app=awo-erp-server -n production --since=30m \
  | grep -E '"module":"temporal|worker|activity|workflow"'

# Look for worker connection errors
kubectl logs -l app=awo-erp-server -n production --since=30m \
  | grep -E 'temporal.*error|unable to connect|dial tcp'
```

---

## 3. Diagnose Stuck Workflows

### Workers Not Polling

If no new workflow tasks are being picked up:

```bash
# Check Temporal server connectivity from app pod
kubectl exec -it deploy/awo-erp-server -n production -- \
  nc -zv temporal-frontend 7233
```

If connection refused → Temporal server is down. Check Temporal deployment:

```bash
kubectl get pods -n temporal
kubectl logs -l app=temporal -n temporal --tail=50
```

### Activity Failing Repeatedly

In Temporal UI, open a stuck workflow → click the failing activity → view the last error.

Common causes:
- DB connection error inside activity → transient, will auto-retry
- Business rule violation with retryable error → should be non-retryable, fix code
- Activity timeout too short for workload → increase `StartToCloseTimeout`

### Workflow Stuck on Signal

A workflow awaiting a signal (e.g., `ApprovalDecision`) will sit in `Running` indefinitely until signaled. This is normal behaviour — verify with business team whether the workflow is legitimately waiting for approval.

```bash
# Using tctl
tctl wf describe --workflow-id {workflow-id}
```

If the signal should have been sent but wasn't, the service that sends the signal failed silently. Check the HTTP handler logs around the time of the action.

---

## 4. Manually Signal a Stuck Workflow

If a workflow is stuck waiting for a signal that was lost:

```bash
# Send approval signal manually
tctl wf signal \
  --workflow-id {workflow-id} \
  --name approval-decision \
  --input '{"approved": true, "comment": "Manual recovery", "approver_id": "..."}'
```

**Confirm with the business team before sending manual signals** — this triggers real state changes.

---

## 5. Terminate and Restart a Workflow

If a workflow is in a bad state and cannot recover:

```bash
# Terminate workflow
tctl wf terminate \
  --workflow-id {workflow-id} \
  --reason "Manual recovery: stuck workflow"
```

Then restart from the workflow outbox:

```sql
-- Check if workflow outbox has a pending dispatch for this workflow
SELECT * FROM workflow_outbox
WHERE workflow_id = '{workflow-id}'
ORDER BY created_at DESC LIMIT 5;
```

---

## 6. Bulk Failed Workflows

If many workflows failed due to a deployment issue:

```bash
# List failed workflows in Temporal UI or via tctl
tctl wf list --status Failed --query 'WorkflowType="InvoiceApprovalWorkflow"'
```

After fixing the root cause, re-run failed workflows by re-triggering the entity action or manually writing to `workflow_outbox`.

---

## 7. Worker Registration Check

If no workers are polling:

```bash
# Verify task queues have registered workers
tctl tq describe --task-queue finance.invoice.approval
tctl tq describe --task-queue finance.invoice.submit
```

Look for `pollers` in the output. If empty → workers are not connecting. Restart pods:

```bash
kubectl rollout restart deployment/awo-erp-server -n production
```

---

## Resolution Criteria

- Temporal UI shows workflows progressing (no new stuck ones appearing)
- `tctl tq describe` shows active pollers on all task queues
- No `workflow_task_failed` errors in worker logs
- Failed workflow count returns to 0 for critical workflow types

---

## References

- [`08-workflow/TEMPORAL_INTEGRATION.md`](../08-workflow/TEMPORAL_INTEGRATION.md) — Workflow determinism rules
- [`08-workflow/OUTBOX_SPEC.md`](../08-workflow/OUTBOX_SPEC.md) — Workflow outbox schema
