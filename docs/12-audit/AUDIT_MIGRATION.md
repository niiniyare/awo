# Audit Migration Strategy

**Classification:** Specification — Tier 1
**Owner:** `12-audit/AUDIT_MIGRATION.md`
**Status:** Approved — v1.0
**ADR:** ADR-019

---

## Purpose

Specifies the migration strategy from the legacy dual audit system (`iam_audit_log` Go entity + `audit_log` SQL trigger table) to the unified `platform_audit_log`. Covers migration phases, feature flag gating, rollback procedure, and cleanup steps.

---

## 1. Legacy System Inventory

| Component | Location | Status |
|-----------|----------|--------|
| `iam_audit_log` Go entity | `awo/platform/audit/definition.go` | **Retire** |
| `audit_log` SQL table | migrations 000448 | **Retire** |
| SQL trigger `audit_trigger_function()` | migration 000451 | **Remove** |
| `enable_audit_on_table()` function | migration 000451 | **Remove** |
| `enable_audit_on_schema()` function | migration 000451 | **Remove** |
| `disable_audit_on_table()` function | migration 000451 | **Remove** |
| `get_audit_statistics()` function | migration 000451 | **Remove** |
| `get_current_user_context()` function | migration 000451 | **Remove** |
| `calculate_audit_risk_score()` function | migration 000451 | **Remove** |
| `extract_compliance_flags()` function | migration 000451 | **Remove** |
| `determine_event_category()` function | migration 000451 | **Remove** |
| `determine_severity()` function | migration 000451 | **Remove** |
| `iam.audit_log.read` permission | Casbin / iam_role_permissions | **Replace** with `platform.audit_log.read` |
| Side-effect import `awo/platform/audit` | `awo/cmd/server/main.go` | **Update** |

---

## 2. Migration Phases

### Phase 1: Infrastructure Creation (no-op to application)

**Migration number:** 000452 (or next available)

**Actions:**
1. Create `platform_audit_log` partitioned table (base + current month + 2 months ahead)
2. Create all indexes on parent table
3. Create `platform_audit_checkpoint` table
4. Create `platform_audit_config` feature flag table
5. Create `platform_audit_migration_log` table
6. Seed feature flag: `feature.unified_audit.enabled = 'false'`
7. Grant permissions: `awo_app` gets INSERT + SELECT; `audit_retention_role` gets UPDATE + DELETE
8. Seed `platform.audit_log.read` permission into `iam_role_permissions` (alongside existing `iam.audit_log.read` — both active during transition)

**Gating condition:** Unconditional — runs as part of normal migration sequence.

**Rollback:** Drop tables created in this phase. No application behavior changes.

**Validation:**
- `\d platform_audit_log` shows partitioned table structure
- `SELECT * FROM platform_audit_config WHERE key = 'feature.unified_audit.enabled'` returns `value = 'false'`

---

### Phase 2: Deploy TransactionalWriter (flag-gated no-op)

**Code changes:**
1. Create `awo/audit/` package with `AuditWriter`, `AuditRecord`, `TransactionalWriter`, `Sanitizer`, `RiskScorer`, `EntityAuditConfig` registry
2. Add `AuditWriter` injection to `awo/runtime.Pipeline`
3. Update `awo/bootstrap/bootstrap.go` to construct `TransactionalWriter` and warm `RiskScorer`
4. `TransactionalWriter.Write()` checks feature flag on first call; returns nil (no-op) if flag = false
5. Register `EntityAuditConfig` for all existing entities from their `init()` functions

**Gating condition:** Feature flag `feature.unified_audit.enabled = 'false'`. Legacy system remains sole active audit path.

**Rollback:** Revert code changes. Feature flag still false — no behavioral change.

**Validation:**
- Application starts without errors
- `audit_write_total` metric is emitted but all writes are no-ops (zero actual rows in `platform_audit_log`)
- Legacy `iam_audit_log` and `audit_log` still receiving records

---

### Phase 3: Enable Unified System

**Action:** Set feature flag to true:

```sql
UPDATE platform_audit_config
SET value = 'true', updated_at = now()
WHERE key = 'feature.unified_audit.enabled';
```

**Effect:** `TransactionalWriter.Write()` now executes real INSERTs into `platform_audit_log`. Legacy system (SQL triggers + `iam_audit_log` entity) continues running in parallel during this phase.

**Gating condition:** Phase 2 validation complete. Both audit paths active simultaneously.

**Rollback:** Set flag back to false. `platform_audit_log` rows written during Phase 3 remain but are ignored.

**Validation (minimum 24 hours before proceeding to Phase 4):**
- New records appearing in `platform_audit_log`
- `audit_write_failure_total` counter is zero
- `audit_write_duration_seconds` p99 < 5ms
- Cross-reference: entity mutations appear in both `platform_audit_log` and `audit_log` for the same record
- `platform_audit_log.risk_score` and `severity` populated correctly for test mutations

---

### Phase 4: Historical Migration

**Background job:** Runs as a Temporal workflow (or equivalent scheduled job). Non-blocking to application.

**Source tables:** `iam_audit_log`, `audit_log`

**Mapping:**

| Legacy Column | `platform_audit_log` Column | Notes |
|---------------|---------------------------|-------|
| `iam_audit_log.id` | `id` | Direct |
| `iam_audit_log.tenant_id_ref` | `tenant_id` | Cast to UUID |
| `iam_audit_log.actor_id` | `actor_id` | Direct |
| `iam_audit_log.actor_email` | `context->'actor_email'` | JSONB context field |
| `iam_audit_log.ip_address` | `ip_address` | Direct |
| `iam_audit_log.operation` | `operation` | Direct |
| `iam_audit_log.entity_name` | `entity_name` | Direct |
| `iam_audit_log.record_id` | `record_id` | Cast to UUID |
| `iam_audit_log.before_snapshot` | `before_data` | Direct |
| `iam_audit_log.after_snapshot` | `after_data` | Direct |
| `iam_audit_log.diff` | `changed_fields` | Extract keys from diff JSONB |
| `iam_audit_log.request_id` | `request_id` | Direct |
| `iam_audit_log.created_at` | `created_at` | Direct |
| — | `event_category` | Default 'DATA' for legacy records |
| — | `severity` | Default 'INFO' for legacy records |
| — | `risk_score` | Default 0 for legacy records |
| `audit_log.id` | `id` | Direct |
| `audit_log.table_name` | `entity_name` | Map to qualified name |
| `audit_log.operation` | `operation` | Direct |
| `audit_log.old_data` | `before_data` | Direct |
| `audit_log.new_data` | `after_data` | Direct |
| `audit_log.created_at` | `created_at` | Direct |

**Progress tracking:** `platform_audit_migration_log` records last migrated ID per source table. Job is restartable from last checkpoint.

**Gating condition:** Phase 3 stable for ≥ 24 hours.

**Rollback:** Stop the migration job. Historical records remain in `platform_audit_log` (acceptable — they are duplicates of legacy records, not incorrect).

---

### Phase 5: Remove Legacy Infrastructure

**Gating condition:** Phase 4 complete (all historical records migrated), Phase 3 stable for ≥ 7 days.

**Migration number:** 000453 (or next available after 000452)

**Actions:**
1. Drop all SQL trigger functions (000451 scope):
   - `audit_trigger_function()`
   - `enable_audit_on_table()`
   - `enable_audit_on_schema()`
   - `disable_audit_on_table()`
   - `get_audit_statistics()`
   - `get_current_user_context()`
   - `calculate_audit_risk_score()`
   - `extract_compliance_flags()`
   - `determine_event_category()`
   - `determine_severity()`
2. Drop triggers on all tables that had audit triggers enabled
3. Drop `audit_log` table (legacy SQL trigger table)
4. Drop `audit_sensitive_tables` and `audit_sensitive_fields` (now superseded by Go registry + `platform_audit_log` config)
5. Revoke `iam.audit_log.read` permission from all roles; `platform.audit_log.read` remains

**Code changes:**
- Remove `awo/platform/audit/definition.go` (LogDefinition entity)
- Remove `def.Register(&LogDefinition)` call
- Remove side-effect import of `awo/platform/audit` from `main.go` (if only used for LogDefinition registration)
- Remove `audit_log` entity registration from `iam_role_permissions` seed

**Rollback:** Phase 5 is NOT easily reversible. Do NOT proceed until Phase 3 has been stable for at minimum 7 days and historical migration is verified complete.

**Validation:**
- No references to `audit_log` table in codebase
- `iam.audit_log.read` permission no longer in Casbin policies
- `platform.audit_log.read` permission functional for all roles that previously had `iam.audit_log.read`

---

### Phase 6: Package Repurpose

**Code changes:**
- Repurpose `awo/platform/audit/` package as read-only query API for `platform_audit_log`
- Add `AuditLogQuery`, `AuditLogFilter`, and handler for `GET /api/v1/platform/audit_log`
- Permission: `platform.audit_log.read`
- Tenant scoping: application-layer filter on `tenant_id` (not RLS)

**Gating condition:** Phase 5 complete.

---

## 3. Rollback Decision Tree

```
Problem detected at Phase N
        │
        ├─ Phase 1 → Drop migration tables; re-run migration without Phase 1
        │
        ├─ Phase 2 → Revert code changes; no DB rollback needed
        │
        ├─ Phase 3 → SET feature.unified_audit.enabled = 'false'
        │            Application immediately reverts to legacy-only path
        │
        ├─ Phase 4 → Stop migration job; platform_audit_log has partial history
        │            (acceptable; unified system still active for new events)
        │
        ├─ Phase 5 → NOT easily reversible
        │            Must restore from backup if legacy infrastructure needed
        │            DO NOT proceed to Phase 5 without ≥7 days of Phase 3 stability
        │
        └─ Phase 6 → Revert package changes; no DB rollback needed
```

---

## 4. Permission Migration

| Legacy Permission | New Permission | Roles |
|------------------|----------------|-------|
| `iam.audit_log.read` | `platform.audit_log.read` | `role:platform-admin`, `role:tenant.admin` |

Both permissions are active during Phases 2–4. Legacy permission is removed in Phase 5.

---

## References

- [`12-audit/AUDIT_ARCH.md`](AUDIT_ARCH.md) — Unified audit architecture
- [`12-audit/AUDIT_STORAGE.md`](AUDIT_STORAGE.md) — `platform_audit_log` schema
- ADR-019 in [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md)
- `db/migration/000448` through `000451` — Legacy audit migration files
