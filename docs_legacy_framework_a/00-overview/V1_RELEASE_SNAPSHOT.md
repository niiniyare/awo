> ⚠️ **LEGACY DOCUMENTATION**
>
> This document describes the deprecated Framework A architecture and is retained for historical reference only.
>
> It MUST NOT be used when implementing or extending the Awo Framework (Framework B).
>
> Refer to [`awo/docs/`](../awo/docs/README.md) for the current canonical documentation.

---

# v1.0 Release Snapshot

**Classification:** Historical — Tier 3
**Owner:** `00-overview/V1_RELEASE_SNAPSHOT.md`
**Date:** 2026-07-20
**Status:** Historical record — do not update

---

## Purpose

This document records the state of the Awo Framework at the v1.0 architecture freeze. It is a historical snapshot, not a living document.

---

## Three Blockers Resolved Before Freeze

### Blocker 1: Authorization Model (ADR-001 + ADR-011)

**Problem:** `PermissionSet` declared roles but no `PolicyEvaluator` interface existed. The compiler emitted `CasbinPolicies` — coupling the compilation output to a specific authorization engine.

**Resolution:**
- Added `auth.PolicyEvaluator` interface to `awo/auth`
- Renamed `CasbinPolicies` to `CapabilityGrants` in `CompiledSchema`
- Authorization enforcement is now engine-agnostic

### Blocker 2: Actor Model (ADR-003)

**Problem:** `def.Actor.IsPlatformAdmin bool` was an architectural violation — a privilege expressed as an identity attribute rather than a role.

**Resolution:**
- Removed `IsPlatformAdmin bool` from `def.Actor`
- Added `ServiceAccountID uuid.UUID` for machine identity
- `IsPlatformAdmin()` is now a method checking `Roles` for `"role:platform-admin"`
- `IsServiceAccount()` checks whether `ServiceAccountID != uuid.Nil`

### Blocker 3: ViewerContext (ADR-002)

**Problem:** Authorization context had no typed contract. Repository methods could not reliably extract the authenticated viewer.

**Resolution:**
- Added `auth.ViewerContext` interface with 6 methods
- `auth.WithViewer()` and `auth.ViewerFromContext()` are the canonical propagation API
- `ViewerFromContext` panics if absent — middleware guarantees presence

---

## Architecture State at v1.0

### What Is Complete and Frozen

| Subsystem | Package | State |
|-----------|---------|-------|
| EntityDefinition contract | `awo/def` | Frozen |
| Field system | `awo/def` | Frozen |
| Filter DSL | `awo/filter` | Frozen |
| Hook pipeline | `awo/runtime` | Frozen |
| NamingSeries | `awo/naming` | Frozen |
| Registry + compiler | `awo/registry`, `awo/compiler` | Frozen |
| RLS enforcement | PostgreSQL + `set_tenant_context()` | Frozen |
| Cache + Counter interfaces | `awo/cache` | Frozen |
| WidgetTree IR | `awo/sdui/widget` | Frozen (ADR-006) |
| Auth: PolicyEvaluator | `awo/auth` | Frozen (ADR-001) |
| Auth: ViewerContext | `awo/auth` | Frozen (ADR-002) |
| Auth: Session | `awo/auth` | Frozen (ADR-004) |
| Actor (post-ADR-003) | `awo/def` | Frozen |
| Audit pipeline stage | `awo/audit` | Frozen (ADR-005) |
| Workflow outbox | `workflow_outbox` table | Frozen (ADR-007) |
| Event outbox | `event_outbox` table | Frozen (ADR-008) |
| Idempotency | `awo/middleware` | Frozen (ADR-009) |
| Hook recursion guard | `awo/runtime` | Frozen (ADR-010) |
| CapabilityGrants | `awo/compiler` | Frozen (ADR-011) |

### What Is Deferred to v1.1

| Feature | ADR |
|---------|-----|
| Organization/branch hierarchy | ADR-012 |
| Multi-org within a tenant | ADR-012 |

---

## Package DAG at v1.0

```
Level 0 (no internal deps):  def, filter, cache, outbox, sdui/widget
Level 1:                      audit(→def), auth(→def)
Level 2:                      naming(→cache,def), registry(→def)
Level 3:                      compiler(→def,registry)
Level 4:                      sdui/amis(→sdui/widget)
Level 5:                      sdui(→compiler,sdui/widget,sdui/amis,def,cache)
                              runtime(→def,auth,compiler,naming,cache,outbox,audit)
```

No cycles. Verified at freeze date.

---

## Documentation Produced at Freeze

All documentation in `awo/docs/` was generated from `ARCH_FREEZE_REVIEW.md` and source code at the freeze date. The documentation suite covers sections 00-overview through 99-modules: approximately 70 documents.

---

## References

- `awo/docs/ARCH_FREEZE_REVIEW.md` — Full ARB decision document with rationale
- [`00-overview/DECISION_REGISTER.md`](DECISION_REGISTER.md) — ADR summary register
