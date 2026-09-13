package audit

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/def"
)

func BenchmarkHashSessionToken(b *testing.B) {
	token := "awo-session-token-00000000000000000000000000000000"
	secret := "awo-hmac-server-secret-32-bytes!!"

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_ = HashSessionToken(token, secret)
	}
}

func BenchmarkSanitizer_Strip_NoSensitive(b *testing.B) {
	s := NewSanitizer()
	input := map[string]any{
		"name":     "ACME Corp Ltd",
		"status":   "active",
		"amount":   "10000.00",
		"currency": "KES",
		"notes":    "standard invoice",
	}

	b.ReportAllocs()
	for b.Loop() {
		_ = s.Strip("finance_invoice_bench_no_sensitive", input)
	}
}

func BenchmarkSanitizer_Strip_WithSensitive(b *testing.B) {
	const entityName = "bench_entity_sensitive_fields"
	Register(EntityAuditConfig{
		EntityName:                entityName,
		Enabled:                   true,
		Category:                  CategoryData,
		Severity:                  SeverityInfo,
		AdditionalSensitiveFields: []string{"password", "api_key", "secret_token"},
	})
	// For benchmarks, use test keys or environment variables
	testStripeKey := os.Getenv("STRIPE_TEST_KEY") // or "sk_test_..."
	s := NewSanitizer()
	input := map[string]any{
		"name":         "ACME Corp Ltd",
		"password":     "dummy-password-for-testing",
		"api_key":      testStripeKey,         // Official Stripe test key
		"secret_token": "tok_test_1234567890", // Test token format
		"status":       "active",
	}

	b.ReportAllocs()
	for b.Loop() {
		_ = s.Strip(entityName, input)
	}
}

func BenchmarkComputeChangedFields(b *testing.B) {
	before := map[string]any{
		"name":     "ACME Corp",
		"status":   "draft",
		"amount":   "5000.00",
		"currency": "KES",
		"notes":    "initial invoice",
		"due_date": "2026-08-01",
	}
	after := map[string]any{
		"name":     "ACME Corp",
		"status":   "submitted",
		"amount":   "5500.00",
		"currency": "KES",
		"notes":    "updated with discount",
		"due_date": "2026-08-15",
	}

	b.ReportAllocs()
	for b.Loop() {
		_ = ComputeChangedFields(before, after)
	}
}

func BenchmarkRiskScorer_Score(b *testing.B) {
	rs := NewRiskScorer()
	rec := &AuditRecord{
		EntityName:    "finance_invoice",
		Operation:     OperationDelete,
		EventCategory: CategoryAdmin,
		Actor:         &def.Actor{UserID: uuid.New()},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_ = rs.Score(rec)
	}
}

func BenchmarkRecordingWriter_Write(b *testing.B) {
	rw := &RecordingWriter{}
	rec := AuditRecord{
		TenantID:      uuid.New(),
		EntityName:    "finance_invoice",
		Operation:     OperationCreate,
		EventCategory: CategoryData,
		Actor:         &def.Actor{UserID: uuid.New()},
		BeforeData:    nil,
		AfterData:     map[string]any{"name": "ACME", "status": "draft", "amount": "5000.00"},
		ChangedFields: nil,
	}
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_ = rw.Write(ctx, rec)
	}
}

func BenchmarkApply_NoError(b *testing.B) {
	rw := &RecordingWriter{}
	rec := AuditRecord{
		TenantID:      uuid.New(),
		EntityName:    "finance_invoice",
		Operation:     OperationCreate,
		EventCategory: CategoryData,
		Actor:         &def.Actor{UserID: uuid.New()},
	}
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_ = Apply(ctx, rw, rec)
		rw.Reset()
	}
}
