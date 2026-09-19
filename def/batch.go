package def

// BatchCreateResult is the outcome of a canonical batch-create operation
// (Phase 2 Step 5 — bulk import through the mutation pipeline,
// PHASE2_ARCHITECTURE_PLAN.md Step 5, ADR-025 §14). It is the return type of
// api/service.EntityService.CreateBatch, referenced from ioport (which does
// not import api/service — see ioport.EntityBatchMutator) so both packages
// can share one result shape without a new package dependency edge, since
// both already depend on def.
type BatchCreateResult struct {
	// Created holds every record whose RunBeforeCreate, persist, audit,
	// after-hook, and outbox stages all succeeded — all from the same
	// shared flush transaction, which commits only if every one of these
	// rows' stages succeeded (per-flush atomicity: all rows here committed
	// together, or Created is empty and Err (returned alongside this
	// result) is non-nil).
	Created []*EntityRecord

	// Skipped holds rows excluded from the flush transaction because
	// RunBeforeCreate (defaults, validation, before hooks) failed for
	// them — populated only when the caller opted into per-row skip
	// semantics (ioport.ImportOptions.SkipErrors). A skipped row never
	// reaches the database: it is not counted as created, audited, or
	// published.
	Skipped []BatchRowError
}

// BatchRowError attributes a pre-persist (RunBeforeCreate) failure to its
// 0-based position within the rows slice passed to a batch-create call.
type BatchRowError struct {
	// Index is the 0-based position of the row within the input slice.
	Index int

	// Err is the error RunBeforeCreate returned for this row.
	Err error
}
