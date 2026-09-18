package dberr

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"awo.so/awo/runtime"
)

// TestParse_MatchesRealPgxV5Error proves that dberr.Parse recognizes an error
// of the exact concrete type pgx/v5 actually returns.
//
// Phase 0 discovered that dberr.go imported the legacy, standalone
// "github.com/jackc/pgconn" package (pgx v4-era) while every other framework
// package uses pgx/v5. errors.As matches by concrete Go type, not by
// structural shape, so *github.com/jackc/pgconn.PgError and
// *github.com/jackc/pgx/v5/pgconn.PgError are permanently incompatible even
// though their fields look alike. Before the import fix, this test fails:
// Parse falls through to the generic "%s: database error (...): %w" wrap
// branch instead of returning a typed *runtime.BusinessError.
func TestParse_MatchesRealPgxV5Error(t *testing.T) {
	tests := []struct {
		name     string
		pgErr    *pgconn.PgError
		wantCode string
		wantKind string // "business" or "validation"
	}{
		{
			name:     "unique_violation",
			pgErr:    &pgconn.PgError{Code: CodeUniqueViolation, ConstraintName: "finance_invoice_number_uniq"},
			wantCode: "duplicate",
			wantKind: "business",
		},
		{
			name:     "foreign_key_violation",
			pgErr:    &pgconn.PgError{Code: CodeForeignKeyViolation},
			wantCode: "reference_violation",
			wantKind: "business",
		},
		{
			name:     "check_violation",
			pgErr:    &pgconn.PgError{Code: CodeCheckViolation, ConstraintName: "status_check"},
			wantCode: "constraint_violation",
			wantKind: "business",
		},
		{
			name:     "not_null_violation",
			pgErr:    &pgconn.PgError{Code: CodeNotNullViolation, ColumnName: "name"},
			wantKind: "validation",
		},
		{
			name:     "deadlock",
			pgErr:    &pgconn.PgError{Code: CodeDeadlockDetected},
			wantCode: "conflict_retry",
			wantKind: "business",
		},
		{
			name:     "serialization_failure",
			pgErr:    &pgconn.PgError{Code: CodeSerializationFailure},
			wantCode: "conflict_retry",
			wantKind: "business",
		},
		{
			name:     "tenant_not_found",
			pgErr:    &pgconn.PgError{Code: CodeTenantNotFound},
			wantCode: "tenant.not_found",
			wantKind: "business",
		},
		{
			name:     "tenant_not_active",
			pgErr:    &pgconn.PgError{Code: CodeTenantNotActive},
			wantCode: "tenant.not_active",
			wantKind: "business",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Wrap the way pgx/v5 itself wraps driver errors, so Parse must
			// unwrap via errors.As rather than a direct type assertion.
			wrapped := fmt.Errorf("exec: %w", tt.pgErr)

			got := Parse(wrapped, "test.op")

			switch tt.wantKind {
			case "business":
				var be *runtime.BusinessError
				if !errors.As(got, &be) {
					t.Fatalf("Parse(%s) = %v (%T), want *runtime.BusinessError — "+
						"this means errors.As(err, &pgErr) failed to match the real "+
						"pgx/v5 error type, and Parse silently fell through to the "+
						"generic wrap branch", tt.name, got, got)
				}
				if be.Code != tt.wantCode {
					t.Errorf("Parse(%s) code = %q, want %q", tt.name, be.Code, tt.wantCode)
				}
			case "validation":
				var ve *runtime.ValidationError
				if !errors.As(got, &ve) {
					t.Fatalf("Parse(%s) = %v (%T), want *runtime.ValidationError", tt.name, got, got)
				}
			}
		})
	}
}

// TestIsTransient_MatchesRealPgxV5Error is the same proof for IsTransient.
func TestIsTransient_MatchesRealPgxV5Error(t *testing.T) {
	deadlock := fmt.Errorf("exec: %w", &pgconn.PgError{Code: CodeDeadlockDetected})
	if !IsTransient(deadlock) {
		t.Error("IsTransient(deadlock) = false, want true — errors.As did not match the real pgx/v5 error type")
	}

	unique := fmt.Errorf("exec: %w", &pgconn.PgError{Code: CodeUniqueViolation})
	if IsTransient(unique) {
		t.Error("IsTransient(unique_violation) = true, want false")
	}
}

// TestParse_NonPgError proves ordinary errors still get op-context wrapping,
// unaffected by the pgconn type used internally.
func TestParse_NonPgError(t *testing.T) {
	err := errors.New("connection refused")
	got := Parse(err, "test.op")
	if got == nil {
		t.Fatal("Parse(non-nil, ...) = nil")
	}
	if !errors.Is(got, err) {
		t.Errorf("Parse result does not wrap the original error: %v", got)
	}
}

func TestParse_Nil(t *testing.T) {
	if got := Parse(nil, "test.op"); got != nil {
		t.Errorf("Parse(nil, ...) = %v, want nil", got)
	}
}
