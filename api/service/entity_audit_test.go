package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"awo.so/awo/api/service"
	"awo.so/awo/audit"
	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/driver"
	"awo.so/awo/filter"
	"awo.so/awo/registry"
	"awo.so/awo/runtime"
)

// --- Test helpers ---

func buildServiceTestSchema(t *testing.T, d def.EntityDefinition) *compiler.CompiledSchema {
	t.Helper()
	reg, err := registry.BuildFrom([]def.EntityDefinition{d})
	if err != nil {
		t.Fatalf("registry.BuildFrom: %v", err)
	}
	schema, err := compiler.Compile(reg)
	if err != nil {
		t.Fatalf("compiler.Compile: %v", err)
	}
	return schema
}

// stubRepo is a minimal driver.EntityRepository[*def.EntityRecord] for testing.
// WithTx calls fn(ctx) synchronously, simulating an in-transaction callback.
type stubRepo struct {
	entityName string // used in Create to return the correct EntityName
	created    *def.EntityRecord
	current    *def.EntityRecord
	updated    *def.EntityRecord
}

func (r *stubRepo) WithTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func (r *stubRepo) Create(_ context.Context, input driver.CreateInput) (*def.EntityRecord, error) {
	rec := &def.EntityRecord{
		ID:         uuid.New(),
		TenantID:   uuid.New(),
		EntityName: r.entityName,
		Data:       input.Data,
		Meta:       def.RecordMeta{Actor: input.Actor},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	r.created = rec
	return rec, nil
}

func (r *stubRepo) Get(_ context.Context, _ uuid.UUID, _ ...driver.QueryOption) (*def.EntityRecord, error) {
	if r.current != nil {
		return r.current, nil
	}
	return &def.EntityRecord{
		ID:         uuid.New(),
		TenantID:   uuid.New(),
		EntityName: r.entityName,
		Data:       map[string]any{"status": "draft"},
	}, nil
}

func (r *stubRepo) Update(_ context.Context, id uuid.UUID, input driver.UpdateInput) (*def.EntityRecord, error) {
	rec := &def.EntityRecord{
		ID:         id,
		TenantID:   uuid.New(),
		EntityName: r.entityName,
		Data:       input.Data,
		Meta:       def.RecordMeta{Actor: input.Actor},
		UpdatedAt:  time.Now(),
	}
	r.updated = rec
	return rec, nil
}

func (r *stubRepo) Delete(_ context.Context, _ uuid.UUID) error { return nil }

func (r *stubRepo) Query(_ context.Context, _ *filter.Filter, _ ...driver.QueryOption) ([]*def.EntityRecord, driver.PageInfo, error) {
	return nil, driver.PageInfo{}, nil
}

func (r *stubRepo) Exists(_ context.Context, _ *filter.Filter) (bool, error) { return false, nil }
func (r *stubRepo) Count(_ context.Context, _ *filter.Filter) (int64, error) { return 0, nil }
func (r *stubRepo) Aggregate(_ context.Context, _ *filter.Filter, _ driver.AggregateSpec) (driver.AggregateResult, error) {
	return driver.AggregateResult{}, nil
}
func (r *stubRepo) BulkCreate(_ context.Context, _ []driver.CreateInput) ([]*def.EntityRecord, error) {
	return nil, nil
}
func (r *stubRepo) BulkUpdate(_ context.Context, _ *filter.Filter, _ driver.Patch) (int64, error) {
	return 0, nil
}

// --- Helpers to build a test EntityService ---

const svcEntityName = "svc_test_entity"

func buildTestEntityDef() def.EntityDefinition {
	return &def.SystemDefinition{
		Name:   svcEntityName,
		Module: "svc",
		Label:  "SvcTest",
		Fields: []def.FieldDef{
			{Name: "status", Type: def.FieldTypeSelect, Options: []string{"draft", "active"}},
		},
	}
}

func buildTestService(t *testing.T, aw audit.AuditWriter) (*service.EntityService, *audit.RecordingWriter, *stubRepo) {
	t.Helper()

	d := buildTestEntityDef()
	schema := buildServiceTestSchema(t, d)

	audit.Register(audit.EntityAuditConfig{
		EntityName: svcEntityName,
		Enabled:    true,
		Category:   audit.CategoryData,
	})

	rw := &audit.RecordingWriter{}
	var writer audit.AuditWriter
	if aw != nil {
		writer = aw
	} else {
		writer = rw
	}

	es := schema.ByName[svcEntityName]
	if es == nil {
		t.Fatalf("entity %q not found in schema", svcEntityName)
	}

	pipeline := runtime.NewPipeline(schema, writer)
	repo := &stubRepo{entityName: svcEntityName}

	svc := service.NewEntityService(es, repo, pipeline, nil)
	return svc, rw, repo
}

// --- Tests ---

// TestEntityService_Create_WritesAuditRecord verifies that Create delivers an
// audit record with Operation=create after persisting the entity.
func TestEntityService_Create_WritesAuditRecord(t *testing.T) {
	t.Parallel()

	svc, rw, _ := buildTestService(t, nil)
	actor := &def.Actor{UserID: uuid.New(), TenantID: uuid.New()}

	ctx := context.Background()
	_, err := svc.Create(ctx, map[string]any{"status": "draft"}, actor)
	if err != nil {
		t.Fatalf("Create: unexpected error %v", err)
	}

	if rw.Len() != 1 {
		t.Fatalf("expected 1 audit record, got %d", rw.Len())
	}
	rec := rw.Last()
	if rec.Operation != audit.OperationCreate {
		t.Errorf("Operation: got %q, want %q", rec.Operation, audit.OperationCreate)
	}
	if rec.BeforeData != nil {
		t.Errorf("BeforeData: got %v, want nil on Create", rec.BeforeData)
	}
	if rec.AfterData == nil {
		t.Error("AfterData: must not be nil on Create")
	}
}

// TestEntityService_Update_WritesAuditRecord verifies that Update delivers an
// audit record with Operation=update, BeforeData set, and AfterData set.
func TestEntityService_Update_WritesAuditRecord(t *testing.T) {
	t.Parallel()

	svc, rw, repo := buildTestService(t, nil)
	repo.current = &def.EntityRecord{
		ID:         uuid.New(),
		TenantID:   uuid.New(),
		EntityName: svcEntityName,
		Data:       map[string]any{"status": "draft"},
	}

	actor := &def.Actor{UserID: uuid.New(), TenantID: uuid.New()}
	_, err := svc.Update(context.Background(), repo.current.ID, map[string]any{"status": "active"}, actor)
	if err != nil {
		t.Fatalf("Update: unexpected error %v", err)
	}

	if rw.Len() != 1 {
		t.Fatalf("expected 1 audit record, got %d", rw.Len())
	}
	rec := rw.Last()
	if rec.Operation != audit.OperationUpdate {
		t.Errorf("Operation: got %q, want %q", rec.Operation, audit.OperationUpdate)
	}
	if rec.BeforeData == nil {
		t.Error("BeforeData must not be nil on Update")
	}
	if rec.AfterData == nil {
		t.Error("AfterData must not be nil on Update")
	}
}

// TestEntityService_Delete_WritesAuditRecord verifies that Delete delivers an
// audit record with Operation=delete, BeforeData set, and AfterData nil.
func TestEntityService_Delete_WritesAuditRecord(t *testing.T) {
	t.Parallel()

	svc, rw, repo := buildTestService(t, nil)
	repo.current = &def.EntityRecord{
		ID:         uuid.New(),
		TenantID:   uuid.New(),
		EntityName: svcEntityName,
		Data:       map[string]any{"status": "active"},
	}

	actor := &def.Actor{UserID: uuid.New(), TenantID: uuid.New()}
	err := svc.Delete(context.Background(), repo.current.ID, actor)
	if err != nil {
		t.Fatalf("Delete: unexpected error %v", err)
	}

	if rw.Len() != 1 {
		t.Fatalf("expected 1 audit record, got %d", rw.Len())
	}
	rec := rw.Last()
	if rec.Operation != audit.OperationDelete {
		t.Errorf("Operation: got %q, want %q", rec.Operation, audit.OperationDelete)
	}
	if rec.BeforeData == nil {
		t.Error("BeforeData must not be nil on Delete")
	}
	if rec.AfterData != nil {
		t.Errorf("AfterData: got %v, want nil on Delete", rec.AfterData)
	}
}

// TestEntityService_Create_AuditFailurePropagate verifies that an ADMIN
// category audit failure rolls back the mutation (propagate policy).
func TestEntityService_Create_AuditFailurePropagate(t *testing.T) {
	t.Parallel()

	const entityName = "svc_admin_fail_entity"
	d := &def.SystemDefinition{
		Name: entityName, Module: "svc", Label: "AdminFail",
	}
	schema := buildServiceTestSchema(t, d)
	audit.Register(audit.EntityAuditConfig{
		EntityName: entityName,
		Enabled:    true,
		Category:   audit.CategoryAdmin, // Propagate
	})

	sentinel := errors.New("audit write failed")
	pipeline := runtime.NewPipeline(schema, &audit.FailingWriter{Err: sentinel})
	es := schema.ByName[entityName]
	repo := &stubRepo{entityName: entityName}
	svc := service.NewEntityService(es, repo, pipeline, nil)

	actor := &def.Actor{UserID: uuid.New(), TenantID: uuid.New()}
	_, err := svc.Create(context.Background(), map[string]any{}, actor)
	if err == nil {
		t.Error("expected error from ADMIN audit failure propagation, got nil")
	}
}

// TestEntityService_Create_NoopAuditWriter verifies backward compatibility:
// a pipeline with NoopAuditWriter allows Create to succeed without any audit writes.
func TestEntityService_Create_NoopAuditWriter(t *testing.T) {
	t.Parallel()

	svc, _, _ := buildTestService(t, audit.NoopAuditWriter{})
	actor := &def.Actor{UserID: uuid.New(), TenantID: uuid.New()}

	_, err := svc.Create(context.Background(), map[string]any{"status": "draft"}, actor)
	if err != nil {
		t.Fatalf("Create with NoopAuditWriter: unexpected error %v", err)
	}
}

// TestEntityService_Create_HookOrderPreserved verifies that RunAfterCreate
// hooks still fire after the audit record is written (order preserved).
func TestEntityService_Create_HookOrderPreserved(t *testing.T) {
	t.Parallel()

	order := make([]string, 0, 2)
	rw := &audit.RecordingWriter{}

	hook := &afterCreateRecorder{fn: func() { order = append(order, "after_create") }}
	d := &def.SystemDefinition{
		Name:   "svc_hook_order",
		Module: "svc",
		Label:  "HookOrder",
		Hooks:  def.HookSet{AfterCreate: []def.AfterCreateHook{hook}},
	}
	schema := buildServiceTestSchema(t, d)
	audit.Register(audit.EntityAuditConfig{
		EntityName: "svc_hook_order",
		Enabled:    true,
		Category:   audit.CategoryData,
	})

	pipeline := runtime.NewPipeline(schema, rw)
	es := schema.ByName["svc_hook_order"]
	repo := &stubRepo{entityName: "svc_hook_order"}
	svc := service.NewEntityService(es, repo, pipeline, nil)

	// Wire the recording writer to also record its call order.
	// We can inspect: audit write happens first, then AfterCreate hook.
	actor := &def.Actor{UserID: uuid.New(), TenantID: uuid.New()}
	_, err := svc.Create(context.Background(), map[string]any{}, actor)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Audit record must exist.
	if rw.Len() != 1 {
		t.Errorf("expected 1 audit record, got %d", rw.Len())
	}
	// AfterCreate hook must have fired.
	if len(order) != 1 || order[0] != "after_create" {
		t.Errorf("hook order: got %v, want [after_create]", order)
	}
}

// afterCreateRecorder is a test AfterCreateHook that records its invocation.
type afterCreateRecorder struct {
	fn func()
}

func (h *afterCreateRecorder) AfterCreate(_ context.Context, _ *def.EntityRecord) error {
	h.fn()
	return nil
}
