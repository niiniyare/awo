# Monitoring

**Classification:** Guide — Tier 2
**Owner:** `20-devops/MONITORING.md`
**Status:** Living document

---

## Prometheus Scrape Config

```yaml
# prometheus.yml
scrape_configs:
  - job_name: awo-erp
    kubernetes_sd_configs:
    - role: pod
    relabel_configs:
    - source_labels: [__meta_kubernetes_pod_label_app]
      action: keep
      regex: awo-erp-server
    - source_labels: [__meta_kubernetes_pod_ip]
      target_label: __address__
      replacement: '${1}:9090'
    - source_labels: [__meta_kubernetes_namespace]
      target_label: namespace
    - source_labels: [__meta_kubernetes_pod_name]
      target_label: pod
```

---

## Key Dashboards

### Request Rate & Latency

```promql
# Request rate (req/sec)
rate(http_request_duration_seconds_count[5m])

# p99 latency
histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))

# Error rate
rate(http_request_duration_seconds_count{status_code=~"5.."}[5m])
/ rate(http_request_duration_seconds_count[5m])
```

### Database

```promql
# p95 query duration
histogram_quantile(0.95, rate(db_query_duration_seconds_bucket[5m]))

# Connection pool utilisation
db_connections_open / (db_connections_open + db_connections_idle)
```

### Outbox Health

```promql
# Event outbox pending
event_outbox_pending

# Workflow outbox pending
workflow_outbox_pending

# Event delivery failures
rate(event_delivery_failed_total[5m])
```

---

## Alert Rules

```yaml
# rules/awo-erp.yml
groups:
- name: awo-erp
  rules:
  - alert: HighErrorRate
    expr: |
      rate(http_request_duration_seconds_count{status_code=~"5.."}[5m])
      / rate(http_request_duration_seconds_count[5m]) > 0.01
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: "High error rate on awo-erp"
      description: "5xx error rate > 1% for 2 minutes"

  - alert: HighLatency
    expr: |
      histogram_quantile(0.99,
        rate(http_request_duration_seconds_bucket[5m])
      ) > 2
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High p99 latency on awo-erp"

  - alert: EventOutboxBacklog
    expr: event_outbox_pending > 500
    for: 10m
    labels:
      severity: warning
    annotations:
      summary: "Event outbox backlog > 500"

  - alert: WorkflowOutboxBacklog
    expr: workflow_outbox_pending > 100
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "Workflow outbox backlog > 100 — Temporal worker may be down"

  - alert: ReadinessProbeFailing
    expr: kube_pod_status_ready{namespace="production",condition="true"} == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "awo-erp pods not ready"
```

---

## Log Aggregation

Forward pod logs to your log aggregation system (Loki, CloudWatch, Datadog):

```yaml
# Fluent Bit DaemonSet (example)
[FILTER]
    Name    grep
    Match   kube.*awo-erp-server*
    Regex   log .*

[OUTPUT]
    Name        loki
    Match       kube.*awo-erp-server*
    Host        loki.monitoring.svc.cluster.local
    Labels      app=awo-erp, namespace=$kubernetes['namespace_name']
```

Key log queries (Loki):

```logql
# Error logs
{app="awo-erp"} | json | level = "error"

# Slow requests (> 1000ms)
{app="awo-erp"} | json | duration_ms > 1000

# By tenant
{app="awo-erp"} | json | tenant_id = "{uuid}"
```

---

## References

- [`17-observability/METRICS_SPEC.md`](../17-observability/METRICS_SPEC.md) — Metric definitions
- [`17-observability/HEALTH_CHECKS.md`](../17-observability/HEALTH_CHECKS.md) — Health check contracts
- [`19-operations/RUNBOOK_INDEX.md`](../19-operations/RUNBOOK_INDEX.md) — Runbooks
