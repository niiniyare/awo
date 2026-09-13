package workflow

import (
	"go.temporal.io/sdk/workflow"
)

// Saga coordinates compensating transactions for a sequence of activities.
// Register a compensation after each successful activity step; call Compensate
// on failure to run all compensations in LIFO order.
//
// Saga state is local to the workflow goroutine — do not share across
// workflow.Go goroutines without external synchronisation.
//
// Example usage:
//
//	var s workflow.Saga
//	if err := doStepA(ctx); err != nil {
//	    return err
//	}
//	s.AddCompensation(undoStepA)
//
//	if err := doStepB(ctx); err != nil {
//	    s.Compensate(ctx)
//	    return err
//	}
//	s.AddCompensation(undoStepB)
type Saga struct {
	compensations []func(workflow.Context) error
}

// AddCompensation registers fn to run when Compensate is called.
// Compensations run in reverse registration order (LIFO).
func (s *Saga) AddCompensation(fn func(workflow.Context) error) {
	s.compensations = append(s.compensations, fn)
}

// Compensate runs all registered compensations in LIFO order.
// Each compensation error is logged via workflow.GetLogger but does not
// prevent the remaining compensations from running.
func (s *Saga) Compensate(ctx workflow.Context) {
	logger := workflow.GetLogger(ctx)
	for i := len(s.compensations) - 1; i >= 0; i-- {
		fn := s.compensations[i]
		if err := fn(ctx); err != nil {
			logger.Error("saga compensation failed", "index", i, "error", err)
		}
	}
}
