package workflow_test

import (
	"context"
	"errors"
	"testing"

	. "awo.so/awo/workflow"
)

func TestNoopExecutor_Start_ReturnsUnavailable(t *testing.T) {
	var exec NoopExecutor
	_, err := exec.Start(context.Background(), WorkflowSpec{
		WorkflowID: "test-wf-id",
		TaskQueue:  "test.queue",
	})
	if !errors.Is(err, ErrWorkflowUnavailable) {
		t.Fatalf("expected ErrWorkflowUnavailable, got %v", err)
	}
}

func TestNoopExecutor_Signal_ReturnsUnavailable(t *testing.T) {
	var exec NoopExecutor
	err := exec.Signal(context.Background(), "wf-1", "my-signal", nil)
	if !errors.Is(err, ErrWorkflowUnavailable) {
		t.Fatalf("expected ErrWorkflowUnavailable, got %v", err)
	}
}

func TestNoopExecutor_Query_ReturnsUnavailable(t *testing.T) {
	var exec NoopExecutor
	result, err := exec.Query(context.Background(), "wf-1", "my-query")
	if !errors.Is(err, ErrWorkflowUnavailable) {
		t.Fatalf("expected ErrWorkflowUnavailable, got %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result, got %v", result)
	}
}

func TestNoopExecutor_Cancel_ReturnsUnavailable(t *testing.T) {
	var exec NoopExecutor
	err := exec.Cancel(context.Background(), "wf-1")
	if !errors.Is(err, ErrWorkflowUnavailable) {
		t.Fatalf("expected ErrWorkflowUnavailable, got %v", err)
	}
}

func TestNoopExecutor_ImplementsInterface(t *testing.T) {
	var _ WorkflowExecutor = NoopExecutor{}
}

func TestErrWorkflowUnavailable_IsDistinct(t *testing.T) {
	if ErrWorkflowUnavailable == nil {
		t.Fatal("ErrWorkflowUnavailable must not be nil")
	}
}
