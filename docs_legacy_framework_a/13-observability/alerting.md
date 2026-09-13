> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Alerting"
id: obs-004
status: accepted
category: GUIDE
stability: STABLE
audience: [operators, framework-authors]
since: "1.0"
normative-level: informative
related:
  - "[Metrics Reference](metrics-reference.md)"
  - "[Structured Logging](structured-logging.md)"
  - "[Troubleshooting](../14-operations/troubleshooting.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Alerting

**OBS-004 | Status: Accepted | Stability: Stable**

Alert rules, severity levels, and escalation paths for Awo deployments.

---

## 1. Severity Levels

| Level | Response time | Channel |
|---|---|---|
| P1 — Critical | Immediate (15 min) | PagerDuty / phone call |
| P2 — High | 1 hour | Slack #alerts-high |
| P3 — Medium | Business hours | Slack #alerts-medium |
| P4 — Low | Next sprint | JIRA ticket |

---

## 2. Core Alert Rules

### P1 — Service Down

```yaml
- alert: AwoServiceDown
  expr: up{job="awo-api"} == 0
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "Awo API instance down"
    runbook: "https://docs.internal/runbooks/service-down"
```

### P1 — Redis Unavailable (Auth Broken)

```yaml
- alert: RedisAuthUnavailable
  expr: awo_cache_operations_total{result="error", key_type="session"} > 0
  for: 30s
  labels:
    severity: critical
  annotations:
    summary: "Redis session store unreachable — all auth failing"
```

### P1 — High Error Rate

```yaml
- alert: HighErrorRate
  expr: |
    sum(rate(awo_http_requests_total{status=~"5.."}[5m]))
    /
    sum(rate(awo_http_requests_total[5m])) > 0.05
  for: 2m
  labels:
    severity: critical
  annotations:
    summary: "Error rate above 5%"
```

### P2 — Elevated Latency

```yaml
- alert: HighP99Latency
  expr: |
    histogram_quantile(0.99,
      sum(rate(awo_http_request_duration_seconds_bucket[5m])) by (le)
    ) > 2.0
  for: 5m
  labels:
    severity: high
  annotations:
    summary: "P99 latency above 2s"
```

### P2 — Temporal Worker Down

```yaml
- alert: TemporalWorkerDown
  expr: |
    sum(rate(awo_workflow_started_total[10m])) == 0
    AND
    sum(rate(awo_http_requests_total[10m])) > 0
  for: 10m
  labels:
    severity: high
  annotations:
    summary: "Temporal worker not processing — workflow dispatch stacking in outbox"
```

### P2 — Outbox Backlog

```yaml
- alert: OutboxBacklogGrowing
  expr: awo_outbox_pending_total > 100
  for: 5m
  labels:
    severity: high
  annotations:
    summary: "Outbox events not being dispatched — Temporal worker may be down"
```

### P3 — DB Connection Pool Saturation

```yaml
- alert: DBPoolNearSaturation
  expr: |
    awo_db_pool_connections{state="idle"} /
    (awo_db_pool_connections{state="idle"} + awo_db_pool_connections{state="active"}) < 0.1
  for: 10m
  labels:
    severity: medium
  annotations:
    summary: "DB connection pool < 10% idle — risk of connection starvation"
```

### P3 — Slow DB Queries

```yaml
- alert: SlowDBQueries
  expr: |
    histogram_quantile(0.95,
      sum(rate(awo_db_query_duration_seconds_bucket[10m])) by (le, entity, operation)
    ) > 1.0
  for: 10m
  labels:
    severity: medium
  annotations:
    summary: "95th percentile DB query > 1s"
```

### P3 — High Authentication Failure Rate

```yaml
- alert: HighAuthFailureRate
  expr: |
    sum(rate(awo_iam_auth_failures_total[10m])) > 10
  for: 5m
  labels:
    severity: medium
  annotations:
    summary: "High rate of authentication failures — possible brute force"
```

### P3 — Rate Limit Exceeded Frequently

```yaml
- alert: FrequentRateLimitExceeded
  expr: |
    sum(rate(awo_iam_rate_limit_exceeded_total[10m])) by (tenant_id) > 50
  for: 10m
  labels:
    severity: medium
  annotations:
    summary: "Tenant {{ $labels.tenant_id }} hitting rate limits frequently"
```

### P4 — Workflow Failures

```yaml
- alert: WorkflowFailures
  expr: sum(rate(awo_workflow_failed_total[1h])) by (workflow_type) > 5
  for: 30m
  labels:
    severity: low
  annotations:
    summary: "Workflow {{ $labels.workflow_type }} failing repeatedly"
```

---

## 3. Business-Level Alerts

These fire based on domain metrics, not infrastructure:

```yaml
- alert: PayrollRunStuck
  expr: |
    awo_entity_operations_total{entity="payroll_run",operation="create"}
    AND
    awo_workflow_completed_total{workflow_type="PayrollProcessingWorkflow"} == 0
  for: 60m
  labels:
    severity: high
  annotations:
    summary: "Payroll run created but no workflow completed in 60 minutes"

- alert: TenantProvisioningStuck
  expr: |
    awo_workflow_started_total{workflow_type="TenantProvisioningWorkflow"}
    - awo_workflow_completed_total{workflow_type="TenantProvisioningWorkflow"} > 5
  for: 30m
  labels:
    severity: high
  annotations:
    summary: "5+ tenant provisioning workflows in progress — possible stuck state"
```

---

## 4. Alertmanager Routing

```yaml
# alertmanager.yml
route:
  group_by: [alertname, severity]
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h
  receiver: slack-medium

  routes:
  - match:
      severity: critical
    receiver: pagerduty
    continue: true

  - match:
      severity: high
    receiver: slack-high

receivers:
- name: pagerduty
  pagerduty_configs:
  - service_key: ${PAGERDUTY_KEY}

- name: slack-high
  slack_configs:
  - api_url: ${SLACK_WEBHOOK_HIGH}
    channel: '#alerts-high'
    title: '[{{ .Status | toUpper }}] {{ .GroupLabels.alertname }}'
    text: '{{ range .Alerts }}{{ .Annotations.summary }}{{ end }}'

- name: slack-medium
  slack_configs:
  - api_url: ${SLACK_WEBHOOK_MEDIUM}
    channel: '#alerts-medium'
```

---

## 5. Runbook Links

Each alert annotation includes a `runbook` URL pointing to the relevant troubleshooting document:

| Alert | Runbook |
|---|---|
| AwoServiceDown | [Troubleshooting §1](../14-operations/troubleshooting.md#503) |
| RedisAuthUnavailable | [Troubleshooting §7](../14-operations/troubleshooting.md#redis) |
| HighErrorRate | [Troubleshooting §2](../14-operations/troubleshooting.md#errors) |
| TemporalWorkerDown | [Troubleshooting §5](../14-operations/troubleshooting.md#temporal) |
| OutboxBacklogGrowing | [Troubleshooting §5](../14-operations/troubleshooting.md#temporal) |

---

## Related Documents

- [Metrics Reference](metrics-reference.md) — full metric definitions used in alert expressions
- [Structured Logging](structured-logging.md) — log-based alerting for events not captured in metrics
- [Troubleshooting](../14-operations/troubleshooting.md) — symptom-first diagnosis
- [Performance Tuning](../14-operations/performance-tuning.md) — remediation for latency and pool alerts
