package audit

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/def"
)

// TestRecordingWriter_ConcurrentWrites verifies RecordingWriter is race-safe.
func TestRecordingWriter_ConcurrentWrites(t *testing.T) {
	t.Parallel()

	rw := &RecordingWriter{}
	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			rec := AuditRecord{
				TenantID:      uuid.New(),
				EntityName:    "finance_invoice",
				Operation:     OperationCreate,
				EventCategory: CategoryData,
				Actor:         &def.Actor{UserID: uuid.New()},
			}
			_ = rw.Write(context.Background(), rec)
		}()
	}
	wg.Wait()

	if rw.Len() != goroutines {
		t.Errorf("after %d concurrent writes: Len = %d", goroutines, rw.Len())
	}
}

// TestRecordingWriter_ConcurrentReset verifies Reset is race-safe under concurrent writes.
func TestRecordingWriter_ConcurrentReset(t *testing.T) {
	t.Parallel()

	rw := &RecordingWriter{}
	const goroutines = 20

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for range goroutines {
		go func() {
			defer wg.Done()
			_ = rw.Write(context.Background(), AuditRecord{
				TenantID:      uuid.New(),
				EntityName:    "finance_invoice",
				Operation:     OperationCreate,
				EventCategory: CategoryData,
				Actor:         &def.Actor{UserID: uuid.New()},
			})
		}()
		go func() {
			defer wg.Done()
			rw.Reset()
		}()
	}
	wg.Wait()
	// No panic = race-safe.
}

// TestMultiWriter_ConcurrentWrites verifies MultiWriter fans out safely under load.
func TestMultiWriter_ConcurrentWrites(t *testing.T) {
	t.Parallel()

	r1, r2 := &RecordingWriter{}, &RecordingWriter{}
	mw := NewMultiWriter(r1, r2)
	const goroutines = 30

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			_ = mw.Write(context.Background(), AuditRecord{
				TenantID:      uuid.New(),
				EntityName:    "finance_invoice",
				Operation:     OperationCreate,
				EventCategory: CategoryData,
				Actor:         &def.Actor{UserID: uuid.New()},
			})
		}()
	}
	wg.Wait()

	if r1.Len() != goroutines {
		t.Errorf("r1 Len = %d, want %d", r1.Len(), goroutines)
	}
	if r2.Len() != goroutines {
		t.Errorf("r2 Len = %d, want %d", r2.Len(), goroutines)
	}
}

// TestRiskScorer_ConcurrentScore verifies the scorer is race-safe.
func TestRiskScorer_ConcurrentScore(t *testing.T) {
	t.Parallel()

	rs := NewRiskScorer()
	rec := &AuditRecord{
		EntityName:    "finance_invoice",
		Operation:     OperationDelete,
		EventCategory: CategoryAdmin,
		Actor:         &def.Actor{UserID: uuid.New()},
	}

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			score := rs.Score(rec)
			if score < 0 || score > 100 {
				t.Errorf("Score out of bounds: %d", score)
			}
		}()
	}
	wg.Wait()
}

// TestSanitizer_ConcurrentStrip verifies Sanitizer.Strip is race-safe.
// The sanitizer itself is immutable after construction; the test verifies
// that concurrent reads of the overrides map are safe.
func TestSanitizer_ConcurrentStrip(t *testing.T) {
	t.Parallel()

	s := NewSanitizer().WithOverrides(map[string][]string{
		"finance_invoice": {"internal_ref"},
	})

	const goroutines = 40
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			input := map[string]any{
				"name":         "ACME Corp",
				"internal_ref": "secret",
			}
			got := s.Strip("finance_invoice", input)
			if got["internal_ref"] != "[REDACTED]" {
				t.Errorf("concurrent Strip: internal_ref should be [REDACTED]")
			}
		}()
	}
	wg.Wait()
}

// TestApply_ConcurrentSuppress verifies Apply suppression is race-safe.
func TestApply_ConcurrentSuppress(t *testing.T) {
	t.Parallel()

	w := &FailingWriter{}
	rec := AuditRecord{
		TenantID:      uuid.New(),
		EntityName:    "finance_invoice",
		Operation:     OperationCreate,
		EventCategory: CategoryData, // suppress
		Actor:         &def.Actor{UserID: uuid.New()},
	}

	const goroutines = 30
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			if err := Apply(context.Background(), w, rec); err != nil {
				t.Errorf("Apply suppress: unexpected error %v", err)
			}
		}()
	}
	wg.Wait()
}
