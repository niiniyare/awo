package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"awo.so/awo/tx"
)

// auditLogDDL is the CREATE TABLE statement for the platform_audit_log table.
// Call CreateTable to apply it before first use.
const auditLogDDL = `
CREATE TABLE IF NOT EXISTS platform_audit_log (
    id               UUID        NOT NULL DEFAULT gen_random_uuid(),
    tenant_id        UUID        NOT NULL,
    request_id       TEXT,
    entity_name      TEXT        NOT NULL,
    record_id        UUID,
    operation        TEXT        NOT NULL,
    actor_id         UUID,
    service_account_id UUID,
    system_actor     TEXT,
    ip_address       TEXT,
    session_id       TEXT,
    before_data      JSONB,
    after_data       JSONB,
    changed_fields   TEXT[],
    event_category   TEXT        NOT NULL,
    severity         TEXT        NOT NULL DEFAULT 'INFO',
    risk_score       INT         NOT NULL DEFAULT 0,
    compliance_flags JSONB,
    context          JSONB,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT platform_audit_log_pkey PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS platform_audit_log_tenant_entity_idx
    ON platform_audit_log (tenant_id, entity_name, created_at DESC);

CREATE INDEX IF NOT EXISTS platform_audit_log_record_idx
    ON platform_audit_log (entity_name, record_id, created_at DESC)
    WHERE record_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS platform_audit_log_actor_idx
    ON platform_audit_log (tenant_id, actor_id, created_at DESC)
    WHERE actor_id IS NOT NULL;
`

// PostgresWriter is an AuditWriter that persists AuditRecord values to the
// platform_audit_log table. It participates in the active database transaction
// (if any) via tx.QuerierFromContext, ensuring that the audit record is
// committed or rolled back atomically with the originating mutation.
//
// Construction:
//
//	w := audit.NewPostgresWriter(pool)
//
// The writer is safe for concurrent use. All state is derived from the
// context and AuditRecord at Write time.
type PostgresWriter struct {
	fallback tx.Querier // pool-backed Querier used when no tx is active
}

// NewPostgresWriter returns a PostgresWriter that uses fallback when the
// request context does not carry an active transaction. fallback must be a
// pool-backed tx.Querier (e.g. from contribpgx.NewPoolQuerier).
//
// Passing nil panics at construction time to surface misconfiguration early.
func NewPostgresWriter(fallback tx.Querier) *PostgresWriter {
	if fallback == nil {
		panic("audit.NewPostgresWriter: fallback Querier must not be nil")
	}
	return &PostgresWriter{fallback: fallback}
}

// Write persists record to platform_audit_log. Sensitive fields must already
// be stripped by the caller before invoking Write (the writer does not
// redact). The write participates in the active transaction when one is
// present in ctx; otherwise it executes via the fallback pool connection
// (auto-commit semantics).
func (w *PostgresWriter) Write(ctx context.Context, record AuditRecord) error {
	if record.ID == uuid.Nil {
		record.ID = uuid.New()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}

	q, ok := tx.QuerierFromContext(ctx)
	if !ok {
		q = w.fallback
	}

	beforeJSON, err := marshalJSONNullable(record.BeforeData)
	if err != nil {
		return fmt.Errorf("audit.PostgresWriter: marshal before_data: %w", err)
	}
	afterJSON, err := marshalJSONNullable(record.AfterData)
	if err != nil {
		return fmt.Errorf("audit.PostgresWriter: marshal after_data: %w", err)
	}
	flagsJSON, err := marshalJSONNullable(mapBoolToAny(record.ComplianceFlags))
	if err != nil {
		return fmt.Errorf("audit.PostgresWriter: marshal compliance_flags: %w", err)
	}
	ctxJSON, err := marshalJSONNullable(record.Context)
	if err != nil {
		return fmt.Errorf("audit.PostgresWriter: marshal context: %w", err)
	}

	var actorID, saID *uuid.UUID
	var systemActor *string
	if record.Actor != nil {
		if record.Actor.UserID != uuid.Nil {
			id := record.Actor.UserID
			actorID = &id
		}
		if record.Actor.ServiceAccountID != uuid.Nil {
			id := record.Actor.ServiceAccountID
			saID = &id
		}
	} else if record.SystemActor != "" {
		s := string(record.SystemActor)
		systemActor = &s
	}

	var recordID *uuid.UUID
	if record.RecordID != uuid.Nil {
		id := record.RecordID
		recordID = &id
	}

	const insertSQL = `
INSERT INTO platform_audit_log (
    id, tenant_id, request_id,
    entity_name, record_id, operation,
    actor_id, service_account_id, system_actor,
    ip_address, session_id,
    before_data, after_data, changed_fields,
    event_category, severity, risk_score,
    compliance_flags, context, created_at
) VALUES (
    $1, $2, $3,
    $4, $5, $6,
    $7, $8, $9,
    $10, $11,
    $12, $13, $14,
    $15, $16, $17,
    $18, $19, $20
)`

	_, err = q.ExecSQL(
		ctx, insertSQL,
		record.ID, record.TenantID, nullableString(record.RequestID),
		record.EntityName, recordID, string(record.Operation),
		actorID, saID, systemActor,
		nullableString(record.IPAddress), nullableString(record.SessionID),
		beforeJSON, afterJSON, changedFieldsArray(record.ChangedFields),
		string(record.EventCategory), string(record.Severity), record.RiskScore,
		flagsJSON, ctxJSON, record.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("audit.PostgresWriter: insert: %w", err)
	}
	return nil
}

// StripSensitiveFields removes sensitive field names from data maps before
// they are written to the audit log. Returns shallow copies; original maps
// are not modified. Pass def.FieldDef slice — fields with Sensitive:true are
// excluded. Pass nil fields to skip stripping (all fields included).
//
// Call this before constructing an AuditRecord:
//
//	before = audit.StripSensitiveFields(before, entity.EntityFields())
//	after  = audit.StripSensitiveFields(after,  entity.EntityFields())
func StripSensitiveFields(data map[string]any, sensitiveNames map[string]bool) map[string]any {
	if len(data) == 0 || len(sensitiveNames) == 0 {
		return data
	}
	out := make(map[string]any, len(data))
	for k, v := range data {
		if !sensitiveNames[k] {
			out[k] = v
		}
	}
	return out
}

// --- helpers ---

func marshalJSONNullable(v map[string]any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

func mapBoolToAny(m map[string]bool) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// func nullableString(s string) *string {
// 	if s == "" {
// 		return nil
// 	}
// 	return &s
// }

func changedFieldsArray(fields []string) any {
	if len(fields) == 0 {
		return nil
	}
	return fields
}
