package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/def"
)

func TestPolicyFor(t *testing.T) {
	t.Parallel()

	propagate := []EventCategory{CategoryAdmin, CategorySecurity}
	for _, cat := range propagate {
		if got := PolicyFor(cat); got != FailurePolicyPropagate {
			t.Errorf("PolicyFor(%v) = %v, want Propagate", cat, got)
		}
	}

	suppress := []EventCategory{CategoryData, CategoryAuth, CategoryAccess, CategoryWorkflow, CategorySystem, CategoryOutbound}
	for _, cat := range suppress {
		if got := PolicyFor(cat); got != FailurePolicySilentSuppress {
			t.Errorf("PolicyFor(%v) = %v, want SilentSuppress", cat, got)
		}
	}
}

func TestApply_Propagate(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("db failure")
	w := &FailingWriter{Err: wantErr}

	rec := AuditRecord{
		TenantID:      uuid.New(),
		EntityName:    "platform_tenant",
		Operation:     OperationCreate,
		EventCategory: CategoryAdmin, // → Propagate
		Actor:         &def.Actor{UserID: uuid.New()},
	}

	err := Apply(context.Background(), w, rec)
	if !errors.Is(err, wantErr) {
		t.Errorf("Apply with Propagate policy: got %v, want %v", err, wantErr)
	}
}

func TestApply_Suppress(t *testing.T) {
	t.Parallel()

	w := &FailingWriter{Err: errors.New("db failure")}

	rec := AuditRecord{
		TenantID:      uuid.New(),
		EntityName:    "finance_invoice",
		Operation:     OperationCreate,
		EventCategory: CategoryData, // → Suppress
		Actor:         &def.Actor{UserID: uuid.New()},
	}

	// Error must be suppressed — Apply returns nil.
	if err := Apply(context.Background(), w, rec); err != nil {
		t.Errorf("Apply with Suppress policy: expected nil error, got %v", err)
	}
}

func TestApply_NoError(t *testing.T) {
	t.Parallel()

	rec := AuditRecord{
		TenantID:      uuid.New(),
		EntityName:    "finance_invoice",
		Operation:     OperationCreate,
		EventCategory: CategoryAdmin,
		Actor:         &def.Actor{UserID: uuid.New()},
	}

	rw := &RecordingWriter{}
	if err := Apply(context.Background(), rw, rec); err != nil {
		t.Errorf("Apply with no error: got %v", err)
	}
	if rw.Len() != 1 {
		t.Errorf("Apply: expected 1 record written, got %d", rw.Len())
	}
}
