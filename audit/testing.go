package audit

import (
	"context"
	"sync"
)

// RecordingWriter is an in-memory AuditWriter for use in tests. It records
// every written AuditRecord and exposes them for assertion. Safe for
// concurrent use.
type RecordingWriter struct {
	mu      sync.Mutex
	records []AuditRecord
}

// Write appends record to the internal slice. Always returns nil.
func (r *RecordingWriter) Write(_ context.Context, record AuditRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, record)
	return nil
}

// Records returns a snapshot of all written records in write order.
// The returned slice is a copy — mutations do not affect the writer's state.
func (r *RecordingWriter) Records() []AuditRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]AuditRecord, len(r.records))
	copy(out, r.records)
	return out
}

// Len returns the number of records written so far.
func (r *RecordingWriter) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.records)
}

// Reset discards all recorded records.
func (r *RecordingWriter) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = r.records[:0]
}

// Last returns the most recently written record. Panics if no records have
// been written. Use in single-operation test assertions.
func (r *RecordingWriter) Last() AuditRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.records) == 0 {
		panic("audit.RecordingWriter: Last called but no records written")
	}
	return r.records[len(r.records)-1]
}

// FailingWriter is an AuditWriter that always returns the provided error.
// Use in tests that verify failure policy behaviour.
type FailingWriter struct {
	Err error
}

// Write always returns FailingWriter.Err.
func (f *FailingWriter) Write(_ context.Context, _ AuditRecord) error {
	return f.Err
}
