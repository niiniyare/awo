package audit

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/def"
)

func TestRiskScorer_Warm(t *testing.T) {
	t.Parallel()

	rs := NewRiskScorer()
	// Warm is a stub in Phase 1 — must return nil.
	if err := rs.Warm(context.Background()); err != nil {
		t.Errorf("Warm() unexpected error: %v", err)
	}
}

func TestRiskScorer_Score_AllOperations(t *testing.T) {
	t.Parallel()

	rs := NewRiskScorer()
	actor := &def.Actor{UserID: uuid.New()}

	cases := []struct {
		op        OperationType
		cat       EventCategory
		wantScore int
	}{
		{OperationCreate, CategoryData, 10},
		{OperationUpdate, CategoryData, 10},
		{OperationDelete, CategoryData, 30},
		{OperationAction, CategoryData, 15},
		{OperationLogin, CategoryData, 5},
		{OperationLogout, CategoryData, 0},
		{OperationSystem, CategorySystem, 0},
		// Unknown operation falls back to 0.
		{OperationType("unknown"), CategoryData, 0},
	}

	for _, tc := range cases {
		t.Run(string(tc.op), func(t *testing.T) {
			t.Parallel()
			rec := &AuditRecord{
				EntityName:    "finance_invoice",
				Operation:     tc.op,
				EventCategory: tc.cat,
				Actor:         actor,
			}
			got := rs.Score(rec)
			if got != tc.wantScore {
				t.Errorf("Score(%s/%s) = %d, want %d", tc.op, tc.cat, got, tc.wantScore)
			}
		})
	}
}

func TestRiskScorer_Score_AllCategoryPremiums(t *testing.T) {
	t.Parallel()

	rs := NewRiskScorer()
	actor := &def.Actor{UserID: uuid.New()}

	cases := []struct {
		cat     EventCategory
		premium int
	}{
		{CategoryAdmin, 30},
		{CategorySecurity, 30},
		{CategoryAccess, 20},
		{CategoryAuth, 10},
		{CategoryData, 0},
		{CategoryWorkflow, 0},
		{CategorySystem, 0},
		{CategoryOutbound, 0},
	}

	for _, tc := range cases {
		t.Run(string(tc.cat), func(t *testing.T) {
			t.Parallel()
			rec := &AuditRecord{
				EntityName:    "finance_invoice",
				Operation:     OperationLogout, // base = 0; premium = category only
				EventCategory: tc.cat,
				Actor:         actor,
			}
			got := rs.Score(rec)
			if got != tc.premium {
				t.Errorf("categoryPremium(%s) = %d, want %d", tc.cat, got, tc.premium)
			}
		})
	}
}

func TestRiskScorer_Score_NeverNegative(t *testing.T) {
	t.Parallel()

	rs := NewRiskScorer()
	rec := &AuditRecord{
		EntityName:    "finance_invoice",
		Operation:     OperationLogout,
		EventCategory: CategoryData,
		Actor:         &def.Actor{UserID: uuid.New()},
	}
	got := rs.Score(rec)
	if got < 0 {
		t.Errorf("Score must not be negative, got %d", got)
	}
}
