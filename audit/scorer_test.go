package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/def"
)

func TestRiskScorer_Score_Bounds(t *testing.T) {
	t.Parallel()

	rs := NewRiskScorer()
	rec := &AuditRecord{
		TenantID:      uuid.New(),
		EntityName:    "finance_invoice",
		Operation:     OperationDelete,
		EventCategory: CategoryAdmin,
		Actor:         &def.Actor{UserID: uuid.New()},
	}
	score := rs.Score(rec)
	if score < 0 || score > 100 {
		t.Errorf("Score() = %d, must be in [0, 100]", score)
	}
}

func TestRiskScorer_Score_DeleteAdmin(t *testing.T) {
	t.Parallel()

	rs := NewRiskScorer()
	rec := &AuditRecord{
		EntityName:    "platform_tenant",
		Operation:     OperationDelete, // base = 30
		EventCategory: CategoryAdmin,   // premium = +30
		Actor:         &def.Actor{UserID: uuid.New()},
	}
	got := rs.Score(rec)
	// 30 + 30 = 60, no sensitive entity premium.
	if got != 60 {
		t.Errorf("Score(delete/admin) = %d, want 60", got)
	}
}

func TestRiskScorer_Score_Logout(t *testing.T) {
	t.Parallel()

	rs := NewRiskScorer()
	rec := &AuditRecord{
		EntityName:    "iam_session",
		Operation:     OperationLogout, // base = 0
		EventCategory: CategoryAuth,    // premium = +10
		Actor:         &def.Actor{UserID: uuid.New()},
	}
	got := rs.Score(rec)
	if got != 10 {
		t.Errorf("Score(logout/auth) = %d, want 10", got)
	}
}

func TestRiskScorer_Score_Cap(t *testing.T) {
	t.Parallel()

	rs := NewRiskScorer()
	// delete(30) + SECURITY(30) = 60; capped at 100 even with sensitive premium.
	rec := &AuditRecord{
		EntityName:    "finance_invoice",
		Operation:     OperationDelete,
		EventCategory: CategorySecurity,
		Actor:         &def.Actor{UserID: uuid.New()},
	}
	got := rs.Score(rec)
	if got > 100 {
		t.Errorf("Score must be capped at 100, got %d", got)
	}
}

func TestSeverityFromScore_Thresholds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		score int
		want  Severity
	}{
		{0, SeverityInfo},
		{9, SeverityInfo},
		{10, SeverityLow},
		{29, SeverityLow},
		{30, SeverityMedium},
		{49, SeverityMedium},
		{50, SeverityHigh},
		{69, SeverityHigh},
		{70, SeverityCritical},
		{100, SeverityCritical},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			got := severityFromScore(tc.score)
			if got != tc.want {
				t.Errorf("severityFromScore(%d) = %q, want %q", tc.score, got, tc.want)
			}
		})
	}
}

// TestRiskScorer_Warm_WithLoader verifies that Warm() populates sensitive
// entities and they receive the +20 premium in subsequent Score() calls.
func TestRiskScorer_Warm_WithLoader(t *testing.T) {
	t.Parallel()

	rs := NewRiskScorer().WithLoader(func(_ context.Context) ([]string, error) {
		return []string{"finance_invoice"}, nil
	})
	if err := rs.Warm(context.Background()); err != nil {
		t.Fatalf("Warm: unexpected error %v", err)
	}

	rec := &AuditRecord{
		EntityName:    "finance_invoice",
		Operation:     OperationCreate, // base = 10
		EventCategory: CategoryData,    // premium = 0
		Actor:         &def.Actor{UserID: uuid.New()},
	}
	// 10 + 0 + 20 (sensitive) = 30
	if got := rs.Score(rec); got != 30 {
		t.Errorf("Score after Warm with sensitive entity = %d, want 30", got)
	}

	// Non-sensitive entity must not receive the premium.
	other := &AuditRecord{
		EntityName:    "platform_tenant",
		Operation:     OperationCreate,
		EventCategory: CategoryData,
		Actor:         &def.Actor{UserID: uuid.New()},
	}
	if got := rs.Score(other); got != 10 {
		t.Errorf("Score for non-sensitive entity = %d, want 10", got)
	}
}

// TestRiskScorer_Warm_NoLoader verifies the scorer works with default rules
// when no loader is configured.
func TestRiskScorer_Warm_NoLoader(t *testing.T) {
	t.Parallel()

	rs := NewRiskScorer() // no WithLoader
	if err := rs.Warm(context.Background()); err != nil {
		t.Fatalf("Warm with no loader: unexpected error %v", err)
	}
	rec := &AuditRecord{
		EntityName:    "finance_invoice",
		Operation:     OperationCreate,
		EventCategory: CategoryData,
		Actor:         &def.Actor{UserID: uuid.New()},
	}
	// 10 + 0 + 0 = 10 (no sensitive premium without loader)
	if got := rs.Score(rec); got != 10 {
		t.Errorf("Score with no loader = %d, want 10", got)
	}
}

// TestRiskScorer_Warm_LoaderError verifies that a loader error causes Warm
// to return the error while leaving the scorer on default rules.
func TestRiskScorer_Warm_LoaderError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("db unavailable")
	rs := NewRiskScorer().WithLoader(func(_ context.Context) ([]string, error) {
		return nil, wantErr
	})
	err := rs.Warm(context.Background())
	if !errors.Is(err, wantErr) {
		t.Errorf("Warm loader error: got %v, want %v", err, wantErr)
	}
	// Scorer must still be operational (default rules) after error.
	rec := &AuditRecord{
		EntityName:    "finance_invoice",
		Operation:     OperationCreate,
		EventCategory: CategoryData,
		Actor:         &def.Actor{UserID: uuid.New()},
	}
	if got := rs.Score(rec); got < 0 || got > 100 {
		t.Errorf("Score after loader error = %d, must be in [0,100]", got)
	}
}

// TestRiskScorer_Warm_EmptyLoader verifies that nil, nil from a loader
// is treated as "no sensitive entities" without error.
func TestRiskScorer_Warm_EmptyLoader(t *testing.T) {
	t.Parallel()

	rs := NewRiskScorer().WithLoader(func(_ context.Context) ([]string, error) {
		return nil, nil // table not yet migrated
	})
	if err := rs.Warm(context.Background()); err != nil {
		t.Fatalf("Warm with empty loader: unexpected error %v", err)
	}
}
