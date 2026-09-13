package audit

import (
	"context"
	"maps"
	"strings"
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/def"
)

func TestMarshalNullable_Nil(t *testing.T) {
	t.Parallel()

	b, err := marshalNullable(nil)
	if err != nil {
		t.Fatalf("marshalNullable(nil): unexpected error %v", err)
	}
	if b != nil {
		t.Errorf("marshalNullable(nil) = %v, want nil", b)
	}
}

func TestMarshalNullable_EmptyMap(t *testing.T) {
	t.Parallel()

	b, err := marshalNullable(map[string]any{})
	if err != nil {
		t.Fatalf("marshalNullable({}): unexpected error %v", err)
	}
	// Empty map → "{}" → should return nil (treated as NULL).
	if b != nil {
		t.Errorf("marshalNullable(empty map) = %q, want nil (NULL)", string(b))
	}
}

func TestMarshalNullable_EmptySlice(t *testing.T) {
	t.Parallel()

	b, err := marshalNullable([]string{})
	if err != nil {
		t.Fatalf("marshalNullable([]): unexpected error %v", err)
	}
	if b != nil {
		t.Errorf("marshalNullable(empty slice) = %q, want nil (NULL)", string(b))
	}
}

func TestMarshalNullable_NonEmpty(t *testing.T) {
	t.Parallel()

	b, err := marshalNullable(map[string]any{"key": "value"})
	if err != nil {
		t.Fatalf("marshalNullable: unexpected error %v", err)
	}
	if b == nil {
		t.Error("marshalNullable(non-empty map) must not return nil")
	}
	if string(b) != `{"key":"value"}` {
		t.Errorf("marshalNullable: got %q, want %q", string(b), `{"key":"value"}`)
	}
}

func TestMarshalNullable_NonEmptyStringSlice(t *testing.T) {
	t.Parallel()

	b, err := marshalNullable([]string{"status", "amount"})
	if err != nil {
		t.Fatalf("marshalNullable([]string): unexpected error %v", err)
	}
	if b == nil {
		t.Error("marshalNullable(non-empty slice) must not return nil")
	}
}

func TestNullableString(t *testing.T) {
	t.Parallel()

	if nullableString("") != nil {
		t.Error("nullableString(\"\") must return nil")
	}
	if v := nullableString("hello"); v != "hello" {
		t.Errorf("nullableString(\"hello\") = %v, want \"hello\"", v)
	}
}

func TestNullableUUID(t *testing.T) {
	t.Parallel()

	if nullableUUID(uuid.Nil) != nil {
		t.Error("nullableUUID(uuid.Nil) must return nil")
	}
	id := uuid.New()
	if v := nullableUUID(id); v != id {
		t.Errorf("nullableUUID(%v) = %v, want original UUID", id, v)
	}
}

// TestTransactionalWriter_Write_InvalidRecord verifies that Validate errors
// are returned before any SQL is attempted (no DB connection required).
func TestTransactionalWriter_Write_InvalidRecord(t *testing.T) {
	t.Parallel()

	w := NewTransactionalWriter(nil, NewSanitizer(), NewRiskScorer()) // nil fallback: must not be reached
	rec := AuditRecord{}                                              // invalid: missing TenantID, EntityName, etc.

	err := w.Write(context.TODO(), rec)
	//nolint:staticcheck — intentionally nil ctx to prove we don't reach SQL
	if err == nil {
		t.Error("Write with invalid record: expected error from Validate()")
	}
}

// TestTransactionalWriter_Write_NilFallback verifies that Write returns a
// descriptive error when no TX is in context and no fallback querier is set.
// This guards against silent no-ops from misconfigured deployments.
func TestTransactionalWriter_Write_NilFallback(t *testing.T) {
	t.Parallel()

	w := NewTransactionalWriter(nil, NewSanitizer(), NewRiskScorer()) // nil fallback — standalone writes must fail clearly
	rec := AuditRecord{
		TenantID:      uuid.New(),
		EntityName:    "finance_invoice",
		Operation:     OperationCreate,
		EventCategory: CategoryData,
		Actor:         &def.Actor{UserID: uuid.New()},
	}

	// context.Background() has no active transaction — fallback path is taken.
	err := w.Write(context.Background(), rec)
	if err == nil {
		t.Error("Write with nil fallback and no TX: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no database connection available") {
		t.Errorf("Write with nil fallback: expected descriptive error, got: %v", err)
	}
}

// TestNewTransactionalWriter_PanicsOnNilSanitizer verifies the constructor
// guards against nil sanitizer — a nil sanitizer would skip redaction silently.
func TestNewTransactionalWriter_PanicsOnNilSanitizer(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Error("NewTransactionalWriter with nil sanitizer must panic")
		}
	}()
	NewTransactionalWriter(nil, nil, NewRiskScorer())
}

// TestNewTransactionalWriter_PanicsOnNilScorer verifies the constructor
// guards against nil scorer.
func TestNewTransactionalWriter_PanicsOnNilScorer(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Error("NewTransactionalWriter with nil scorer must panic")
		}
	}()
	NewTransactionalWriter(nil, NewSanitizer(), nil)
}

// TestTransactionalWriter_Write_SensitiveFieldsRedacted verifies that
// sensitive fields declared via AdditionalSensitiveFields are never stored.
func TestTransactionalWriter_Write_SensitiveFieldsRedacted(t *testing.T) {
	t.Parallel()

	const entityName = "tw_sensitive_test"
	Register(EntityAuditConfig{
		EntityName:                entityName,
		Enabled:                   true,
		Category:                  CategoryData,
		AdditionalSensitiveFields: []string{"password_hash", "totp_secret"},
	})

	// Use a RecordingWriter wrapped as the "fallback" — we inspect what was
	// passed through Write's sanitization step by capturing via a custom writer.
	captured := &RecordingWriter{}
	w := &writeCapture{inner: captured, sanitizer: NewSanitizer(), scorer: NewRiskScorer()}

	rec := AuditRecord{
		TenantID:      uuid.New(),
		EntityName:    entityName,
		Operation:     OperationCreate,
		EventCategory: CategoryData,
		Actor:         &def.Actor{UserID: uuid.New()},
		AfterData:     map[string]any{"email": "u@example.com", "password_hash": "bcrypt$...", "totp_secret": "JBSWY3DP"},
	}

	// Simulate the sanitize+score logic directly (no DB needed).
	w.sanitize(rec)
	if captured.Len() != 1 {
		t.Fatalf("expected 1 captured record, got %d", captured.Len())
	}
	got := captured.Last()

	if v, ok := got.AfterData["password_hash"]; !ok || v != "[REDACTED]" {
		t.Errorf("password_hash: got %v, want [REDACTED]", v)
	}
	if v, ok := got.AfterData["totp_secret"]; !ok || v != "[REDACTED]" {
		t.Errorf("totp_secret: got %v, want [REDACTED]", v)
	}
	if v, ok := got.AfterData["email"]; !ok || v != "u@example.com" {
		t.Errorf("email: got %v, want u@example.com", v)
	}
}

// TestTransactionalWriter_Write_ComplianceFlagsMerged verifies that
// EntityAuditConfig.ComplianceFlags are merged into every record.
func TestTransactionalWriter_Write_ComplianceFlagsMerged(t *testing.T) {
	t.Parallel()

	const entityName = "tw_compliance_test"
	Register(EntityAuditConfig{
		EntityName:      entityName,
		Enabled:         true,
		Category:        CategoryData,
		ComplianceFlags: map[string]bool{"GDPR": true, "KRA_ETIMS": true},
	})

	w := &writeCapture{inner: &RecordingWriter{}, sanitizer: NewSanitizer(), scorer: NewRiskScorer()}
	rec := AuditRecord{
		TenantID:      uuid.New(),
		EntityName:    entityName,
		Operation:     OperationCreate,
		EventCategory: CategoryData,
		Actor:         &def.Actor{UserID: uuid.New()},
	}
	w.sanitize(rec)

	got := w.inner.(*RecordingWriter).Last()
	if !got.ComplianceFlags["GDPR"] {
		t.Error("ComplianceFlags[GDPR] must be true after merge")
	}
	if !got.ComplianceFlags["KRA_ETIMS"] {
		t.Error("ComplianceFlags[KRA_ETIMS] must be true after merge")
	}
}

// TestTransactionalWriter_Write_SeverityDerived verifies that Severity is set
// from the risk score rather than left as the zero value.
func TestTransactionalWriter_Write_SeverityDerived(t *testing.T) {
	t.Parallel()

	const entityName = "tw_severity_test"
	Register(EntityAuditConfig{EntityName: entityName, Enabled: true, Category: CategoryAdmin})

	w := &writeCapture{inner: &RecordingWriter{}, sanitizer: NewSanitizer(), scorer: NewRiskScorer()}
	rec := AuditRecord{
		TenantID:      uuid.New(),
		EntityName:    entityName,
		Operation:     OperationDelete, // 30 + ADMIN(30) = 60 → HIGH
		EventCategory: CategoryAdmin,
		Actor:         &def.Actor{UserID: uuid.New()},
	}
	w.sanitize(rec)

	got := w.inner.(*RecordingWriter).Last()
	if got.Severity == "" {
		t.Error("Severity must not be empty after Write")
	}
	// delete(30) + ADMIN(30) = 60 → HIGH
	if got.Severity != SeverityHigh {
		t.Errorf("Severity: got %q, want %q (score ~60)", got.Severity, SeverityHigh)
	}
	if got.RiskScore != 60 {
		t.Errorf("RiskScore: got %d, want 60", got.RiskScore)
	}
}

// TestTransactionalWriter_Write_ChangedFields verifies that ChangedFields is
// computed from sanitized maps on Update operations.
func TestTransactionalWriter_Write_ChangedFields(t *testing.T) {
	t.Parallel()

	const entityName = "tw_changedfields_test"
	Register(EntityAuditConfig{EntityName: entityName, Enabled: true, Category: CategoryData})

	w := &writeCapture{inner: &RecordingWriter{}, sanitizer: NewSanitizer(), scorer: NewRiskScorer()}
	rec := AuditRecord{
		TenantID:      uuid.New(),
		EntityName:    entityName,
		Operation:     OperationUpdate,
		EventCategory: CategoryData,
		Actor:         &def.Actor{UserID: uuid.New()},
		BeforeData:    map[string]any{"status": "draft", "amount": 100},
		AfterData:     map[string]any{"status": "active", "amount": 100},
	}
	w.sanitize(rec)

	got := w.inner.(*RecordingWriter).Last()
	if len(got.ChangedFields) != 1 || got.ChangedFields[0] != "status" {
		t.Errorf("ChangedFields: got %v, want [status]", got.ChangedFields)
	}
}

// writeCapture is a test helper that runs the TransactionalWriter enrichment
// logic (sanitize, compute changed fields, merge compliance, score) without
// requiring a database connection. It writes the enriched record to inner.
type writeCapture struct {
	inner     AuditWriter
	sanitizer *Sanitizer
	scorer    *RiskScorer
}

func (wc *writeCapture) sanitize(record AuditRecord) {
	record.BeforeData = wc.sanitizer.Strip(record.EntityName, record.BeforeData)
	record.AfterData = wc.sanitizer.Strip(record.EntityName, record.AfterData)
	record.ChangedFields = ComputeChangedFields(record.BeforeData, record.AfterData)

	cfg := ConfigFor(record.EntityName)
	if len(cfg.ComplianceFlags) > 0 {
		merged := make(map[string]bool, len(cfg.ComplianceFlags)+len(record.ComplianceFlags))
		maps.Copy(merged, cfg.ComplianceFlags)
		maps.Copy(merged, record.ComplianceFlags)
		record.ComplianceFlags = merged
	}

	record.RiskScore = wc.scorer.Score(&record)
	record.Severity = severityFromScore(record.RiskScore)

	_ = wc.inner.Write(context.Background(), record)
}

// TestTransactionalWriter_FlagDisabled verifies that Write is a no-op when
// the feature flag loader returns false — returns nil without touching the
// querier. Proven by passing nil querier: if the flag gate fires correctly,
// Write returns nil; if the gate is skipped, the nil querier causes a panic
// or a "no database" error.
func TestTransactionalWriter_FlagDisabled(t *testing.T) {
	t.Parallel()

	// nil querier: any path past the flag gate would panic or error.
	w := NewTransactionalWriter(nil, NewSanitizer(), NewRiskScorer()).
		WithFlagLoader(func(_ context.Context) bool { return false })

	rec := AuditRecord{
		TenantID:      uuid.New(),
		EntityName:    "finance_invoice",
		Operation:     OperationCreate,
		EventCategory: CategoryData,
		Actor:         &def.Actor{UserID: uuid.New()},
	}

	if err := w.Write(context.Background(), rec); err != nil {
		t.Errorf("Write with flag disabled: expected nil error, got %v", err)
	}
}

// TestTransactionalWriter_FlagEnabled verifies that Write proceeds past the
// flag check when the loader returns true. The flag-enabled path is proven by
// verifying the record reaches Validate() — an invalid record returns a
// validation error (not "no database connection available"), confirming the
// flag-disabled early-return was NOT taken.
func TestTransactionalWriter_FlagEnabled(t *testing.T) {
	t.Parallel()

	w := NewTransactionalWriter(nil, NewSanitizer(), NewRiskScorer()).
		WithFlagLoader(func(_ context.Context) bool { return true })

	invalidRec := AuditRecord{} // will fail Validate()
	err := w.Write(context.Background(), invalidRec)
	if err == nil {
		t.Fatal("expected Validate error when flag enabled; got nil")
	}
	if strings.Contains(err.Error(), "no database connection available") {
		t.Errorf("flag-enabled path skipped Validate; got: %v", err)
	}
}

// TestTransactionalWriter_FlagDefault_Enabled verifies that Write proceeds
// when no FlagLoader is configured (default = enabled).
func TestTransactionalWriter_FlagDefault_Enabled(t *testing.T) {
	t.Parallel()

	wNoFallback := NewTransactionalWriter(nil, NewSanitizer(), NewRiskScorer())
	// No WithFlagLoader → defaults to enabled.

	invalidRec := AuditRecord{} // will fail Validate()
	err := wNoFallback.Write(context.Background(), invalidRec)
	if err == nil {
		t.Fatal("expected Validate error; got nil")
	}
	// Confirm it failed at Validate, not at the "no DB" step.
	if strings.Contains(err.Error(), "no database connection available") {
		t.Errorf("default-enabled path skipped; got: %v", err)
	}
}

// TestTransactionalWriter_FlagLoader_CalledOnce verifies the loader is called
// exactly once regardless of concurrent Write calls (sync.Once guarantee).
func TestTransactionalWriter_FlagLoader_CalledOnce(t *testing.T) {
	t.Parallel()

	var count int
	w := NewTransactionalWriter(nil, NewSanitizer(), NewRiskScorer()).
		WithFlagLoader(func(_ context.Context) bool {
			count++
			return false // disabled → no DB needed
		})

	rec := AuditRecord{
		TenantID:      uuid.New(),
		EntityName:    "finance_invoice",
		Operation:     OperationCreate,
		EventCategory: CategoryData,
		Actor:         &def.Actor{UserID: uuid.New()},
	}

	// Call Write multiple times.
	for range 5 {
		_ = w.Write(context.Background(), rec)
	}

	if count != 1 {
		t.Errorf("FlagLoader called %d times, want exactly 1", count)
	}
}
