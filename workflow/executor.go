// Package workflow provides the WorkflowExecutor abstraction for async
// multi-step operations.
//
// WorkflowExecutor decouples entity code from the Temporal SDK. Entity hooks,
// action handlers, and service code call WorkflowExecutor without importing
// go.temporal.io/sdk directly. The concrete implementation (TemporalExecutor)
// is wired at startup in main.go.
//
// # Usage
//
//	var exec workflow.WorkflowExecutor = workflow.NewTemporalExecutor(client)
//
//	id, err := exec.Start(ctx, workflow.WorkflowSpec{
//	    WorkflowID: "invoice-submit-" + invoiceID.String(),
//	    TaskQueue:  "finance.invoice.submit",
//	    WorkflowFn: "SubmitInvoiceWorkflow",
//	    Input:      invoiceID,
//	})
//
// # Degraded mode
//
// When Temporal is unavailable, wire NoopExecutor. It logs a warning and
// returns ErrWorkflowUnavailable for all operations. Framework code that
// calls Start checks for this error and can choose to retry later or fail.
package workflow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	temporalclient "go.temporal.io/sdk/client"
)

// ErrWorkflowUnavailable is returned by NoopExecutor for all operations.
// Callers can check for this error to apply degraded-mode logic.
var ErrWorkflowUnavailable = errors.New("workflow: executor not configured (degraded mode)")

// WorkflowID is the unique identifier for a running workflow instance.
type WorkflowID string

// WorkflowSpec describes the workflow to start.
type WorkflowSpec struct {
	// WorkflowID is the stable, idempotency-key identifier for this workflow run.
	// Convention: "{tenant}.{entity}.{record-id}.{event}"
	// e.g. "abc123.finance_invoice.inv456.on_submit"
	WorkflowID string

	// TaskQueue is the Temporal task queue that processes this workflow.
	// Convention: "{module}.{entity}.{event}"
	// e.g. "finance.invoice.submit"
	TaskQueue string

	// WorkflowFn is the registered Temporal workflow function reference.
	// Pass the function itself (Temporal uses reflect to get the name).
	WorkflowFn any

	// Input is the workflow input. Must be JSON-serializable.
	Input any

	// ExecutionTimeout caps the total workflow execution time.
	// Zero uses the Temporal default (no timeout).
	ExecutionTimeout time.Duration

	// RunTimeout caps a single workflow run's execution time.
	// Zero uses the Temporal default.
	RunTimeout time.Duration
}

// WorkflowExecutor starts, signals, queries, and cancels Temporal workflows.
// The concrete implementation is TemporalExecutor; the stub is NoopExecutor.
type WorkflowExecutor interface {
	// Start launches a new workflow instance according to spec.
	// Returns the WorkflowID that can be used for subsequent operations.
	// Returns ErrWorkflowUnavailable if the executor is in degraded mode.
	Start(ctx context.Context, spec WorkflowSpec) (WorkflowID, error)

	// Signal sends a named signal with an optional payload to a running workflow.
	Signal(ctx context.Context, id WorkflowID, signal string, payload any) error

	// Query executes a named query against a running workflow and returns the result.
	Query(ctx context.Context, id WorkflowID, queryType string) (any, error)

	// Cancel requests graceful cancellation of a running workflow.
	Cancel(ctx context.Context, id WorkflowID) error
}

// ── NoopExecutor ─────────────────────────────────────────────────────────────

// NoopExecutor is a degraded-mode WorkflowExecutor. All operations return
// ErrWorkflowUnavailable and log a warning. Use when Temporal is not configured.
type NoopExecutor struct{}

var _ WorkflowExecutor = NoopExecutor{}

func (NoopExecutor) Start(_ context.Context, spec WorkflowSpec) (WorkflowID, error) {
	slog.Warn("workflow: Start called but no executor configured (degraded mode)",
		"workflow_id", spec.WorkflowID, "task_queue", spec.TaskQueue)
	return "", ErrWorkflowUnavailable
}

func (NoopExecutor) Signal(_ context.Context, id WorkflowID, signal string, _ any) error {
	slog.Warn("workflow: Signal called but no executor configured (degraded mode)",
		"workflow_id", id, "signal", signal)
	return ErrWorkflowUnavailable
}

func (NoopExecutor) Query(_ context.Context, id WorkflowID, queryType string) (any, error) {
	slog.Warn("workflow: Query called but no executor configured (degraded mode)",
		"workflow_id", id, "query_type", queryType)
	return nil, ErrWorkflowUnavailable
}

func (NoopExecutor) Cancel(_ context.Context, id WorkflowID) error {
	slog.Warn("workflow: Cancel called but no executor configured (degraded mode)",
		"workflow_id", id)
	return ErrWorkflowUnavailable
}

// ── TemporalExecutor ──────────────────────────────────────────────────────────

// TemporalExecutor implements WorkflowExecutor using the Temporal Go SDK client.
type TemporalExecutor struct {
	client temporalclient.Client
}

var _ WorkflowExecutor = (*TemporalExecutor)(nil)

// NewTemporalExecutor constructs a TemporalExecutor wrapping the given client.
// The client must be non-nil; use NoopExecutor when Temporal is unavailable.
func NewTemporalExecutor(client temporalclient.Client) *TemporalExecutor {
	if client == nil {
		panic("workflow: NewTemporalExecutor requires a non-nil client; use NoopExecutor for degraded mode")
	}
	return &TemporalExecutor{client: client}
}

// Start launches a Temporal workflow according to spec.
func (e *TemporalExecutor) Start(ctx context.Context, spec WorkflowSpec) (WorkflowID, error) {
	opts := temporalclient.StartWorkflowOptions{
		ID:                       spec.WorkflowID,
		TaskQueue:                spec.TaskQueue,
		WorkflowExecutionTimeout: spec.ExecutionTimeout,
		WorkflowRunTimeout:       spec.RunTimeout,
	}
	run, err := e.client.ExecuteWorkflow(ctx, opts, spec.WorkflowFn, spec.Input)
	if err != nil {
		return "", fmt.Errorf("workflow: start %q: %w", spec.WorkflowID, err)
	}
	return WorkflowID(run.GetID()), nil
}

// Signal sends a Temporal signal to the workflow identified by id.
func (e *TemporalExecutor) Signal(ctx context.Context, id WorkflowID, signal string, payload any) error {
	if err := e.client.SignalWorkflow(ctx, string(id), "", signal, payload); err != nil {
		return fmt.Errorf("workflow: signal %q/%q: %w", id, signal, err)
	}
	return nil
}

// Query executes a Temporal query against the workflow identified by id.
func (e *TemporalExecutor) Query(ctx context.Context, id WorkflowID, queryType string) (any, error) {
	val, err := e.client.QueryWorkflow(ctx, string(id), "", queryType)
	if err != nil {
		return nil, fmt.Errorf("workflow: query %q/%q: %w", id, queryType, err)
	}
	var result any
	if err := val.Get(&result); err != nil {
		return nil, fmt.Errorf("workflow: query decode %q/%q: %w", id, queryType, err)
	}
	return result, nil
}

// Cancel requests graceful cancellation of the workflow identified by id.
func (e *TemporalExecutor) Cancel(ctx context.Context, id WorkflowID) error {
	if err := e.client.CancelWorkflow(ctx, string(id), ""); err != nil {
		return fmt.Errorf("workflow: cancel %q: %w", id, err)
	}
	return nil
}
