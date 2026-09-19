package outbox

import (
	"context"
	"encoding/json"
	"fmt"

	"awo.so/awo/def"
	"awo.so/awo/events"
	"awo.so/awo/workflow"
)

// WorkflowTriggerSubscriber dispatches durable workflow-start intents
// (events.EventWorkflowTriggerFired) to a workflow.WorkflowExecutor. It is
// the ONLY place, outside a real Temporal worker, that calls
// WorkflowExecutor.Start for a workflow-trigger event — mutation-bound code
// (runtime.ActionContext.StartWorkflow, and — once Step 6 converts it —
// api/service.EntityService.startWorkflows) never calls Temporal directly;
// it only publishes a durable outbox event this subscriber later dispatches,
// strictly after the causing transaction has already committed (ADR-025
// §4.D, §10).
//
// Register it on a Relay the same way any other events.Subscriber is
// registered:
//
//	relay.Subscribe(events.EventWorkflowTriggerFired, outbox.NewWorkflowTriggerSubscriber(executor))
//
// A dispatch failure (including workflow.ErrWorkflowUnavailable when
// executor is a workflow.NoopExecutor — Temporal not configured) is an
// ordinary HandleEvent error: the relay's own existing retry/backoff/
// dead-letter state machine (Step 3B) decides what happens next. This type
// makes no retry or terminal-failure decisions of its own.
type WorkflowTriggerSubscriber struct {
	executor workflow.WorkflowExecutor
}

// NewWorkflowTriggerSubscriber creates a subscriber that dispatches via
// executor. Pass workflow.NoopExecutor{} when Temporal is not configured
// (degraded mode) — every dispatch then fails with
// workflow.ErrWorkflowUnavailable, which HandleEvent propagates as an
// ordinary dispatch failure, retried/dead-lettered like any other.
func NewWorkflowTriggerSubscriber(executor workflow.WorkflowExecutor) *WorkflowTriggerSubscriber {
	return &WorkflowTriggerSubscriber{executor: executor}
}

var _ events.Subscriber = (*WorkflowTriggerSubscriber)(nil)

// HandleEvent unmarshals e.Payload as a def.ActionWorkflowSpec and starts the
// workflow via the configured executor. A non-nil return causes the relay to
// retry or dead-letter this event per its own existing policy (Step 3B) —
// this method never itself decides retry vs. terminal.
func (s *WorkflowTriggerSubscriber) HandleEvent(ctx context.Context, e events.DomainEvent) error {
	if e.Type != events.EventWorkflowTriggerFired {
		// Defense-in-depth only: Relay.deliver dispatches by the EventType
		// this subscriber is registered under, so this branch is not
		// expected to be reached in practice unless registered as a
		// wildcard ("") subscriber alongside other event types.
		return nil
	}

	var spec def.ActionWorkflowSpec
	if err := json.Unmarshal(e.Payload, &spec); err != nil {
		return fmt.Errorf("outbox.WorkflowTriggerSubscriber: unmarshal workflow spec: %w", err)
	}

	_, err := s.executor.Start(ctx, workflow.WorkflowSpec{
		WorkflowID: spec.WorkflowID,
		TaskQueue:  spec.TaskQueue,
		WorkflowFn: spec.WorkflowFn,
		Input:      spec.Input,
	})
	if err != nil {
		return fmt.Errorf("outbox.WorkflowTriggerSubscriber: start workflow %q: %w", spec.WorkflowID, err)
	}
	return nil
}
