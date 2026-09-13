// Package audit implements the Awo Unified Audit System.
//
// # Architecture
//
// The audit system writes one [AuditRecord] per entity mutation (Create,
// Update, Delete) and per explicit security/auth event (login, logout,
// permission denied). Records are written inside the same PostgreSQL
// transaction as the originating mutation, guaranteeing that no mutation
// escapes the audit trail.
//
// The central contract is [AuditWriter]:
//
//	type AuditWriter interface {
//	    Write(ctx context.Context, record AuditRecord) error
//	}
//
// The production implementation is [TransactionalWriter]. Tests use
// [NoopAuditWriter] or [RecordingWriter].
//
// # Integration point
//
// The runtime pipeline calls Write from within the [EntityService] repo.WithTx
// callback, between PERSIST and the after_save hooks. This guarantees
// transactional atomicity: if Write returns a propagating error, the entire
// mutation rolls back.
//
// # Failure policy
//
// Whether a Write failure propagates (aborts the mutation) or is suppressed
// (logged and metered, mutation proceeds) depends on the [EventCategory] of
// the record. ADMIN and SECURITY events propagate; all others suppress.
// See [FailurePolicy].
//
// # Entity configuration
//
// Per-entity audit configuration (enabled/disabled, category, additional
// sensitive fields) is declared via [Register] from module init() functions.
// Entities without an explicit registration use defaults: enabled, DATA
// category.
//
// # Sensitive field handling
//
// Fields marked Sensitive:true in their [def.FieldDef], listed in
// [EntityAuditConfig.AdditionalSensitiveFields], or present in the
// audit_sensitive_fields database config are stripped from BeforeData and
// AfterData snapshots. The diff ([AuditRecord.ChangedFields]) is computed
// from the stripped maps, so sensitive field names never appear in the trail.
//
// # References
//
//   - Architecture: awo/docs/12-audit/AUDIT_ARCH.md
//   - Specification: awo/docs/12-audit/AUDIT_SPEC.md
//   - ADRs: 005, 013–020 in awo/docs/00-overview/DECISION_REGISTER.md
package audit
