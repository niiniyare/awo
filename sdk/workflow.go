package sdk

import "awo.so/awo/def"

// TriggerBuilder constructs a def.WorkflowTrigger with a fluent API.
type TriggerBuilder struct {
	t def.WorkflowTrigger
}

// OnCreate starts a trigger that fires on EventOnCreate.
func OnCreate(workflowFn, taskQueue string) *TriggerBuilder {
	return &TriggerBuilder{t: def.WorkflowTrigger{
		On: def.EventOnCreate, WorkflowFn: workflowFn, TaskQueue: taskQueue,
	}}
}

// OnUpdate starts a trigger that fires on EventOnUpdate.
func OnUpdate(workflowFn, taskQueue string) *TriggerBuilder {
	return &TriggerBuilder{t: def.WorkflowTrigger{
		On: def.EventOnUpdate, WorkflowFn: workflowFn, TaskQueue: taskQueue,
	}}
}

// OnDelete starts a trigger that fires on EventOnDelete.
func OnDelete(workflowFn, taskQueue string) *TriggerBuilder {
	return &TriggerBuilder{t: def.WorkflowTrigger{
		On: def.EventOnDelete, WorkflowFn: workflowFn, TaskQueue: taskQueue,
	}}
}

// OnSubmit starts a trigger that fires on EventOnSubmit.
func OnSubmit(workflowFn, taskQueue string) *TriggerBuilder {
	return &TriggerBuilder{t: def.WorkflowTrigger{
		On: def.EventOnSubmit, WorkflowFn: workflowFn, TaskQueue: taskQueue,
	}}
}

// OnApprove starts a trigger that fires on EventOnApprove.
func OnApprove(workflowFn, taskQueue string) *TriggerBuilder {
	return &TriggerBuilder{t: def.WorkflowTrigger{
		On: def.EventOnApprove, WorkflowFn: workflowFn, TaskQueue: taskQueue,
	}}
}

// OnCancel starts a trigger that fires on EventOnCancel.
func OnCancel(workflowFn, taskQueue string) *TriggerBuilder {
	return &TriggerBuilder{t: def.WorkflowTrigger{
		On: def.EventOnCancel, WorkflowFn: workflowFn, TaskQueue: taskQueue,
	}}
}

// InputFn sets the input builder function.
func (b *TriggerBuilder) InputFn(fn func(*def.EntityRecord, def.TriggerContext) (any, error)) *TriggerBuilder {
	b.t.InputBuilder = fn
	return b
}

// Build returns the completed WorkflowTrigger.
func (b *TriggerBuilder) Build() def.WorkflowTrigger { return b.t }
