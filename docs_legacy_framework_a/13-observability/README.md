> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

---
title: "Observability — Section Overview"
id: obs-000-readme
status: accepted
category: GUIDE
stability: STABLE
audience: [module-authors, operators]
since: "1.0"
normative-level: informative
related:
  - "[Observability](observability.md)"
  - "[Configuration](../12-configuration/configuration.md)"
  - "[Awo Glossary](../GLOSSARY.md)"
---

# Observability

**Section 13 | Observability**

Observability covers structured logging, Prometheus metrics, health endpoints, and distributed tracing. All three pillars are instrumented by the framework; modules extend them using the provided APIs.

---

## Contents

| Document | ID | Purpose | Stability |
|---|---|---|---|
| [Observability](observability.md) | OBS-001 | Logging standards, metrics catalog, health endpoints, tracing | STABLE |
| [Structured Logging Guide](structured-logging.md) | OBS-002 | Required log fields, levels, sensitive field exclusion, error logging | STABLE |
| [Metrics Reference](metrics-reference.md) | OBS-003 | Complete Prometheus metrics catalog, alert rules, Grafana panels | STABLE |
| [Alerting](alerting.md) | OBS-004 | Alert rules, severity levels, Alertmanager routing, runbook index | STABLE |
| [Distributed Tracing](tracing.md) | OBS-005 | OpenTelemetry instrumentation, span attributes, Jaeger/Tempo integration | STABLE |

---

## Prerequisites

- [Configuration](../12-configuration/configuration.md) — log level, metrics, tracing config
- [Startup Sequence](../03-kernel/startup-sequence.md) — health endpoint behavior at startup
- [Glossary](../GLOSSARY.md) — Structured Logging, Prometheus, Health Check
