# Audit Specification

**Classification:** Specification — Tier 1
**Owner:** `12-audit/AUDIT_SPEC.md`
**Status:** Updated — v1.0 (ADR-005, ADR-013 through ADR-019)
**Package:** `awo.so/awo/audit`

---

## Purpose

This document specifies:

1. The AUDIT RECORD pipeline stage (ADR-005) — its position, transactional guarantees, and skip conditions.
2. The `AuditWriter` interface — the contract between the pipeline and the audit subsystem.
3. The `AuditRecord` schema — the data written per mutation event.
4. The `EntityAuditConfig` registry — per-entity audit configuration without modifying frozen kernel types.
5. Sensitive field handling — what is included and excluded from snapshots.
6. Normative requirements.

For the full unified audit architecture, storage schema, partitioning, failure policy, migration strategy, and operational model, see [`12-audit/AUDIT_ARCH.md`](AUDIT_ARCH.md).

---

## 1. ADR-005: Audit Is a Mandatory Pipeline Stage

Audit is a non-optional pipeline stage inserted by the runtime between PERSIST and after_save, inside the driver-managed transaction:

```
PERSIST → AUDIT RECORD (inside TX) → after_save → [TX commits]
```

**Why inside the transaction:** If the entity write succeeds but the audit write fails (and the failure policy for the entity's category is Propagate), the transaction rolls back. The entity record and its audit trail are always consistent — either both exist or neither exists.

**Why not a hook:** ADR-005 rejected optional audit hooks because hook authors can forget to add them or remove them without removing audit side effects. Mandatory stage eliminates silent compliance gaps.

**Transaction ownership (ADR-013):** The driver (`contrib/pgx`) owns and manages the transaction. The pipeline does not open or commit transactions. The driver calls `pipeline.RunAuditRecord(ctx, record)` from within the open transaction. The `AuditWriter` implementation executes its INSERT using the transaction-carrying context. See [`02-pipeline/LIFECYCLE_SPEC.md`](../02-pipeline/LIFECYCLE_SPEC.md) for the complete pipeline with transaction boundary.

---

## 2. AuditWriter Interface

```go
// Package awo.so/awo/audit

// AuditWriter is the contract between the pipeline's AUDIT RECORD stage
// and the audit storage backend. It is injected into the pipeline at
// bootstrap construction time.
//
// The pipeline guarantees that Write is called:
//   - after PERSIST succeeds
//   - before after_save hooks run
//   - inside the driver-managed database transaction (ctx carries the TX)
//
// Implementations must use the context-carried transaction connection
// for their INSERT. Using a separate connection breaks the atomicity
// guarantee.
type AuditWriter interface {
    // Write appends one audit record. ctx must carry an active database
    // transaction. Write participates in that transaction.
    //
    // If the entity's EntityAuditConfig.Enabled is false, Write returns
    // nil without writing (no-op per ADR-016).
    //
    // Failure behaviour is governed by ADR-017:
    //   - ADMIN / SECURITY category: return the error (pipeline rolls back TX)
    //   - All other categories: log + meter, return nil (mutation proceeds)
    Write(ctx context.Context, record AuditRecord) error
}

// NoopAuditWriter discards all audit records. Use in unit tests
// and in non-production environments where audit storage is not configured.
type NoopAuditWriter struct{}

func (NoopAuditWriter) Write(_ context.Context, _ AuditRecord) error { return nil }
```

---

## 3. AuditRecord

```go
// AuditRecord is produced by the pipeline for every entity mutation
// where EntityAuditConfig.Enabled is true (default).
type AuditRecord struct {
    // Identity
    ID        uuid.UUID
    TenantID  uuid.UUID
    RequestID string    // X-Request-ID from middleware; empty for system ops

    // Entity
    EntityName string        // qualified name, e.g. "finance_invoice"
    RecordID   uuid.UUID     // primary key of the affected record
    Operation  OperationType // Create | Update | Delete

    // Actor — exactly one of Actor or SystemActor is set
    Actor       *def.Actor  // nil for system operations (ADR-015)
    SystemActor SystemActor // empty for human/service-account operations

    // Request context (nil for system operations)
    IPAddress string
    SessionID string // HMAC-SHA256(session_token, server_secret) — never raw token

    // Snapshots — sensitive fields excluded before population (§5)
    BeforeData    map[string]any // nil on Create
    AfterData     map[string]any // nil on Delete
    ChangedFields []string       // populated on Update only

    // Classification
    EventCategory EventCategory // DATA | AUTH | ACCESS | ADMIN | WORKFLOW | SYSTEM | OUTBOUND | SECURITY
    Severity      Severity      // CRITICAL | HIGH | MEDIUM | LOW | INFO
    RiskScore     int           // 0–100 composite score
    ComplianceFlags map[string]bool // GDPR, PCI_DSS, KRA_ETIMS, etc.

    // Context — event-type-specific JSONB blob; schema by EventCategory
    Context map[string]any

    // Timestamps
    CreatedAt time.Time
}

// OperationType represents the mutation operation.
type OperationType string

const (
    OperationCreate OperationType = "create"
    OperationUpdate OperationType = "update"
    OperationDelete OperationType = "delete"
)

// EventCategory classifies the audit event for routing, failure policy, and retention.
type EventCategory string

const (
    CategoryData     EventCategory = "DATA"
    CategoryAuth     EventCategory = "AUTH"
    CategoryAccess   EventCategory = "ACCESS"
    CategoryAdmin    EventCategory = "ADMIN"
    CategoryWorkflow EventCategory = "WORKFLOW"
    CategorySystem   EventCategory = "SYSTEM"
    CategoryOutbound EventCategory = "OUTBOUND"
    CategorySecurity EventCategory = "SECURITY"
)

// Severity classifies the risk level of the audit event.
type Severity string

const (
    SeverityCritical Severity = "CRITICAL"
    SeverityHigh     Severity = "HIGH"
    SeverityMedium   Severity = "MEDIUM"
    SeverityLow      Severity = "LOW"
    SeverityInfo     Severity = "INFO"
)

// SystemActor identifies non-human execution contexts.
type SystemActor string

const (
    SystemBootstrap   SystemActor = "system:bootstrap"
    SystemMigration   SystemActor = "system:migration"
    SystemOutboxRelay SystemActor = "system:outbox-relay"
    SystemScheduler   SystemActor = "system:scheduler"
)
```

---

## 4. EntityAuditConfig Registry

`def.SystemDefinition` and `def.CustomDefinition` are frozen kernel types (ADR-016). Audit-specific configuration is declared separately in `awo/audit`:

```go
// EntityAuditConfig declares audit behaviour for one entity.
// Register from init() alongside def.Register().
type EntityAuditConfig struct {
    // EntityName is the qualified entity name, e.g. "finance_invoice".
    // Must match the EntityDefinition.EntityName() exactly.
    EntityName string

    // Enabled controls whether audit records are written for this entity.
    // Default: true. Setting false suppresses ALL audit for this entity.
    //
    // MUST NOT be set to false for:
    //   - Any Finance module entity (KRA eTIMS, regulatory compliance)
    //   - IAM entities: user, tenant, role (IAM audit requirement)
    //   - Any entity with Sensitive: true fields
    Enabled bool

    // Category classifies all audit records for this entity.
    // Default: CategoryData.
    // IAM module sets CategoryAuth / CategoryAdmin.
    // Platform admin operations set CategoryAdmin.
    Category EventCategory

    // AdditionalSensitiveFields lists field names to strip from snapshots
    // in addition to fields already marked Sensitive: true in FieldDef.
    // Use for runtime-discovered PII not known at compile time.
    AdditionalSensitiveFields []string
}

// Register declares audit configuration for an entity.
// Panics if called after bootstrap completes (init() only).
// Panics if EntityName is already registered (duplicate registration).
func Register(cfg EntityAuditConfig) { ... }

// ConfigFor returns the audit configuration for entityName.
// Returns default config (Enabled: true, Category: CategoryData) if
// no explicit registration exists.
func ConfigFor(entityName string) EntityAuditConfig { ... }
```

Entities without explicit registration use defaults: `Enabled: true`, `Category: CategoryData`.

---

## 5. Sensitive Field Handling

The audit writer applies sensitivity stripping before populating `BeforeData` and `AfterData`. The stripping order is:

```
1. Strip fields where FieldDef.Sensitive == true (compile-time; from CompiledSchema)
2. Strip fields in EntityAuditConfig.AdditionalSensitiveFields (registration-time)
3. Strip fields in audit_sensitive_fields DB config (runtime; RiskScorer cache)
4. Compute ChangedFields diff from the stripped maps (not from raw data)
```

**Critical invariant:** Diff computation uses stripped data. If a field is sensitive, its change is not recorded in `ChangedFields`. The field name does not appear in `ChangedFields` at all.

**`custom_fields` JSONB:** The entire blob is included as-is. Per-key sensitivity filtering within `custom_fields` is not supported in v1 — if any key in `custom_fields` is sensitive, suppress the entire blob via `AdditionalSensitiveFields: []string{"custom_fields"}`.

**Computed/virtual fields:** Never in snapshots. Not persisted, not audited.

---

## 6. Skip Conditions

The AUDIT RECORD stage is skipped (no write, no error) when:

1. `EntityAuditConfig.Enabled == false` for the entity.
2. No `AuditWriter` is injected (bootstrap injected `NoopAuditWriter`).
3. The operation is a read (Get, Query, Count, Exists) — reads are not pipelined.

When skipped, the pipeline proceeds to after_save without executing any audit write.

---

## 7. Actor Population Rules

The `AuditRecord` actor fields follow these rules:

| Execution Context | `Actor` | `SystemActor` |
|---|---|---|
| Human user request | `*def.Actor` (UserID set, ServiceAccountID nil) | empty |
| Service account request | `*def.Actor` (ServiceAccountID set, UserID nil) | empty |
| Platform admin request | `*def.Actor` (UserID set, roles include "role:platform-admin") | empty |
| Bootstrap migration | nil | `SystemBootstrap` |
| Outbox relay dispatch | nil | `SystemOutboxRelay` |
| Scheduler job | nil | `SystemScheduler` |

Exactly one of `Actor` or `SystemActor` is non-nil/non-empty per record.

---

## 8. Changed Fields Computation

For Update operations:

- `ChangedFields` contains field names whose value differs between `BeforeData` (pre-strip) and `AfterData` (pre-strip), after sensitivity stripping.
- Comparison: deep equality after JSON round-trip normalization.
- Fields not in the update patch but whose value did not change are NOT in `ChangedFields`.
- Sensitive fields are excluded from `ChangedFields` (the field name does not appear).

---

## 9. Failure Policy

Governed by ADR-017. Applied by the `AuditWriter` implementation, not the pipeline:

| Category | On write failure |
|----------|-----------------|
| ADMIN | Return error → pipeline propagates → driver rolls back TX |
| SECURITY | Return error → pipeline propagates → driver rolls back TX |
| DATA | Log ERROR + increment `audit_write_failure_total{category="DATA"}` → return nil |
| AUTH | Log ERROR + increment counter → return nil |
| ACCESS | Log ERROR + increment counter → return nil |
| WORKFLOW | Log ERROR + increment counter → return nil |
| SYSTEM | Log ERROR + increment counter → return nil |
| OUTBOUND | Log ERROR + increment counter → return nil |

---

## 10. Session ID Handling

The `AuditRecord.SessionID` field stores an HMAC-SHA256 of the session token, not the raw token. This allows:
- Correlating all requests from the same session without exposing the raw token.
- Preventing rainbow-table attacks on session tokens if the audit log is compromised.

The HMAC secret is the server's session signing secret, injected at bootstrap. It is never stored in the audit record.

```
SessionID = HMAC-SHA256(session_token, server_signing_secret)
         encoded as hex string (64 characters)
```

---

## 11. Normative Requirements

- The AUDIT RECORD stage MUST execute within the driver-managed database transaction (ADR-013).
- The `AuditWriter` MUST use the context-carried transaction connection for its INSERT (ADR-013).
- Sensitive fields MUST be stripped before populating `BeforeData` and `AfterData` (§5).
- `ChangedFields` MUST be computed from stripped data, not raw data (§5).
- Exactly one of `Actor` or `SystemActor` MUST be set per record (§7).
- `SessionID` MUST be HMAC-SHA256 of the session token; the raw token MUST NOT be stored (§10).
- `EntityAuditConfig.Enabled: false` MUST NOT be set on finance, IAM, or sensitive entities (§4).
- The `AuditWriter` failure policy MUST follow ADR-017 by category (§9).
- The `AuditWriter` is injected at bootstrap; there is no global singleton (ADR-014).
- `def.Actor` is the single actor type; no parallel audit actor type exists (ADR-015).

---

## References

- [`12-audit/AUDIT_ARCH.md`](AUDIT_ARCH.md) — Full unified audit architecture
- [`12-audit/AUDIT_STORAGE.md`](AUDIT_STORAGE.md) — Storage schema, partitioning, RLS, retention
- [`12-audit/AUDIT_MIGRATION.md`](AUDIT_MIGRATION.md) — Migration from legacy dual system
- [`02-pipeline/LIFECYCLE_SPEC.md`](../02-pipeline/LIFECYCLE_SPEC.md) — Pipeline with AUDIT RECORD stage
- [`00-overview/DECISION_REGISTER.md`](../00-overview/DECISION_REGISTER.md) — ADR-005, ADR-013 through ADR-019
- [`01-entity/FIELD_TYPES_REFERENCE.md`](../01-entity/FIELD_TYPES_REFERENCE.md) — `Sensitive` field flag
- [`04-multitenancy/GLOBAL_TABLES.md`](../04-multitenancy/GLOBAL_TABLES.md) — `platform_audit_log` as global table
