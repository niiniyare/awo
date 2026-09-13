package def

import "github.com/google/uuid"

// WorkflowTrigger binds a Temporal workflow start to an entity lifecycle
// event. The framework starts the workflow OUTSIDE the database transaction,
// after the transaction commits. If the workflow start fails, the failure is
// recorded in the retry queue for re-attempt — the entity record is not
// rolled back.
//
// Workflow ID convention: "{tenant-uuid}.{entity-name}.{record-id}.{event}"
// e.g. "abc123.finance_invoice.inv456.on_submit"
//
// # Determinism requirement
//
// The InputBuilder must be deterministic and free of I/O. It runs in the
// request path, after the transaction commits. Heavy computation or network
// calls must happen inside Temporal activities.
type WorkflowTrigger struct {
	// On is the lifecycle event that activates this trigger.
	On EventType

	// WorkflowFn is the registered Temporal workflow function name.
	// Must match the name used in temporal.RegisterWorkflow(...).
	WorkflowFn string

	// TaskQueue is the Temporal task queue that processes this workflow.
	// Convention: "{module}.{entity}.{event}" e.g. "finance.invoice.submit"
	TaskQueue string

	// InputBuilder constructs the workflow input from the saved record.
	// Return (nil, nil) to pass no input. The returned value must be
	// JSON-serialisable.
	InputBuilder func(record *EntityRecord, ctx TriggerContext) (any, error)

	// WorkflowIDFunc overrides the default workflow ID generation. Return ""
	// to use the framework default convention.
	WorkflowIDFunc func(tenantID uuid.UUID, record *EntityRecord) string

	// SearchAttributes are Temporal search attributes attached to the workflow
	// execution. Key-value pairs; values must be Temporal-compatible types.
	SearchAttributes map[string]any
}
