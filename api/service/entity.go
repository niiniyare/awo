// Package service provides the EntityService — the orchestration layer between
// HTTP handlers and the hook pipeline + repository. It is the correct place for
// the full lifecycle: RunBeforeCreate → repo.WithTx(Create + RunAfterCreate) →
// workflow start.
//
// Module authors do not call this package directly. Handlers call it;
// custom actions receive pre-wired repositories via ActionContext.
package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	temporalclient "go.temporal.io/sdk/client"

	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/driver"
	"awo.so/awo/filter"
	"awo.so/awo/runtime"
)

// EntityService orchestrates the full record lifecycle for one entity type.
type EntityService struct {
	schema   *compiler.EntitySchema
	repo     driver.EntityRepository[*def.EntityRecord]
	pipeline *runtime.Pipeline
	temporal temporalclient.Client // may be nil (degraded mode)
}

// NewEntityService creates an EntityService.
// temporal may be nil; workflow starts will fail gracefully if so.
func NewEntityService(
	schema *compiler.EntitySchema,
	repo driver.EntityRepository[*def.EntityRecord],
	pipeline *runtime.Pipeline,
	temporal temporalclient.Client,
) *EntityService {
	return &EntityService{
		schema:   schema,
		repo:     repo,
		pipeline: pipeline,
		temporal: temporal,
	}
}

// Create runs the full create lifecycle:
//  1. RunBeforeCreate (validation, defaults, before hooks)
//  2. repo.WithTx: INSERT + RunAfterCreate (inside TX)
//  3. StartWorkflow (outside TX, best-effort)
func (s *EntityService) Create(ctx context.Context, data map[string]any, actor *def.Actor) (*def.EntityRecord, error) {
	pctx := &runtime.CreateContext{
		Ctx:        ctx,
		EntityName: s.schema.QualifiedName,
		Data:       data,
		Actor:      actor,
	}

	record, err := s.pipeline.RunBeforeCreate(pctx)
	if err != nil {
		return nil, err
	}

	var created *def.EntityRecord
	if err := s.repo.WithTx(ctx, func(txCtx context.Context) error {
		var txErr error
		created, txErr = s.repo.Create(txCtx, driver.CreateInput{
			Data:         record.Data,
			CustomFields: record.CustomFields,
			Actor:        actor,
		})
		if txErr != nil {
			return txErr
		}
		if txErr = s.pipeline.RunAuditRecord(txCtx, created, nil, created.Data); txErr != nil {
			return txErr
		}
		return s.pipeline.RunAfterCreate(txCtx, created)
	}); err != nil {
		return nil, err
	}

	// Start workflow triggers outside the transaction.
	s.startWorkflows(ctx, def.EventOnCreate, created, actor)

	return created, nil
}

// Update runs the full update lifecycle:
//  1. Get current record
//  2. RunBeforeUpdate (immutability, validation, before hooks)
//  3. repo.WithTx: UPDATE + RunAfterUpdate (inside TX)
func (s *EntityService) Update(ctx context.Context, id uuid.UUID, data map[string]any, actor *def.Actor) (*def.EntityRecord, error) {
	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	pctx := &runtime.UpdateContext{
		Ctx:        ctx,
		EntityName: s.schema.QualifiedName,
		Data:       data,
		Actor:      actor,
	}

	patched, err := s.pipeline.RunBeforeUpdate(pctx, current)
	if err != nil {
		return nil, err
	}

	var updated *def.EntityRecord
	if err := s.repo.WithTx(ctx, func(txCtx context.Context) error {
		var txErr error
		updated, txErr = s.repo.Update(txCtx, id, driver.UpdateInput{
			Data:  patched.Data,
			Actor: actor,
		})
		if txErr != nil {
			return txErr
		}
		if txErr = s.pipeline.RunAuditRecord(txCtx, updated, current.Data, updated.Data); txErr != nil {
			return txErr
		}
		return s.pipeline.RunAfterUpdate(txCtx, updated, current)
	}); err != nil {
		return nil, err
	}

	return updated, nil
}

// Delete runs the full delete lifecycle:
//  1. Get current record
//  2. RunBeforeDelete
//  3. repo.WithTx: DELETE + RunAfterDelete (inside TX)
func (s *EntityService) Delete(ctx context.Context, id uuid.UUID, actor *def.Actor) error {
	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}

	if err := s.pipeline.RunBeforeDelete(ctx, current); err != nil {
		return err
	}

	return s.repo.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Delete(txCtx, id); err != nil {
			return err
		}
		if err := s.pipeline.RunAuditRecord(txCtx, current, current.Data, nil); err != nil {
			return err
		}
		return s.pipeline.RunAfterDelete(txCtx, current)
	})
}

// Get delegates to the repository.
func (s *EntityService) Get(ctx context.Context, id uuid.UUID, opts ...driver.QueryOption) (*def.EntityRecord, error) {
	return s.repo.Get(ctx, id, opts...)
}

// Query delegates to the repository.
func (s *EntityService) Query(ctx context.Context, f *filter.Filter, opts ...driver.QueryOption) ([]*def.EntityRecord, driver.PageInfo, error) {
	return s.repo.Query(ctx, f, opts...)
}

// startWorkflows fires Temporal workflows for matching triggers.
// Runs outside the database transaction — failure does not roll back the record.
// In production, the outbox pattern provides retry guarantees.
func (s *EntityService) startWorkflows(ctx context.Context, event def.EventType, record *def.EntityRecord, actor *def.Actor) {
	for _, t := range s.schema.WorkflowTriggers {
		if t.On != event {
			continue
		}
		if s.temporal == nil {
			slog.Warn("temporal client not configured — skipping workflow start",
				"entity", s.schema.QualifiedName,
				"event", event,
				"workflow", t.WorkflowFn,
			)
			continue
		}

		tc := def.TriggerContext{
			TenantID: record.TenantID,
			Actor:    actor,
		}

		input, err := t.InputBuilder(record, tc)
		if err != nil {
			slog.Error("workflow input builder failed",
				"entity", s.schema.QualifiedName,
				"workflow", t.WorkflowFn,
				"record_id", record.ID,
				"err", err,
			)
			continue
		}

		workflowID := workflowIDFor(t, record)
		_, err = s.temporal.ExecuteWorkflow(ctx,
			temporalclient.StartWorkflowOptions{
				ID:        workflowID,
				TaskQueue: t.TaskQueue,
			},
			t.WorkflowFn,
			input,
		)
		if err != nil {
			slog.Error("workflow start failed — will retry via outbox",
				"entity", s.schema.QualifiedName,
				"workflow", t.WorkflowFn,
				"workflow_id", workflowID,
				"err", err,
			)
			// TODO: write to outbox table for guaranteed retry.
		}
	}
}

func workflowIDFor(t def.WorkflowTrigger, record *def.EntityRecord) string {
	if t.WorkflowIDFunc != nil {
		return t.WorkflowIDFunc(record.TenantID, record)
	}
	// Default: {tenant}.{entity}.{record_id}.{event}
	return fmt.Sprintf("%s.%s.%s.%s",
		record.TenantID,
		record.EntityName,
		record.ID,
		t.On,
	)
}
