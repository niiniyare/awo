// Package audit — context.go removed in Phase 4.
// WithWriter and WriterFromContext were deleted.
// The only AuditWriter injection mechanism is Pipeline constructor injection.
// See runtime.NewPipeline and audit.NewTransactionalWriter.
package audit
