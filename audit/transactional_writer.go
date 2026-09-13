package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"sync"
	"time"

	"github.com/google/uuid"

	"awo.so/awo/tx"
)

// FlagLoader reads the feature flag that gates the unified audit system.
// The result is cached after the first call — restarts are required to
// pick up flag changes (acceptable for v1). A nil FlagLoader defaults to
// enabled (used in tests and when the feature flag table is not yet migrated).
//
// Production implementation: query platform_audit_config WHERE key =
// 'feature.unified_audit.enabled'. Return true if value == "true".
type FlagLoader func(ctx context.Context) bool

// TransactionalWriter is the production AuditWriter implementation.
// It writes AuditRecord values to platform_audit_log using the connection
// (transaction or pool) carried in context.
//
// Before any database write, TransactionalWriter:
//  1. Sanitizes BeforeData and AfterData through the Sanitizer — sensitive
//     fields are replaced with "[REDACTED]" so they never appear in the DB.
//  2. Computes ChangedFields from the sanitized snapshots.
//  3. Merges EntityAuditConfig.ComplianceFlags into AuditRecord.ComplianceFlags.
//  4. Scores the record with RiskScorer and derives Severity.
//
// When called from within an EntityService repo.WithTx callback, the record
// is written inside the active transaction — guaranteeing that no mutation
// escapes the audit trail. If Write fails and the failure policy is Propagate,
// the transaction rolls back together with the mutation.
//
// When called outside a transaction (e.g. standalone auth events), the writer
// falls back to the injected fallback tx.Querier — a pool-backed connection
// supplied at construction time via contrib/pgx.NewPoolQuerier.
//
// TransactionalWriter never imports contrib/pgx or pgxpool. All database
// access goes through tx.Querier — the framework interface for cross-package
// SQL execution. This keeps the audit package free of driver-layer imports.
type TransactionalWriter struct {
	// fallback is used when no active transaction is present in ctx.
	// It is a pool-backed tx.Querier created by contrib/pgx.NewPoolQuerier.
	// May be nil in test environments where all writes go through a
	// recording writer or are validated without a real DB.
	fallback  tx.Querier
	sanitizer *Sanitizer
	scorer    *RiskScorer

	// Feature flag: unified audit is active only when the flag is true.
	// Loaded once on first Write() call; cached for the process lifetime.
	// ADR-019 Phase 2: flag defaults to false; enabled in Phase 3.
	flagLoader  FlagLoader
	flagOnce    sync.Once
	flagEnabled bool
}

// NewTransactionalWriter returns a TransactionalWriter.
//
// fallback is a tx.Querier used for writes that occur outside an active
// database transaction (e.g. auth events triggered by IAM handlers).
// In production, pass contrib/pgx.NewPoolQuerier(pool) as the fallback.
// In tests that exercise only the validation path, nil is acceptable.
//
// sanitizer and scorer must not be nil.
func NewTransactionalWriter(fallback tx.Querier, sanitizer *Sanitizer, scorer *RiskScorer) *TransactionalWriter {
	if sanitizer == nil {
		panic("audit.NewTransactionalWriter: sanitizer must not be nil")
	}
	if scorer == nil {
		panic("audit.NewTransactionalWriter: scorer must not be nil")
	}
	return &TransactionalWriter{fallback: fallback, sanitizer: sanitizer, scorer: scorer}
}

// WithFlagLoader configures a FlagLoader for the feature flag check.
// Call before the first Write(). The loader is called exactly once; the
// result is cached for the process lifetime.
//
// When not set (nil), the writer defaults to enabled — correct for tests and
// deployments where platform_audit_config is not yet migrated.
func (w *TransactionalWriter) WithFlagLoader(loader FlagLoader) *TransactionalWriter {
	w.flagLoader = loader
	return w
}

// Write inserts record into platform_audit_log.
//
// Before inserting, Write:
//  1. Checks the feature flag (platform_audit_config); returns nil if disabled.
//  2. Sanitizes BeforeData and AfterData (sensitive fields → "[REDACTED]").
//  3. Computes ChangedFields from the sanitized snapshots.
//  4. Merges EntityAuditConfig.ComplianceFlags.
//  5. Computes RiskScore and derives Severity.
//
// Connection selection order:
//  1. tx.QuerierFromContext(ctx) — active transaction (entity mutation path).
//  2. w.fallback — pool-backed auto-commit (standalone auth/security events).
//
// Write validates the record before attempting the insert. Validation errors
// are returned regardless of the failure policy — they indicate programming
// errors, not DB errors.
func (w *TransactionalWriter) Write(ctx context.Context, record AuditRecord) error {
	// Lazy-load the feature flag on first call. sync.Once guarantees the loader
	// runs exactly once across all concurrent goroutines.
	w.flagOnce.Do(func() {
		if w.flagLoader == nil {
			// No loader configured: default to enabled.
			// Correct for: test environments, pre-migration deployments where
			// the caller has verified the flag is irrelevant.
			w.flagEnabled = true
			return
		}
		w.flagEnabled = w.flagLoader(ctx)
		if !w.flagEnabled {
			slog.WarnContext(ctx, "audit: unified audit disabled by feature flag (platform_audit_config)")
		}
	})
	if !w.flagEnabled {
		return nil // ADR-019 Phase 2 no-op — legacy system remains authoritative
	}

	if err := record.Validate(); err != nil {
		return err
	}

	// Step 1: Sanitize snapshots — sensitive fields must never reach the DB.
	record.BeforeData = w.sanitizer.Strip(record.EntityName, record.BeforeData)
	record.AfterData = w.sanitizer.Strip(record.EntityName, record.AfterData)

	// Step 2: Compute changed fields from sanitized maps (Update only).
	record.ChangedFields = ComputeChangedFields(record.BeforeData, record.AfterData)

	// Step 3: Merge EntityAuditConfig.ComplianceFlags into the record.
	// Config-level flags are the baseline; record-level flags override.
	cfg := ConfigFor(record.EntityName)
	if len(cfg.ComplianceFlags) > 0 {
		merged := make(map[string]bool, len(cfg.ComplianceFlags)+len(record.ComplianceFlags))
		maps.Copy(merged, cfg.ComplianceFlags)
		maps.Copy(merged, record.ComplianceFlags)
		record.ComplianceFlags = merged
	}

	// Step 4: Score risk and derive severity.
	record.RiskScore = w.scorer.Score(&record)
	record.Severity = severityFromScore(record.RiskScore)

	// Assign ID and timestamp if not already set by caller.
	if record.ID == uuid.Nil {
		record.ID = uuid.New()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}

	beforeJSON, err := marshalNullable(record.BeforeData)
	if err != nil {
		return errorf("audit: marshal BeforeData: %w", err)
	}
	afterJSON, err := marshalNullable(record.AfterData)
	if err != nil {
		return errorf("audit: marshal AfterData: %w", err)
	}
	contextJSON, err := marshalNullable(record.Context)
	if err != nil {
		return errorf("audit: marshal Context: %w", err)
	}
	complianceJSON, err := marshalNullable(record.ComplianceFlags)
	if err != nil {
		return errorf("audit: marshal ComplianceFlags: %w", err)
	}
	changedFieldsJSON, err := marshalNullable(record.ChangedFields)
	if err != nil {
		return errorf("audit: marshal ChangedFields: %w", err)
	}

	actorID := record.ActorID()
	saID := record.ServiceAccountID()

	var actorIDArg, saIDArg any
	if actorID != uuid.Nil {
		actorIDArg = actorID
	}
	if saID != uuid.Nil {
		saIDArg = saID
	}

	var systemActorArg any
	if record.SystemActor != "" {
		systemActorArg = string(record.SystemActor)
	}

	// Raw SQL is architecturally justified here: platform_audit_log is a
	// global partitioned table that is not registered as an EntityDefinition
	// and therefore not accessible via driver.EntityRepository. This mirrors
	// the pattern used by awo/platform/iam/queries.go for IAM junction tables.
	const insertSQL = `
INSERT INTO platform_audit_log (
	id, tenant_id, request_id,
	entity_name, record_id, operation,
	actor_id, service_account_id, system_actor,
	ip_address, session_id,
	before_data, after_data, changed_fields,
	event_category, severity, risk_score, compliance_flags,
	context, created_at
) VALUES (
	$1, $2, $3,
	$4, $5, $6,
	$7, $8, $9,
	$10, $11,
	$12, $13, $14,
	$15, $16, $17, $18,
	$19, $20
)`

	args := []any{
		record.ID, record.TenantID, nullableString(record.RequestID),
		record.EntityName, nullableUUID(record.RecordID), string(record.Operation),
		actorIDArg, saIDArg, systemActorArg,
		nullableString(record.IPAddress), nullableString(record.SessionID),
		beforeJSON, afterJSON, changedFieldsJSON,
		string(record.EventCategory), string(record.Severity), record.RiskScore, complianceJSON,
		contextJSON, record.CreatedAt,
	}

	// Prefer the active transaction from context (entity mutation path).
	if q, ok := tx.QuerierFromContext(ctx); ok {
		_, err = q.ExecSQL(ctx, insertSQL, args...)
		return err
	}

	// Fall back to the pool-backed querier (standalone auth event path).
	if w.fallback == nil {
		return errorf("audit: no database connection available: no active transaction and no fallback querier configured")
	}
	_, err = w.fallback.ExecSQL(ctx, insertSQL, args...)
	return err
}

// marshalNullable marshals v to JSON bytes. Returns nil when v is nil or
// empty (nil map, nil slice) so that the DB column stores NULL rather than
// an empty JSON object/array.
func marshalNullable(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	// Treat empty JSON objects and arrays as NULL.
	s := string(b)
	if s == "{}" || s == "[]" || s == "null" {
		return nil, nil
	}
	return b, nil
}

// nullableString returns nil when s is empty, preserving SQL NULL semantics.
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nullableUUID returns nil when u is the zero UUID, preserving SQL NULL.
func nullableUUID(u uuid.UUID) any {
	if u == uuid.Nil {
		return nil
	}
	return u
}
