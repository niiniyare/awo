package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/def"
)

func TestNoopAuditWriter(t *testing.T) {
	t.Parallel()

	var w AuditWriter = NoopAuditWriter{}
	rec := AuditRecord{
		TenantID:      uuid.New(),
		EntityName:    "any",
		Operation:     OperationCreate,
		EventCategory: CategoryData,
		Actor:         &def.Actor{UserID: uuid.New()},
	}
	if err := w.Write(context.Background(), rec); err != nil {
		t.Errorf("NoopAuditWriter.Write: unexpected error %v", err)
	}
}

func TestMultiWriter_AllWritersCalled(t *testing.T) {
	t.Parallel()

	r1, r2 := &RecordingWriter{}, &RecordingWriter{}
	mw := NewMultiWriter(r1, r2)

	rec := AuditRecord{
		TenantID:      uuid.New(),
		EntityName:    "finance_invoice",
		Operation:     OperationCreate,
		EventCategory: CategoryData,
		Actor:         &def.Actor{UserID: uuid.New()},
	}
	if err := mw.Write(context.Background(), rec); err != nil {
		t.Fatalf("MultiWriter.Write: unexpected error %v", err)
	}
	if r1.Len() != 1 {
		t.Errorf("r1: expected 1 record, got %d", r1.Len())
	}
	if r2.Len() != 1 {
		t.Errorf("r2: expected 1 record, got %d", r2.Len())
	}
}

func TestMultiWriter_StopsOnError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("first writer failed")
	r2 := &RecordingWriter{}
	mw := NewMultiWriter(&FailingWriter{Err: wantErr}, r2)

	rec := AuditRecord{
		TenantID:      uuid.New(),
		EntityName:    "finance_invoice",
		Operation:     OperationCreate,
		EventCategory: CategoryData,
		Actor:         &def.Actor{UserID: uuid.New()},
	}
	err := mw.Write(context.Background(), rec)
	if !errors.Is(err, wantErr) {
		t.Errorf("MultiWriter.Write: got %v, want %v", err, wantErr)
	}
	// Second writer must not have been called.
	if r2.Len() != 0 {
		t.Errorf("MultiWriter: second writer should not be called after error")
	}
}

func TestMultiWriter_PanicsOnNoWriters(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Error("NewMultiWriter with no writers should panic")
		}
	}()
	NewMultiWriter()
}

func TestRecordingWriter(t *testing.T) {
	t.Parallel()

	rw := &RecordingWriter{}
	if rw.Len() != 0 {
		t.Error("new RecordingWriter: Len should be 0")
	}

	rec := AuditRecord{
		TenantID:      uuid.New(),
		EntityName:    "finance_invoice",
		Operation:     OperationCreate,
		EventCategory: CategoryData,
		Actor:         &def.Actor{UserID: uuid.New()},
	}
	_ = rw.Write(context.Background(), rec)
	if rw.Len() != 1 {
		t.Errorf("after Write: Len = %d, want 1", rw.Len())
	}

	last := rw.Last()
	if last.EntityName != "finance_invoice" {
		t.Errorf("Last().EntityName = %v, want finance_invoice", last.EntityName)
	}

	rw.Reset()
	if rw.Len() != 0 {
		t.Errorf("after Reset: Len = %d, want 0", rw.Len())
	}
}
