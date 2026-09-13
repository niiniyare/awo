# Documentation Review Report — Unified Audit System

**Date:** 2026-07-27
**Scope:** Documentation-wide consistency audit and unified audit architecture documentation

---

## Documents Reviewed

| Document | Classification | Review Result |
|----------|---------------|---------------|
| `00-overview/DECISION_REGISTER.md` | Constitutional — Tier 0 | Modified |
| `00-overview/ARCH_OVERVIEW.md` | Specification — Tier 0 | No change required |
| `02-pipeline/LIFECYCLE_SPEC.md` | Specification — Tier 0 | Modified |
| `02-pipeline/HOOK_CONTRACT.md` | Specification — Tier 1 | No change required |
| `02-pipeline/ERROR_MODEL.md` | Specification — Tier 0 | No change required |
| `12-audit/AUDIT_SPEC.md` | Specification — Tier 1 | **Rewritten** |
| `03-auth/ACTOR_SPEC.md` | Specification — Tier 0 | No change required |
| `03-auth/SESSION_SPEC.md` | Specification — Tier 0 | No change required |
| `03-auth/AUTHORIZATION_SPEC.md` | Specification — Tier 0 | No change required |
| `03-auth/VIEWER_CONTEXT.md` | Specification — Tier 0 | No change required |
| `03-auth/RBAC_ROLES_REFERENCE.md` | Reference — Tier 1 | No change required |
| `03-auth/CASBIN_ADAPTER.md` | Reference — Tier 2 | No change required |
| `01-entity/ENTITY_DEFINITION_SPEC.md` | Specification — Tier 0 | No change required |
| `01-entity/FIELD_TYPES_REFERENCE.md` | Reference — Tier 1 | No change required |
| `01-entity/EDGE_TYPES_REFERENCE.md` | Reference — Tier 1 | No change required |
| `04-multitenancy/TENANT_LIFECYCLE.md` | Specification — Tier 1 | No change required |
| `04-multitenancy/RLS_SPEC.md` | Specification — Tier 0 | No change required |
| `05-compiler/COMPILE_SPEC.md` | Specification — Tier 1 | No change required |
| `05-compiler/COMPILED_SCHEMA_REFERENCE.md` | Reference — Tier 1 | No change required |
| `08-workflow/OUTBOX_SPEC.md` | Specification — Tier 1 | No change required |
| `08-workflow/TEMPORAL_INTEGRATION.md` | Specification — Tier 1 | No change required |

---

## Documents Modified

### `00-overview/DECISION_REGISTER.md`

**Changes:**
- Added ADR-013 through ADR-019 to ADR index table
- Appended full ADR body text for ADR-013 through ADR-019
- Added `audit.AuditWriter`, `audit.AuditRecord`, `audit.EntityAuditConfig`, `platform_audit_log` to Public Contracts table
- Added references to `12-audit/AUDIT_ARCH.md` and updated `12-audit/AUDIT_SPEC.md` reference

### `02-pipeline/LIFECYCLE_SPEC.md`

**Changes:**
- Rewrote §3 "Transaction Boundary" → "Transaction Boundary and Ownership"
- Documented that driver (`contrib/pgx`) owns and manages transactions (ADR-013)
- Documented driver's internal execution sequence (PERSIST → RunAuditRecord → RunAfterCreate → commit/rollback)
- Added explicit consequence note: repository wrapper pattern must not be used for atomicity-requiring operations
- Added ADR-013 and ADR-014 to References section

### `12-audit/AUDIT_SPEC.md`

**Changes:** Complete rewrite. Previous content was a stub focused on the pipeline stage only. New content:
- `AuditWriter` interface with full contract documentation
- `AuditRecord` struct with all fields from unified architecture
- `EntityAuditConfig` registry (ADR-016 — no def modification)
- Actor population rules (ADR-015 — unified actor model)
- Sensitive field stripping with correct order (strip first, then diff)
- Skip conditions
- Changed fields computation rule
- Failure policy (ADR-017)
- Session ID HMAC derivation
- Full normative requirements
- Updated references to new documents

---

## New Documents Created

| Document | Purpose |
|----------|---------|
| `12-audit/AUDIT_ARCH.md` | Comprehensive unified audit architecture specification |
| `12-audit/AUDIT_STORAGE.md` | Storage DDL, indexes, partitioning, RLS, retention |
| `12-audit/AUDIT_MIGRATION.md` | Migration strategy from legacy dual system (6 phases) |
| `12-audit/DOC_REVIEW_REPORT.md` | This document |

---

## Architecture Decisions Introduced

| ADR | Decision | Status |
|-----|----------|--------|
| ADR-013 | Transaction ownership: driver owns TX (Model B confirmed) | Frozen |
| ADR-014 | Audit integration: pipeline AUDIT RECORD stage via AuditWriter; no repository wrapper | Frozen |
| ADR-015 | Unified actor model: def.Actor is the single actor type | Frozen |
| ADR-016 | Audit configuration: EntityAuditConfig registry; no def modification | Frozen |
| ADR-017 | Audit failure policy: ADMIN/SECURITY propagate; others suppress+log+meter | Frozen |
| ADR-018 | Audit storage: platform_audit_log, monthly partitions, global table | Frozen |
| ADR-019 | Dual audit elimination: unified system supersedes iam_audit_log + SQL triggers | Frozen |

---

## Obsolete Sections Removed / Superseded

| Component | Status | Replacement |
|-----------|--------|-------------|
| `iam_audit_log` entity registration | Retired (Phase 5 of migration) | `platform_audit_log` |
| `audit_log` SQL trigger table | Retired (Phase 5 of migration) | `platform_audit_log` |
| SQL trigger functions (000451) | Removed (Phase 5 of migration) | Go `TransactionalWriter` |
| `AuditingRepository[T]` wrapper pattern | Rejected (ADR-013) | Pipeline AUDIT RECORD stage |
| `audit.Actor` parallel type | Rejected (ADR-015) | `def.Actor` |
| `AuditEnabled bool` on SystemDefinition | Rejected (ADR-016 — frozen kernel) | `EntityAuditConfig` registry |
| AsyncWriter / channel-based async | Rejected (no durability guarantee) | TransactionalWriter (sync only) |

---

## Remaining Open Questions

These questions were identified during the review and must be resolved before implementation begins. Each is marked "Requires implementation verification" in the relevant document.

| # | Question | Blocking? | Location |
|---|----------|-----------|----------|
| 1 | How does `contrib/pgx` driver call `pipeline.RunAuditRecord`? What is the exact interface signature? | YES — must verify before writing any code | `AUDIT_ARCH.md §2.3` |
| 2 | What is the current `bootstrap.Run()` constructor signature? Does it accept signing secrets? | YES | `AUDIT_ARCH.md §5` |
| 3 | How does `router.Register()` wire the pipeline? Where is `AuditWriter` injected? | YES | `AUDIT_ARCH.md §5` |
| 4 | What PostgreSQL minimum version is required? (partitioning DDL compatibility) | YES | `AUDIT_STORAGE.md §4` |
| 5 | What migration number follows the current highest (000451)? | YES | `AUDIT_MIGRATION.md §2` |
| 6 | How is `platform_audit_config` feature flag read during bootstrap? Synchronous DB read? | YES | `AUDIT_ARCH.md §6.3` |
| 7 | Partition maintenance scheduler: pg_cron, Temporal scheduled workflow, or external cron? | YES — requires ADR | `AUDIT_STORAGE.md §4`, `AUDIT_ARCH.md §11.3` |
| 8 | For standalone AUTH audit writes (no TX), how does `TransactionalWriter` acquire a connection and set `app.current_tenant_id`? | YES | `AUDIT_ARCH.md §3.3` |
| 9 | `ActionDef` handlers — is there any existing interceptor hook for custom actions? | No (gap identified; v2) | `AUDIT_ARCH.md §9` |

---

## Items Requiring Implementation Verification

Before writing any code, these items must be verified by reading the actual implementation:

1. **`contrib/pgx` source** — Verify the driver calls pipeline methods from within its TX. Identify the exact call sites.
2. **`awo/runtime/pipeline.go`** — Verify `AuditWriter` injection point and `RunAuditRecord` method signature.
3. **`awo/bootstrap/bootstrap.go`** — Verify constructor signature and startup sequence.
4. **`awo/cmd/server/main.go`** — Verify `router.Register()` and `RegisterOptions` struct.
5. **Migration sequence** — Run `SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1` to confirm next migration number.

---

## Consistency Audit Results

After modifications, the following cross-document invariants hold:

| Invariant | Status |
|-----------|--------|
| Transaction ownership: driver, not pipeline | ✓ Consistent across LIFECYCLE_SPEC, AUDIT_SPEC, AUDIT_ARCH, DECISION_REGISTER |
| Actor model: def.Actor only | ✓ Consistent across ACTOR_SPEC, AUDIT_SPEC, AUDIT_ARCH, DECISION_REGISTER |
| Audit is mandatory pipeline stage (ADR-005) | ✓ Unchanged; confirmed in LIFECYCLE_SPEC and AUDIT_SPEC |
| Sensitive fields: strip before diff | ✓ Consistent in AUDIT_SPEC §5 and AUDIT_ARCH §2.6 |
| SessionID: HMAC-SHA256 of token, not raw token | ✓ Consistent in AUDIT_SPEC §10, AUDIT_ARCH §7.2 |
| Global table: platform_audit_log has no RLS | ✓ Consistent in AUDIT_STORAGE §5, AUDIT_ARCH §7 |
| EntityAuditConfig: init() only, no def modification | ✓ Consistent in AUDIT_SPEC §4, AUDIT_ARCH §6.1, DECISION_REGISTER ADR-016 |
| Failure policy: ADMIN/SECURITY propagate | ✓ Consistent in AUDIT_SPEC §9, AUDIT_ARCH §2.4, DECISION_REGISTER ADR-017 |
| Feature flag gates migration cutover | ✓ Consistent in AUDIT_ARCH §6.3, AUDIT_MIGRATION §2 |
| AuditWriter injected at bootstrap; no global singleton | ✓ Consistent in AUDIT_SPEC §2, AUDIT_ARCH §2.1, §5 |

No contradictions found after modifications.

---

## Risks

| Risk | Probability | Impact | Mitigation |
|------|------------|--------|------------|
| `contrib/pgx` does not call pipeline methods from within TX | Medium | CRITICAL — entire architecture invalid | Verify before any code; fallback: redesign to pipeline-initiated TX |
| Partition maintenance gap causes missed partitions | Medium | HIGH — audit writes fail for new month | Resolve scheduler decision (ADR required) before Phase 1 deployment |
| RiskScorer warm-up blocks bootstrap too long | Low | MEDIUM — startup timeout | Add timeout + fallback to default scores; alert on warm-up failure |
| Phase 5 executed before Phase 3 stability confirmed | Low | HIGH — compliance gap if rollback needed | Enforce 7-day stability requirement; gate on ops review |
| Custom action audit gap (v1) | Certain | MEDIUM — action events unaudited | Document as known gap; v2 roadmap item |

---

## Recommended Implementation Order

Before writing any code:

1. Read `contrib/pgx/` — all .go files — to verify TX ownership (open question #1)
2. Read `awo/runtime/pipeline.go` — verify AuditWriter injection point (open question #2)
3. Read `awo/bootstrap/bootstrap.go` — verify constructor (open question #3)
4. Read `awo/cmd/server/main.go` and `router.Register()` — verify wiring (open question #4)
5. Resolve partition maintenance scheduler via ADR (open question #7)

After verification:

6. Phase 1: Write and apply migration 000452 (infrastructure creation)
7. Phase 2: Implement `awo/audit` package, inject into pipeline, verify no-op when flag=false
8. Phase 3: Enable feature flag; validate for ≥ 24 hours in staging
9. Phase 4: Run historical migration job
10. Phase 5 + 6: Remove legacy infrastructure after ≥ 7 days stability

---

## Approval Criteria for Implementation Start

Documentation is complete and internally consistent. Implementation may begin after:

- [ ] All 9 "Requires implementation verification" items resolved by code reading
- [ ] Partition maintenance scheduler ADR written and approved
- [ ] Phase 1 migration number confirmed
- [ ] Staging environment available for Phase 3 validation
