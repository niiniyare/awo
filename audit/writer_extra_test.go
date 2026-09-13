package audit

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/def"
)

func TestRecordingWriter_LastPanicsWhenEmpty(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Error("Last() on empty RecordingWriter should panic")
		}
	}()

	rw := &RecordingWriter{}
	_ = rw.Last()
}

func TestRecordingWriter_Records_IsCopy(t *testing.T) {
	t.Parallel()

	rw := &RecordingWriter{}
	_ = rw.Write(context.Background(), AuditRecord{
		TenantID:      uuid.New(),
		EntityName:    "finance_invoice",
		Operation:     OperationCreate,
		EventCategory: CategoryData,
		Actor:         &def.Actor{UserID: uuid.New()},
	})

	snap := rw.Records()
	// Mutate the snapshot — original must not change.
	snap[0].EntityName = "mutated"
	if rw.Last().EntityName == "mutated" {
		t.Error("Records() must return a copy, not a reference")
	}
}

func TestFailingWriter_AlwaysReturnsError(t *testing.T) {
	t.Parallel()

	const msg = "intentional test failure"
	w := &FailingWriter{Err: errorf(msg)}
	err := w.Write(context.Background(), AuditRecord{})
	if err == nil || err.Error() != msg {
		t.Errorf("FailingWriter.Write: got %v, want error %q", err, msg)
	}
}

func TestNoopAuditWriter_ImplementsInterface(t *testing.T) {
	t.Parallel()

	// Compile-time check that NoopAuditWriter implements AuditWriter.
	var _ AuditWriter = NoopAuditWriter{}
	var _ AuditWriter = &RecordingWriter{}
	var _ AuditWriter = &FailingWriter{}
	var _ AuditWriter = &MultiWriter{}
}

func TestMultiWriter_SingleWriter(t *testing.T) {
	t.Parallel()

	rw := &RecordingWriter{}
	mw := NewMultiWriter(rw)

	rec := AuditRecord{
		TenantID:      uuid.New(),
		EntityName:    "finance_invoice",
		Operation:     OperationCreate,
		EventCategory: CategoryData,
		Actor:         &def.Actor{UserID: uuid.New()},
	}
	if err := mw.Write(context.Background(), rec); err != nil {
		t.Errorf("MultiWriter with single writer: unexpected error %v", err)
	}
	if rw.Len() != 1 {
		t.Errorf("MultiWriter with single writer: expected 1 record, got %d", rw.Len())
	}
}
