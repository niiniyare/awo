// Package service provides the EntityService — the orchestration layer between
// HTTP handlers and the hook pipeline + repository. It is the canonical
// mutation pipeline for normal entity mutations (PHASE2_ARCHITECTURE_PLAN.md
// §6, ADR-025 §5):
//
//	RunBeforeCreate (validate, before hooks — outside TX)
//	  → repo.WithTx:
//	      PERSIST → RunAuditRecord (audit, ADR-017 policy) → RunAfterCreate
//	      (after hooks) → publishLifecycleEvent (durable outbox write,
//	      unconditional — ADR-025 §6)
//	  → COMMIT
//	  → workflow start (outside TX, best-effort — durability fix is Step 6)
//
// Module authors do not call this package directly. Handlers call it;
// custom actions receive pre-wired repositories via ActionContext.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	temporalclient "go.temporal.io/sdk/client"

	"awo.so/awo/audit"
	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/driver"
	"awo.so/awo/events"
	"awo.so/awo/filter"
	"awo.so/awo/runtime"
)

// EntityService orchestrates the full record lifecycle for one entity type.
type EntityService struct {
	schema    *compiler.EntitySchema
	repo      driver.EntityRepository[*def.EntityRecord]
	pipeline  *runtime.Pipeline
	temporal  temporalclient.Client // may be nil (degraded mode)
	publisher events.Publisher      // durable outbox write for Create/Update/Delete
	sanitizer *audit.Sanitizer      // redacts Sensitive fields from outbox payloads
}

// NewEntityService creates an EntityService.
// temporal may be nil; workflow starts will fail gracefully if so.
// The event publisher defaults to events.NoopPublisher{} (no outbox writes)
// and the sanitizer to audit.NewSanitizer() (compile-time Sensitive-field
// redaction, no runtime DB overrides) — override either via WithPublisher /
// WithSanitizer. This matches this constructor's existing degraded-mode
// convention for temporal (nil is accepted, not required).
func NewEntityService(
	schema *compiler.EntitySchema,
	repo driver.EntityRepository[*def.EntityRecord],
	pipeline *runtime.Pipeline,
	temporal temporalclient.Client,
) *EntityService {
	return &EntityService{
		schema:    schema,
		repo:      repo,
		pipeline:  pipeline,
		temporal:  temporal,
		publisher: events.NoopPublisher{},
		sanitizer: audit.NewSanitizer(),
	}
}

// WithPublisher sets the transactional-outbox publisher used to emit durable
// domain events for Create/Update/Delete mutations, inside the same
// transaction as the mutation and audit write. A nil argument is ignored
// (the existing publisher, or the NoopPublisher{} default, is kept) so this
// method is always safe to chain.
func (s *EntityService) WithPublisher(pub events.Publisher) *EntityService {
	if pub != nil {
		s.publisher = pub
	}
	return s
}

// WithSanitizer overrides the sanitizer used to redact Sensitive fields from
// outbox event payloads before they are marshaled and published. A nil
// argument is ignored (the existing sanitizer, or the audit.NewSanitizer()
// default, is kept).
func (s *EntityService) WithSanitizer(sz *audit.Sanitizer) *EntityService {
	if sz != nil {
		s.sanitizer = sz
	}
	return s
}

// Create runs the full create lifecycle:
//  1. RunBeforeCreate (validation, defaults, before hooks)
//  2. repo.WithTx: INSERT + RunAuditRecord + RunAfterCreate + durable outbox
//     event, all inside the same transaction (ADR-025 §5)
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
		if txErr = s.pipeline.RunAfterCreate(txCtx, created); txErr != nil {
			return txErr
		}
		return s.publishLifecycleEvent(txCtx, events.EventCreated, created, actor)
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
//  3. repo.WithTx: UPDATE + RunAuditRecord + RunAfterUpdate + durable outbox
//     event, all inside the same transaction (ADR-025 §5)
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
		if txErr = s.pipeline.RunAfterUpdate(txCtx, updated, current); txErr != nil {
			return txErr
		}
		return s.publishLifecycleEvent(txCtx, events.EventUpdated, updated, actor)
	}); err != nil {
		return nil, err
	}

	return updated, nil
}

// Delete runs the full delete lifecycle:
//  1. Get current record
//  2. RunBeforeDelete
//  3. repo.WithTx: DELETE + RunAuditRecord + RunAfterDelete + durable outbox
//     event, all inside the same transaction (ADR-025 §5)
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
		if err := s.pipeline.RunAfterDelete(txCtx, current); err != nil {
			return err
		}
		// current is the last known state — there is no "after" snapshot for
		// a deleted record, matching RunAuditRecord's own before/after=nil
		// convention for Delete above.
		return s.publishLifecycleEvent(txCtx, events.EventDeleted, current, actor)
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

// Count delegates to the repository. Added alongside Exists/WithTx (Phase 2
// Step 4) so AsActionRepo can expose the full def.ActionEntityRepo surface
// without reaching around EntityService into the raw repository directly.
func (s *EntityService) Count(ctx context.Context, f *filter.Filter) (int64, error) {
	return s.repo.Count(ctx, f)
}

// Exists delegates to the repository.
func (s *EntityService) Exists(ctx context.Context, f *filter.Filter) (bool, error) {
	return s.repo.Exists(ctx, f)
}

// WithTx delegates to the repository's transaction boundary. Exposed so
// runtime.ActionContext's TxFn (Phase 2 Step 4) can open a transaction using
// the exact same mechanism — and, when already inside one (the
// InTx() short-circuit), the exact same transaction — every other mutation
// on this entity uses. This is not a second transaction-ownership
// mechanism; it is the same one, reached through EntityService instead of
// the raw repository.
func (s *EntityService) WithTx(ctx context.Context, fn func(context.Context) error) error {
	return s.repo.WithTx(ctx, fn)
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

// publishLifecycleEvent constructs and publishes a durable domain event for a
// successful Create/Update/Delete mutation, called as the last statement
// inside the same repo.WithTx callback as the mutation and audit write
// (PHASE2_ARCHITECTURE_PLAN.md §6, §10; ADR-025 §5, §6).
//
// The payload is record's own field data (Data merged with CustomFields) —
// the persisted state at the point this is called, never the raw request
// body — passed through the sanitizer so Sensitive fields never reach the
// outbox (ADR-025 §7). For Delete, callers pass the pre-deletion record
// (there is no "after" state), matching RunAuditRecord's own convention.
//
// actor is taken directly from Create/Update/Delete's own parameter, not
// from record.Meta.Actor: contrib/pgx.Repository's Create/Update do not
// populate Meta.Actor on the *def.EntityRecord they return (confirmed by
// reading contrib/pgx/repo.go's row-scanning helpers, which construct a
// fresh record with no Meta field set at all) — a pre-existing gap, also
// affecting RunAuditRecord's own Actor field for real (non-test-stub)
// repositories, not introduced or fixed by this change. Using the caller's
// own actor parameter directly sidesteps that gap rather than depending on
// it being fixed.
//
// An error here is NOT subject to ADR-017's audit-failure leniency: per
// ADR-025 §6 it always propagates, so returning it from this method's caller
// (the WithTx callback) fails the whole transaction — mutation, audit, and
// outbox roll back together, exactly like any other error already returned
// from that callback. No new rollback mechanism is introduced.
func (s *EntityService) publishLifecycleEvent(ctx context.Context, eventType events.EventType, record *def.EntityRecord, actor *def.Actor) error {
	snapshot := make(map[string]any, len(record.Data)+len(record.CustomFields))
	for k, v := range record.Data {
		snapshot[k] = v
	}
	for k, v := range record.CustomFields {
		snapshot[k] = v
	}
	sanitized := s.sanitizer.Strip(s.schema.QualifiedName, snapshot)

	payload, err := json.Marshal(sanitized)
	if err != nil {
		return fmt.Errorf("entity service: marshal event payload: %w", err)
	}

	var actorID uuid.UUID
	if actor != nil {
		actorID = actor.UserID
	}

	var correlationID string
	if rc, ok := audit.RequestContextFromContext(ctx); ok {
		correlationID = rc.RequestID
	}

	return s.publisher.Publish(ctx, events.DomainEvent{
		TenantID:      record.TenantID,
		Type:          eventType,
		EntityName:    s.schema.QualifiedName,
		RecordID:      record.ID,
		ActorID:       actorID,
		CorrelationID: correlationID,
		Payload:       payload,
	})
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
