package runtime

import (
	"context"
	"fmt"
	"testing"

	"awo.so/awo/audit"
	"awo.so/awo/def"
)

// TestPipeline_ApplyDefaults verifies that missing fields get default values.
func TestPipeline_ApplyDefaults(t *testing.T) {
	fields := []def.FieldDef{
		{Name: "status", Type: def.FieldTypeData, Default: func() any { return "draft" }},
		{Name: "name", Type: def.FieldTypeData, Required: true},
	}
	schema := buildPipelineSchema(fields, def.HookSet{})
	p := NewPipeline(schema, audit.NoopAuditWriter{})

	record, err := p.RunBeforeCreate(&CreateContext{
		Ctx:        context.Background(),
		EntityName: "test_widget",
		Data:       map[string]any{"name": "Acme"},
		Actor:      &def.Actor{},
	})
	if err != nil {
		t.Fatalf("RunBeforeCreate: %v", err)
	}
	if record.Data["status"] != "draft" {
		t.Errorf("expected default status=draft, got %v", record.Data["status"])
	}
}

// TestPipeline_ApplyDefaults_ExistingValueNotOverwritten verifies that
// a caller-provided value is not replaced by the default.
func TestPipeline_ApplyDefaults_ExistingValueNotOverwritten(t *testing.T) {
	fields := []def.FieldDef{
		{Name: "status", Type: def.FieldTypeData, Default: func() any { return "draft" }},
		{Name: "name", Type: def.FieldTypeData},
	}
	schema := buildPipelineSchema(fields, def.HookSet{})
	p := NewPipeline(schema, audit.NoopAuditWriter{})

	record, err := p.RunBeforeCreate(&CreateContext{
		Ctx:        context.Background(),
		EntityName: "test_widget",
		Data:       map[string]any{"status": "submitted", "name": "x"},
		Actor:      &def.Actor{},
	})
	if err != nil {
		t.Fatalf("RunBeforeCreate: %v", err)
	}
	if record.Data["status"] != "submitted" {
		t.Errorf("expected status=submitted (caller value), got %v", record.Data["status"])
	}
}

// TestPipeline_ValidateFields_WithValidator verifies custom FieldValidator runs.
func TestPipeline_ValidateFields_WithValidator(t *testing.T) {
	validator := func(_ context.Context, val any) string {
		s, ok := val.(string)
		if !ok || len(s) < 3 {
			return "must be at least 3 characters"
		}
		return ""
	}
	fields := []def.FieldDef{
		{Name: "code", Type: def.FieldTypeData, Validators: []def.FieldValidator{validator}},
	}
	schema := buildPipelineSchema(fields, def.HookSet{})
	p := NewPipeline(schema, audit.NoopAuditWriter{})

	_, err := p.RunBeforeCreate(&CreateContext{
		Ctx:        context.Background(),
		EntityName: "test_widget",
		Data:       map[string]any{"code": "ab"}, // too short
		Actor:      &def.Actor{},
	})
	if err == nil {
		t.Fatal("expected ValidationError from validator; got nil")
	}
	if !IsValidation(err) {
		t.Errorf("expected ValidationError; got %T: %v", err, err)
	}
}

// TestPipeline_ValidateFields_Passes_WhenValid verifies validator passes for valid input.
func TestPipeline_ValidateFields_Passes_WhenValid(t *testing.T) {
	validator := func(_ context.Context, val any) string {
		s, ok := val.(string)
		if !ok || len(s) < 3 {
			return "too short"
		}
		return ""
	}
	fields := []def.FieldDef{
		{Name: "code", Type: def.FieldTypeData, Validators: []def.FieldValidator{validator}},
	}
	schema := buildPipelineSchema(fields, def.HookSet{})
	p := NewPipeline(schema, audit.NoopAuditWriter{})

	_, err := p.RunBeforeCreate(&CreateContext{
		Ctx:        context.Background(),
		EntityName: "test_widget",
		Data:       map[string]any{"code": "ABC"},
		Actor:      &def.Actor{},
	})
	if err != nil {
		t.Fatalf("expected no error for valid code; got %v", err)
	}
}

// TestPipeline_NamingSeries_SkippedWhenNilTenantID verifies applyNamingSeries
// exits early when TenantID is zero.
func TestPipeline_NamingSeries_SkippedWhenNilService(t *testing.T) {
	fields := []def.FieldDef{
		{Name: "number", Type: def.FieldTypeNamingSeries, Series: "W-{SEQ:5}"},
	}
	schema := buildPipelineSchema(fields, def.HookSet{})
	// No naming service attached — applyNamingSeries guarded by p.naming != nil.
	p := NewPipeline(schema, audit.NoopAuditWriter{})

	record, err := p.RunBeforeCreate(&CreateContext{
		Ctx:        context.Background(),
		EntityName: "test_widget",
		Data:       map[string]any{"number": ""},
		Actor:      &def.Actor{},
	})
	// Should succeed without allocating: no naming service registered.
	if err != nil {
		t.Fatalf("unexpected error with no naming service: %v", err)
	}
	_ = record
}

// TestPipeline_UnknownEntity_ReturnsError verifies lookupSchema returns error.
func TestPipeline_UnknownEntity_RunBeforeCreate_Error(t *testing.T) {
	schema := buildPipelineSchema(nil, def.HookSet{})
	p := NewPipeline(schema, audit.NoopAuditWriter{})

	_, err := p.RunBeforeCreate(&CreateContext{
		Ctx:        context.Background(),
		EntityName: "nonexistent_entity",
		Data:       map[string]any{},
		Actor:      &def.Actor{},
	})
	if err == nil {
		t.Fatal("expected error for unknown entity; got nil")
	}
}

// TestPipeline_RunAfterCreate_UnknownEntity verifies error propagation.
func TestPipeline_RunAfterCreate_UnknownEntity_Error(t *testing.T) {
	schema := buildPipelineSchema(nil, def.HookSet{})
	p := NewPipeline(schema, audit.NoopAuditWriter{})

	record := &def.EntityRecord{EntityName: "nonexistent"}
	if err := p.RunAfterCreate(context.Background(), record); err == nil {
		t.Fatal("expected error for unknown entity; got nil")
	}
}

// TestPipeline_RunBeforeUpdate_UnknownEntity_Error tests lookupSchema via Update path.
func TestPipeline_RunBeforeUpdate_UnknownEntity_Error(t *testing.T) {
	schema := buildPipelineSchema(nil, def.HookSet{})
	p := NewPipeline(schema, audit.NoopAuditWriter{})

	current := &def.EntityRecord{EntityName: "nonexistent"}
	_, err := p.RunBeforeUpdate(&UpdateContext{
		Ctx: context.Background(), EntityName: "nonexistent",
		Data: map[string]any{}, Actor: &def.Actor{},
	}, current)
	if err == nil {
		t.Fatal("expected error for unknown entity; got nil")
	}
}

// TestPipeline_RunAfterUpdate_UnknownEntity_Error tests After path.
func TestPipeline_RunAfterUpdate_UnknownEntity_Error(t *testing.T) {
	schema := buildPipelineSchema(nil, def.HookSet{})
	p := NewPipeline(schema, audit.NoopAuditWriter{})

	record := &def.EntityRecord{EntityName: "nonexistent"}
	if err := p.RunAfterUpdate(context.Background(), record, record); err == nil {
		t.Fatal("expected error for unknown entity; got nil")
	}
}

// TestPipeline_RunBeforeDelete_UnknownEntity_Error tests Delete path.
func TestPipeline_RunBeforeDelete_UnknownEntity_Error(t *testing.T) {
	schema := buildPipelineSchema(nil, def.HookSet{})
	p := NewPipeline(schema, audit.NoopAuditWriter{})

	record := &def.EntityRecord{EntityName: "nonexistent"}
	if err := p.RunBeforeDelete(context.Background(), record); err == nil {
		t.Fatal("expected error for unknown entity; got nil")
	}
}

// TestPipeline_RunAfterDelete_UnknownEntity_Error tests After Delete path.
func TestPipeline_RunAfterDelete_UnknownEntity_Error(t *testing.T) {
	schema := buildPipelineSchema(nil, def.HookSet{})
	p := NewPipeline(schema, audit.NoopAuditWriter{})

	record := &def.EntityRecord{EntityName: "nonexistent"}
	if err := p.RunAfterDelete(context.Background(), record); err == nil {
		t.Fatal("expected error for unknown entity; got nil")
	}
}

// TestPipeline_NewPipeline_NilAuditWriter_Panics verifies nil auditWriter panics.
func TestPipeline_NewPipeline_NilAuditWriter_Panics(t *testing.T) {
	schema := buildPipelineSchema(nil, def.HookSet{})
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewPipeline with nil auditWriter should panic")
		}
	}()
	NewPipeline(schema, nil)
}

// TestAuditOperationFor covers auditOperationFor branches.
func TestAuditOperationFor(t *testing.T) {
	// before=nil → Create
	op := auditOperationFor(nil, map[string]any{"k": "v"})
	if op != "create" {
		t.Errorf("expected create, got %q", op)
	}
	// after=nil → Delete
	op = auditOperationFor(map[string]any{"k": "v"}, nil)
	if op != "delete" {
		t.Errorf("expected delete, got %q", op)
	}
	// both set → Update
	op = auditOperationFor(map[string]any{"k": "old"}, map[string]any{"k": "new"})
	if op != "update" {
		t.Errorf("expected update, got %q", op)
	}
}

// TestCloneMap covers the nil and non-nil paths of cloneMap.
func TestCloneMap(t *testing.T) {
	if got := cloneMap(nil); got != nil {
		t.Errorf("cloneMap(nil) should return nil, got %v", got)
	}
	src := map[string]any{"a": 1, "b": "two"}
	got := cloneMap(src)
	if len(got) != 2 {
		t.Errorf("cloneMap: expected len 2, got %d", len(got))
	}
	// Verify independence.
	got["a"] = 99
	if src["a"] == 99 {
		t.Error("cloneMap should produce an independent copy")
	}
}

// TestMergeMaps verifies patch values override base.
func TestMergeMaps(t *testing.T) {
	base := map[string]any{"a": 1, "b": 2}
	patch := map[string]any{"b": 20, "c": 30}
	got := mergeMaps(base, patch)
	if got["a"] != 1 {
		t.Errorf("expected a=1, got %v", got["a"])
	}
	if got["b"] != 20 {
		t.Errorf("expected b=20 (patch wins), got %v", got["b"])
	}
	if got["c"] != 30 {
		t.Errorf("expected c=30, got %v", got["c"])
	}
	// originals unchanged
	if base["b"] != 2 {
		t.Error("mergeMaps must not mutate base")
	}
}

// TestPipeline_BeforeSave_HookError covers before_save error path.
func TestPipeline_BeforeSave_HookError(t *testing.T) {
	errHook := &errorHookOnSave{}
	hooks := def.HookSet{
		BeforeSave: []def.BeforeSaveHook{errHook},
	}
	schema := buildPipelineSchema([]def.FieldDef{{Name: "name", Type: def.FieldTypeData}}, hooks)
	p := NewPipeline(schema, audit.NoopAuditWriter{})

	_, err := p.RunBeforeCreate(&CreateContext{
		Ctx: context.Background(), EntityName: "test_widget",
		Data: map[string]any{"name": "x"}, Actor: &def.Actor{},
	})
	if err == nil {
		t.Fatal("expected error from BeforeSave hook; got nil")
	}
}

type errorHookOnSave struct{}

func (h *errorHookOnSave) BeforeSave(_ context.Context, _ *def.EntityRecord) error {
	return fmt.Errorf("before_save: simulated error")
}
