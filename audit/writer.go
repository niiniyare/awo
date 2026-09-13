package audit

import "context"

// AuditWriter is the central contract for the Unified Audit System.
// Implementations write AuditRecord values to durable storage.
//
// Write is called from within an active database transaction (for entity
// mutations) or outside a transaction (for standalone auth/security events).
// Implementations must handle both cases.
//
// Whether a Write error propagates or is suppressed depends on the
// FailurePolicy for the record's EventCategory. See FailurePolicy.
type AuditWriter interface {
	Write(ctx context.Context, record AuditRecord) error
}

// NoopAuditWriter discards all records silently.
// Use in unit tests that do not test audit behaviour.
type NoopAuditWriter struct{}

func (NoopAuditWriter) Write(_ context.Context, _ AuditRecord) error { return nil }

// MultiWriter fans out a Write call to each writer in order, stopping on the
// first error. Used to compose writers (e.g. transactional + metrics writer).
type MultiWriter struct {
	writers []AuditWriter
}

// NewMultiWriter returns a MultiWriter that writes to each provided writer.
// At least one writer must be provided.
func NewMultiWriter(writers ...AuditWriter) *MultiWriter {
	if len(writers) == 0 {
		panic("audit.NewMultiWriter: at least one writer is required")
	}
	return &MultiWriter{writers: writers}
}

// Write calls each underlying writer in order. Returns the first error
// encountered; subsequent writers are not called after an error.
func (m *MultiWriter) Write(ctx context.Context, record AuditRecord) error {
	for _, w := range m.writers {
		if err := w.Write(ctx, record); err != nil {
			return err
		}
	}
	return nil
}
