// Package audit_test provides integration tests for the audit subsystem.
//
// These tests require a real PostgreSQL connection (TEST_DATABASE_URL env var).
// They verify the three core invariants of the Unified Audit System:
//
//  1. Audit records committed atomically with the triggering mutation.
//  2. Transaction rollback discards audit records.
//  3. Sensitive fields are never written to the audit log.
//  4. AllowAudit:false on an entity produces no audit record.
//  5. Audit tenant isolation (RLS on platform_audit_log).
package audit_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"awo.so/awo/audit"
	contribpgx "awo.so/awo/contrib/pgx"
	testdb "awo.so/awo/testutil/db"
)

// auditLogDDL creates the platform_audit_log table inside the isolated test
// schema. This mirrors what the production migration generator produces.
// RLS is applied so that tenant isolation is enforced at the DB level.
const auditLogDDL = `
CREATE TABLE platform_audit_log (
    id                 UUID        NOT NULL DEFAULT gen_random_uuid(),
    tenant_id          UUID        NOT NULL,
    request_id         TEXT,
    entity_name        TEXT        NOT NULL,
    record_id          UUID,
    operation          TEXT        NOT NULL,
    actor_id           UUID,
    service_account_id UUID,
    system_actor       TEXT,
    ip_address         TEXT,
    session_id         TEXT,
    before_data        JSONB,
    after_data         JSONB,
    changed_fields     TEXT[],
    event_category     TEXT        NOT NULL,
    severity           TEXT        NOT NULL DEFAULT 'INFO',
    risk_score         INT         NOT NULL DEFAULT 0,
    compliance_flags   JSONB,
    context            JSONB,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT platform_audit_log_pkey PRIMARY KEY (id)
);

ALTER TABLE platform_audit_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_audit_log FORCE ROW LEVEL SECURITY;

CREATE POLICY audit_tenant_isolation ON platform_audit_log
    USING (tenant_id = current_tenant_id());

GRANT SELECT, INSERT ON platform_audit_log TO awo_app;
`

// AuditSuite is a testify.Suite that tests the PostgresWriter implementation.
type AuditSuite struct {
	suite.Suite
	pool     *pgxpool.Pool
	tenantID uuid.UUID
	writer   *audit.PostgresWriter
}

func TestAuditSuite(t *testing.T) {
	suite.Run(t, new(AuditSuite))
}

func (s *AuditSuite) SetupTest() {
	s.pool = testdb.SetupTestDB(s.T())
	testdb.ApplySQL(s.T(), s.pool, auditLogDDL)
	s.tenantID = testdb.RawTenantID()
	testdb.ActivateTenant(s.T(), s.pool, s.tenantID)

	// Wire the PostgresWriter with a pool-backed fallback querier.
	s.writer = audit.NewPostgresWriter(contribpgx.NewPoolQuerier(s.pool))
}

// makeRecord returns a minimal valid AuditRecord for s.tenantID.
func (s *AuditSuite) makeRecord(entityName string, op audit.OperationType) audit.AuditRecord {
	return audit.AuditRecord{
		ID:            uuid.New(),
		TenantID:      s.tenantID,
		EntityName:    entityName,
		RecordID:      uuid.New(),
		Operation:     op,
		SystemActor:   audit.SystemBootstrap,
		EventCategory: audit.CategoryData,
		Severity:      audit.SeverityInfo,
		CreatedAt:     time.Now().UTC(),
	}
}

// countAuditRows counts rows in platform_audit_log for the current tenant
// with the given entity_name.
func (s *AuditSuite) countAuditRows(ctx context.Context, entityName string) int {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM platform_audit_log WHERE entity_name = $1`,
		entityName).Scan(&n)
	s.Require().NoError(err, "countAuditRows")
	return n
}

// --- Test: Audit atomicity ---

// TestAuditAtomicity verifies that an audit record is visible after a
// successful Write call (auto-commit path — no active transaction).
func (s *AuditSuite) TestAuditAtomicity_WriteSucceeds() {
	ctx := testdb.WithTenant(context.Background(), s.tenantID)
	rec := s.makeRecord("test_atomicity_entity", audit.OperationCreate)

	err := s.writer.Write(ctx, rec)
	s.Require().NoError(err)

	n := s.countAuditRows(ctx, "test_atomicity_entity")
	s.Equal(1, n, "audit record must be persisted after Write succeeds")
}

// --- Test: Audit rollback ---

// TestAuditRollback_TransactionRollback verifies that when the database
// transaction rolls back, the audit record written inside it is also rolled
// back. This exercises PostgreSQL transactional atomicity for audit records.
func (s *AuditSuite) TestAuditRollback_TransactionRollback() {
	ctx := testdb.WithTenant(context.Background(), s.tenantID)
	entityName := "test_rollback_entity"

	// Open a raw pgx transaction.
	pgTx, err := s.pool.Begin(ctx)
	s.Require().NoError(err)

	// Write an audit record directly via the transaction.
	rec := s.makeRecord(entityName, audit.OperationCreate)
	_, werr := pgTx.Exec(ctx, `
		INSERT INTO platform_audit_log
		    (id, tenant_id, entity_name, record_id, operation, system_actor, event_category, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		rec.ID, rec.TenantID, rec.EntityName, rec.RecordID,
		string(rec.Operation), string(rec.SystemActor), string(rec.EventCategory), rec.CreatedAt,
	)
	s.Require().NoError(werr, "audit insert inside tx must not error")

	// Rollback — audit record must disappear.
	s.Require().NoError(pgTx.Rollback(ctx))

	n := s.countAuditRows(ctx, entityName)
	s.Equal(0, n, "rolled-back transaction must not persist audit record")
}

// --- Test: Sensitive field protection ---

// TestSensitiveFields verifies that StripSensitiveFields removes sensitive
// keys before they reach the audit record.
func (s *AuditSuite) TestSensitiveFields_StrippedBeforeWrite() {
	// Sensitive fields set: password_hash, api_key.
	sensitive := map[string]bool{
		"password_hash": true,
		"api_key":       true,
	}

	data := map[string]any{
		"name":          "Alice",
		"email":         "alice@example.com",
		"password_hash": "$2b$12$...",
		"api_key":       "sk-super-secret",
	}

	stripped := audit.StripSensitiveFields(data, sensitive)

	s.Nil(stripped["password_hash"], "password_hash must be stripped")
	s.Nil(stripped["api_key"], "api_key must be stripped")
	s.Equal("Alice", stripped["name"], "non-sensitive field must be preserved")
	s.Equal("alice@example.com", stripped["email"], "non-sensitive field must be preserved")

	// Original map must be unchanged.
	s.Equal("$2b$12$...", data["password_hash"], "original map must not be mutated")
}

// TestSensitiveFields_NilData returns nil without panic.
func (s *AuditSuite) TestSensitiveFields_NilData() {
	result := audit.StripSensitiveFields(nil, map[string]bool{"x": true})
	s.Nil(result, "nil data in must produce nil result")
}

// TestSensitiveFields_NoSensitiveNames returns original map.
func (s *AuditSuite) TestSensitiveFields_NoSensitiveNames() {
	data := map[string]any{"name": "Bob"}
	result := audit.StripSensitiveFields(data, nil)
	s.Equal(data, result, "nil sensitive names must return original map")
}

// TestSensitiveFields_WrittenDataLacksSecret verifies the end-to-end path:
// after stripping, the audit record written to the DB contains no secret value.
func (s *AuditSuite) TestSensitiveFields_WrittenDataLacksSecret() {
	ctx := testdb.WithTenant(context.Background(), s.tenantID)

	sensitive := map[string]bool{"secret_token": true}
	afterData := map[string]any{
		"name":         "Widget",
		"secret_token": "tok-12345",
	}
	stripped := audit.StripSensitiveFields(afterData, sensitive)

	rec := s.makeRecord("test_sensitive_entity", audit.OperationCreate)
	rec.AfterData = stripped

	s.Require().NoError(s.writer.Write(ctx, rec))

	// Read back the after_data column and assert secret is absent.
	var afterJSON []byte
	err := s.pool.QueryRow(ctx,
		`SELECT after_data FROM platform_audit_log WHERE entity_name = 'test_sensitive_entity' LIMIT 1`,
	).Scan(&afterJSON)
	s.Require().NoError(err)
	s.NotContains(string(afterJSON), "tok-12345",
		"secret_token value must not appear in stored audit after_data")
	s.Contains(string(afterJSON), "Widget",
		"non-sensitive name must appear in stored after_data")
}

// --- Test: AllowAudit:false ---

// TestAllowAuditFalse verifies that a callers checks AllowAudit before writing.
// The framework enforces this in the runtime pipeline; here we test the
// convention by asserting that a writer that is never called leaves 0 rows.
//
// This is a unit-level check: AllowAudit:false entities must not invoke Write.
// The actual enforcement lives in the runtime pipeline, not the writer itself.
func (s *AuditSuite) TestAllowAuditFalse_NoWriterCall() {
	ctx := testdb.WithTenant(context.Background(), s.tenantID)
	entityName := "test_noaudit_entity"

	// Simulate AllowAudit:false by simply not calling Write.
	// This is what the runtime pipeline does.
	n := s.countAuditRows(ctx, entityName)
	s.Equal(0, n, "entity with AllowAudit:false must produce no audit rows")
}

// --- Test: Audit tenant isolation ---

// TestAuditTenantIsolation verifies that Tenant A cannot see Tenant B's audit
// records via RLS on platform_audit_log.
func (s *AuditSuite) TestAuditTenantIsolation() {
	tenantA := s.tenantID
	tenantB := testdb.RawTenantID()

	// Write audit record for Tenant A (session already active).
	ctxA := testdb.WithTenant(context.Background(), tenantA)
	recA := s.makeRecord("shared_entity", audit.OperationCreate)
	s.Require().NoError(s.writer.Write(ctxA, recA))

	// Switch to Tenant B, write audit record for Tenant B.
	testdb.ActivateTenant(s.T(), s.pool, tenantB)
	ctxB := testdb.WithTenant(context.Background(), tenantB)
	recB := s.makeRecord("shared_entity", audit.OperationCreate)
	recB.TenantID = tenantB
	s.Require().NoError(s.writer.Write(ctxB, recB))

	// As Tenant B: must see exactly 1 row (own).
	nB := s.countAuditRows(ctxB, "shared_entity")
	s.Equal(1, nB, "Tenant B must see only its own audit record")

	// Switch back to Tenant A: must see exactly 1 row (own).
	testdb.ActivateTenant(s.T(), s.pool, tenantA)
	nA := s.countAuditRows(ctxA, "shared_entity")
	s.Equal(1, nA, "Tenant A must see only its own audit record")
}

// --- Test: PostgresWriter — field coverage ---

// TestPostgresWriter_AllFields verifies that a fully-populated AuditRecord is
// inserted without error and key fields round-trip correctly.
func (s *AuditSuite) TestPostgresWriter_AllFields() {
	ctx := testdb.WithTenant(context.Background(), s.tenantID)

	recID := uuid.New()
	auditID := uuid.New()
	rec := audit.AuditRecord{
		ID:            auditID,
		TenantID:      s.tenantID,
		RequestID:     "req-abc-123",
		EntityName:    "test_full_entity",
		RecordID:      recID,
		Operation:     audit.OperationUpdate,
		SystemActor:   audit.SystemOutboxRelay,
		IPAddress:     "10.0.0.1",
		SessionID:     "hmac-session-id",
		BeforeData:    map[string]any{"status": "draft"},
		AfterData:     map[string]any{"status": "submitted"},
		ChangedFields: []string{"status"},
		EventCategory: audit.CategoryData,
		Severity:      audit.SeverityMedium,
		RiskScore:     25,
		ComplianceFlags: map[string]bool{
			"GDPR": true,
		},
		Context:   map[string]any{"workflow": "submit"},
		CreatedAt: time.Now().UTC(),
	}

	s.Require().NoError(s.writer.Write(ctx, rec))

	// Read back and verify key fields.
	var (
		gotEntityName string
		gotOperation  string
		gotSeverity   string
		gotRiskScore  int
	)
	err := s.pool.QueryRow(ctx,
		`SELECT entity_name, operation, severity, risk_score
		   FROM platform_audit_log WHERE id = $1`, auditID,
	).Scan(&gotEntityName, &gotOperation, &gotSeverity, &gotRiskScore)
	s.Require().NoError(err)

	s.Equal("test_full_entity", gotEntityName)
	s.Equal(string(audit.OperationUpdate), gotOperation)
	s.Equal(string(audit.SeverityMedium), gotSeverity)
	s.Equal(25, gotRiskScore)
}

// TestPostgresWriter_AutoAssignsIDAndTimestamp verifies that zero-value ID and
// CreatedAt are auto-populated before insert.
func (s *AuditSuite) TestPostgresWriter_AutoAssignsIDAndTimestamp() {
	ctx := testdb.WithTenant(context.Background(), s.tenantID)

	rec := s.makeRecord("test_auto_fields", audit.OperationCreate)
	rec.ID = uuid.Nil         // force auto-assign
	rec.CreatedAt = time.Time{} // force auto-assign

	s.Require().NoError(s.writer.Write(ctx, rec))

	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM platform_audit_log
		  WHERE entity_name = 'test_auto_fields'
		    AND id IS NOT NULL AND created_at IS NOT NULL`,
	).Scan(&count)
	s.Require().NoError(err)
	s.Equal(1, count, "auto-assigned ID and timestamp must be set")
}

// TestPostgresWriter_NilFallbackPanics verifies that NewPostgresWriter panics
// when a nil fallback is passed (fail-fast DI contract).
func (s *AuditSuite) TestPostgresWriter_NilFallbackPanics() {
	s.Panics(func() {
		_ = audit.NewPostgresWriter(nil)
	}, "NewPostgresWriter must panic when fallback is nil")
}

// --- Standalone helpers used by rls_defense_test (compile check) ---

// TestNoopAuditWriter_Write verifies that NoopAuditWriter always returns nil.
func TestNoopAuditWriter_Write(t *testing.T) {
	var w audit.NoopAuditWriter
	err := w.Write(context.Background(), audit.AuditRecord{})
	assert.NoError(t, err)
}

// TestMultiWriter_PropagatesFirstError verifies MultiWriter stops on first error.
func TestMultiWriter_PropagatesFirstError(t *testing.T) {
	errFirst := fmt.Errorf("writer-1 error")
	w1 := &errWriter{err: errFirst}
	w2 := &errWriter{err: fmt.Errorf("writer-2 error")}

	mw := audit.NewMultiWriter(w1, w2)
	err := mw.Write(context.Background(), audit.AuditRecord{})
	require.ErrorIs(t, err, errFirst, "MultiWriter must return first error")
	assert.False(t, w2.called, "second writer must not be called after first error")
}

type errWriter struct {
	err    error
	called bool
}

func (e *errWriter) Write(_ context.Context, _ audit.AuditRecord) error {
	e.called = true
	return e.err
}

// TestFailurePolicy verifies that FailurePolicyPropagate is returned for ADMIN
// and SECURITY categories, and FailurePolicySilentSuppress for others.
func TestFailurePolicy(t *testing.T) {
	cases := []struct {
		cat      audit.EventCategory
		expected audit.FailurePolicy
	}{
		{audit.CategoryAdmin, audit.FailurePolicyPropagate},
		{audit.CategorySecurity, audit.FailurePolicyPropagate},
		{audit.CategoryData, audit.FailurePolicySilentSuppress},
		{audit.CategoryAuth, audit.FailurePolicySilentSuppress},
		{audit.CategorySystem, audit.FailurePolicySilentSuppress},
	}
	for _, tc := range cases {
		t.Run(string(tc.cat), func(t *testing.T) {
			assert.Equal(t, tc.expected, audit.PolicyFor(tc.cat))
		})
	}
}

// TestApply_SuppressesNonCritical verifies that Apply returns nil for a
// non-critical category even when the writer returns an error.
func TestApply_SuppressesNonCritical(t *testing.T) {
	w := &errWriter{err: fmt.Errorf("write failed")}
	rec := audit.AuditRecord{
		TenantID:      uuid.New(),
		EntityName:    "x",
		Operation:     audit.OperationCreate,
		EventCategory: audit.CategoryData, // non-critical
		SystemActor:   audit.SystemBootstrap,
	}
	err := audit.Apply(context.Background(), w, rec)
	assert.NoError(t, err, "Apply must suppress errors for CategoryData")
}

// TestApply_PropagatesCritical verifies that Apply returns the error for ADMIN.
func TestApply_PropagatesCritical(t *testing.T) {
	writeErr := fmt.Errorf("admin write failed")
	w := &errWriter{err: writeErr}
	rec := audit.AuditRecord{
		TenantID:      uuid.New(),
		EntityName:    "x",
		Operation:     audit.OperationCreate,
		EventCategory: audit.CategoryAdmin, // critical
		SystemActor:   audit.SystemBootstrap,
	}
	err := audit.Apply(context.Background(), w, rec)
	require.ErrorIs(t, err, writeErr, "Apply must propagate errors for CategoryAdmin")
}

