# Awo Framework — Documentation

**Status:** Frozen at v1.0
**Architecture decisions:** [`00-overview/DECISION_REGISTER.md`](00-overview/DECISION_REGISTER.md)
**New module authors:** Start at [`99-modules/ONBOARDING_CHECKLIST.md`](99-modules/ONBOARDING_CHECKLIST.md)

---

## 00 — Overview

| Document | Description |
|----------|-------------|
| [`00-overview/ARCH_OVERVIEW.md`](00-overview/ARCH_OVERVIEW.md) | 5-layer architecture, startup sequence, package DAG |
| [`00-overview/DECISION_REGISTER.md`](00-overview/DECISION_REGISTER.md) | All 12 ADRs — constitutional, binding |
| [`00-overview/PRINCIPLES.md`](00-overview/PRINCIPLES.md) | Design principles governing all decisions |
| [`00-overview/GLOSSARY.md`](00-overview/GLOSSARY.md) | Every term defined |
| [`00-overview/PACKAGE_DEPENDENCY_MAP.md`](00-overview/PACKAGE_DEPENDENCY_MAP.md) | Package DAG with import rules |
| [`00-overview/V1_RELEASE_SNAPSHOT.md`](00-overview/V1_RELEASE_SNAPSHOT.md) | Historical record — architecture state at v1.0 freeze |

---

## 01 — Entity Primitives

| Document | Description |
|----------|-------------|
| [`01-entity/ENTITY_DEFINITION_SPEC.md`](01-entity/ENTITY_DEFINITION_SPEC.md) | EntityDefinition interface — all 14 methods |
| [`01-entity/FIELD_TYPES_REFERENCE.md`](01-entity/FIELD_TYPES_REFERENCE.md) | Every field type, PostgreSQL mapping, Go type |
| [`01-entity/EDGE_TYPES_REFERENCE.md`](01-entity/EDGE_TYPES_REFERENCE.md) | EdgeOneToMany / EdgeManyToOne semantics |
| [`01-entity/NAMING_CONVENTIONS.md`](01-entity/NAMING_CONVENTIONS.md) | Entity, field, and action naming rules |
| [`01-entity/SYSTEM_VS_CUSTOM.md`](01-entity/SYSTEM_VS_CUSTOM.md) | When to use system vs custom entity |

---

## 02 — Request Pipeline

| Document | Description |
|----------|-------------|
| [`02-pipeline/LIFECYCLE_SPEC.md`](02-pipeline/LIFECYCLE_SPEC.md) | All 9 pipeline stages, TX boundaries |
| [`02-pipeline/HOOK_CONTRACT.md`](02-pipeline/HOOK_CONTRACT.md) | Hook interfaces, execution order, recursion policy (ADR-010) |
| [`02-pipeline/ERROR_MODEL.md`](02-pipeline/ERROR_MODEL.md) | ValidationError, BusinessError, HTTP mapping |
| [`02-pipeline/PIPELINE_SEQUENCE.md`](02-pipeline/PIPELINE_SEQUENCE.md) | Sequence diagrams: CRUD, custom actions, list, error paths |

---

## 03 — Authorization

| Document | Description |
|----------|-------------|
| [`03-auth/AUTHORIZATION_SPEC.md`](03-auth/AUTHORIZATION_SPEC.md) | PermissionSet + PolicyEvaluator (ADR-001), ViewerContext (ADR-002) |
| [`03-auth/RBAC_ROLES_REFERENCE.md`](03-auth/RBAC_ROLES_REFERENCE.md) | System roles, role naming convention, role hierarchy |
| [`03-auth/SESSION_SPEC.md`](03-auth/SESSION_SPEC.md) | auth.Session struct (ADR-004), Redis storage, ToActor() |
| [`03-auth/ACTOR_SPEC.md`](03-auth/ACTOR_SPEC.md) | def.Actor struct (ADR-003), IsPlatformAdmin(), IsServiceAccount() |

---

## 04 — Multi-Tenancy

| Document | Description |
|----------|-------------|
| [`04-multitenancy/RLS_SPEC.md`](04-multitenancy/RLS_SPEC.md) | Row Level Security setup, set_tenant_context(), PgBouncer requirement |
| [`04-multitenancy/TENANT_LIFECYCLE.md`](04-multitenancy/TENANT_LIFECYCLE.md) | Tenant status machine, HTTP responses per status |
| [`04-multitenancy/TENANT_RESOLUTION.md`](04-multitenancy/TENANT_RESOLUTION.md) | Header / subdomain / query param resolution order |

---

## 05 — Registry and Compiler

| Document | Description |
|----------|-------------|
| [`05-registry/REGISTRY_SPEC.md`](05-registry/REGISTRY_SPEC.md) | EntityRegistry, registration, validation rules |
| [`05-registry/COMPILER_SPEC.md`](05-registry/COMPILER_SPEC.md) | CompiledSchema, CapabilityGrants (ADR-011), route derivation |

---

## 06 — Filter DSL

| Document | Description |
|----------|-------------|
| [`06-filter/FILTER_DSL_REFERENCE.md`](06-filter/FILTER_DSL_REFERENCE.md) | filter.Eq, And, Or, In, Gt, Lt, Contains — complete reference |
| [`06-filter/PRIVACY_POLICY_SPEC.md`](06-filter/PRIVACY_POLICY_SPEC.md) | PolicyFunc, built-in policies, row-level filtering |

---

## 07 — Naming Series

| Document | Description |
|----------|-------------|
| [`07-naming/NAMING_SERIES_SPEC.md`](07-naming/NAMING_SERIES_SPEC.md) | Pattern syntax, atomic counter, tenant overrides, reset policy |

---

## 08 — Workflows

| Document | Description |
|----------|-------------|
| [`08-workflow/TEMPORAL_INTEGRATION.md`](08-workflow/TEMPORAL_INTEGRATION.md) | Temporal setup, determinism rules, activity pattern, saga |
| [`08-workflow/OUTBOX_SPEC.md`](08-workflow/OUTBOX_SPEC.md) | workflow_outbox table schema (ADR-007), outbox worker |
| [`08-workflow/WORKFLOW_ID_CONVENTION.md`](08-workflow/WORKFLOW_ID_CONVENTION.md) | Workflow ID format, deduplication |

---

## 09 — Domain Events

| Document | Description |
|----------|-------------|
| [`09-events/EVENT_OUTBOX_SPEC.md`](09-events/EVENT_OUTBOX_SPEC.md) | event_outbox schema (ADR-008 — public contract), EventBroker interface |
| [`09-events/DOMAIN_EVENTS_REFERENCE.md`](09-events/DOMAIN_EVENTS_REFERENCE.md) | All published event topics, payloads, consumer guarantees |

---

## 10 — SDUI

| Document | Description |
|----------|-------------|
| [`10-sdui/WIDGET_TREE_SPEC.md`](10-sdui/WIDGET_TREE_SPEC.md) | widget.Node, NodeKind constants (ADR-006) |
| [`10-sdui/AMIS_RENDERER_SPEC.md`](10-sdui/AMIS_RENDERER_SPEC.md) | NodeKind→amis type mapping, schema caching, SDK constraint |
| [`10-sdui/PAGE_BUILDER_GUIDE.md`](10-sdui/PAGE_BUILDER_GUIDE.md) | PageBuilderSet, permission-gated elements, DataSource config |
| [`10-sdui/DARK_THEME.md`](10-sdui/DARK_THEME.md) | CSS custom property override — no built-in dark CSS |

---

## 11 — Cache

| Document | Description |
|----------|-------------|
| [`11-cache/CACHE_SPEC.md`](11-cache/CACHE_SPEC.md) | Cache + Counter interfaces, Redis implementations, TTL policy |
| [`11-cache/CACHE_KEY_REFERENCE.md`](11-cache/CACHE_KEY_REFERENCE.md) | All Redis key patterns and construction rules |

---

## 12 — Audit

| Document | Description |
|----------|-------------|
| [`12-audit/AUDIT_SPEC.md`](12-audit/AUDIT_SPEC.md) | Mandatory pipeline stage (ADR-005), audit_log schema, tamper evidence |
| [`12-audit/AUDIT_QUERY_PATTERNS.md`](12-audit/AUDIT_QUERY_PATTERNS.md) | SQL query patterns, Go QueryLog helper, retention policy |

---

## 13 — Custom Actions

| Document | Description |
|----------|-------------|
| [`13-actions/ACTION_HANDLER_GUIDE.md`](13-actions/ACTION_HANDLER_GUIDE.md) | ActionDef declaration, ActionContext, ActionResult |
| [`13-actions/ACTION_RUNTIME_REFERENCE.md`](13-actions/ACTION_RUNTIME_REFERENCE.md) | All 11 ActionRuntime methods, ActionEntityRepo (8 methods) |
| [`13-actions/CUSTOM_ACTIONS_EXAMPLES.md`](13-actions/CUSTOM_ACTIONS_EXAMPLES.md) | Finance module: submit, approve, cancel, record_payment |

---

## 14 — API

| Document | Description |
|----------|-------------|
| [`14-api/API_CONVENTIONS.md`](14-api/API_CONVENTIONS.md) | Routes, response envelope, HTTP status codes, serialisation rules |
| [`14-api/IDEMPOTENCY_SPEC.md`](14-api/IDEMPOTENCY_SPEC.md) | X-Idempotency-Key, Redis storage (ADR-009), fail-closed behaviour |
| [`14-api/PAGINATION_SPEC.md`](14-api/PAGINATION_SPEC.md) | limit/offset model, PageInfo, order expressions |
| [`14-api/ERROR_RESPONSE_FORMAT.md`](14-api/ERROR_RESPONSE_FORMAT.md) | Error envelope, code conventions, all status code scenarios |

---

## 15 — Migrations

| Document | Description |
|----------|-------------|
| [`15-migrations/MIGRATION_GUIDE.md`](15-migrations/MIGRATION_GUIDE.md) | golang-migrate, file naming, zero-downtime patterns |
| [`15-migrations/RLS_TABLE_TEMPLATE.md`](15-migrations/RLS_TABLE_TEMPLATE.md) | Copy-paste SQL template for every new tenant-scoped table |
| [`15-migrations/MIGRATION_CHECKLIST.md`](15-migrations/MIGRATION_CHECKLIST.md) | Sign-off checklist before every migration PR |

---

## 16 — Testing

| Document | Description |
|----------|-------------|
| [`16-testing/TEST_STRATEGY.md`](16-testing/TEST_STRATEGY.md) | Unit / integration / workflow layers, real PostgreSQL mandate |
| [`16-testing/HOOK_TEST_PATTERNS.md`](16-testing/HOOK_TEST_PATTERNS.md) | mockRepo, BeforeCreate/AfterCreate/BeforeUpdate patterns |
| [`16-testing/ACTION_TEST_PATTERNS.md`](16-testing/ACTION_TEST_PATTERNS.md) | mockRuntime (all 11 methods), table-driven action tests |
| [`16-testing/REGISTRY_TEST_PATTERNS.md`](16-testing/REGISTRY_TEST_PATTERNS.md) | Isolated registry, validation rules, CapabilityGrant output |

---

## 17 — Observability

| Document | Description |
|----------|-------------|
| [`17-observability/LOGGING_SPEC.md`](17-observability/LOGGING_SPEC.md) | slog JSON format, mandatory context fields, sensitive field exclusion |
| [`17-observability/METRICS_SPEC.md`](17-observability/METRICS_SPEC.md) | Prometheus metrics, histograms, alert thresholds |
| [`17-observability/HEALTH_CHECKS.md`](17-observability/HEALTH_CHECKS.md) | /health/live, /health/ready, Kubernetes probe config |

---

## 18 — Security

| Document | Description |
|----------|-------------|
| [`18-security/SECURITY_MODEL.md`](18-security/SECURITY_MODEL.md) | Threat model, 8 security layers, OWASP Top 10 coverage |
| [`18-security/SENSITIVE_FIELDS.md`](18-security/SENSITIVE_FIELDS.md) | Sensitive: true — SDUI, audit, log, API behaviour |
| [`18-security/SECRET_MANAGEMENT.md`](18-security/SECRET_MANAGEMENT.md) | Config struct, Vault/External Secrets, rotation table |

---

## 19 — Operations Runbooks

| Document | Description |
|----------|-------------|
| [`19-operations/RUNBOOK_INDEX.md`](19-operations/RUNBOOK_INDEX.md) | Service endpoints, common diagnostics, alerting thresholds |
| [`19-operations/RUNBOOK_SERVICE_DOWN.md`](19-operations/RUNBOOK_SERVICE_DOWN.md) | Service unavailable — 7-step diagnosis |
| [`19-operations/RUNBOOK_DATABASE.md`](19-operations/RUNBOOK_DATABASE.md) | Active queries, locks, pool exhaustion, RLS verification |
| [`19-operations/RUNBOOK_HIGH_ERROR_RATE.md`](19-operations/RUNBOOK_HIGH_ERROR_RATE.md) | Error rate spike — classify, identify, rollback decision |
| [`19-operations/RUNBOOK_MIGRATION_FAILURE.md`](19-operations/RUNBOOK_MIGRATION_FAILURE.md) | Dirty state, lock timeout, constraint violation, recovery |
| [`19-operations/RUNBOOK_TEMPORAL_WORKER.md`](19-operations/RUNBOOK_TEMPORAL_WORKER.md) | Stuck workflows, not polling, signal, bulk failed |
| [`19-operations/RUNBOOK_EVENT_OUTBOX.md`](19-operations/RUNBOOK_EVENT_OUTBOX.md) | Outbox backlog, failed events, replay, prevention |

---

## 20 — DevOps

| Document | Description |
|----------|-------------|
| [`20-devops/ENVIRONMENT_VARIABLES.md`](20-devops/ENVIRONMENT_VARIABLES.md) | Required / optional / Temporal / observability variables |
| [`20-devops/LOCAL_DEVELOPMENT.md`](20-devops/LOCAL_DEVELOPMENT.md) | docker-compose, first-time setup, air live reload, ports |
| [`20-devops/KUBERNETES_DEPLOYMENT.md`](20-devops/KUBERNETES_DEPLOYMENT.md) | Deployment, migration Job, PgBouncer, HPA, Ingress |
| [`20-devops/CICD_PIPELINE.md`](20-devops/CICD_PIPELINE.md) | GitHub Actions CI, Docker build, deploy sequence |
| [`20-devops/MONITORING.md`](20-devops/MONITORING.md) | Prometheus scrape, dashboard PromQL, alert rules, log aggregation |

---

## 99 — Module Authoring

| Document | Description |
|----------|-------------|
| [`99-modules/ONBOARDING_CHECKLIST.md`](99-modules/ONBOARDING_CHECKLIST.md) | 6-phase reading order + PR readiness checklist |
| [`99-modules/MODULE_AUTHOR_GUIDE.md`](99-modules/MODULE_AUTHOR_GUIDE.md) | Directory structure, registration, entity/hook/policy/action patterns |
| [`99-modules/FINANCE_MODULE_SPEC.md`](99-modules/FINANCE_MODULE_SPEC.md) | Canonical reference module — 10 entities, all patterns exemplified |

---

## Constitutional Source

[`ARCH_FREEZE_REVIEW.md`](ARCH_FREEZE_REVIEW.md) — Full ARB decision document with rationale and rejected alternatives for all 12 ADRs. Do not modify.
