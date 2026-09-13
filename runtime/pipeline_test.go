package runtime

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"awo.so/awo/audit"
	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/registry"
)

// --- Helpers ---

func mustCompile(defs ...def.EntityDefinition) *compiler.CompiledSchema {
	reg, err := registry.BuildFrom(defs)
	if err != nil {
		panic(fmt.Sprintf("mustCompile: BuildFrom: %v", err))
	}
	schema, err := compiler.Compile(reg)
	if err != nil {
		panic(fmt.Sprintf("mustCompile: Compile: %v", err))
	}
	return schema
}

func buildPipelineSchema(fields []def.FieldDef, hooks def.HookSet) *compiler.CompiledSchema {
	return mustCompile(&def.SystemDefinition{
		Name: "widget", Module: "test", Fields: fields, Hooks: hooks,
	})
}

// --- Hook tracking ---

type trackingHook struct {
	label string
	log   *[]string
}

func (h *trackingHook) BeforeValidate(_ context.Context, _ *def.EntityRecord) error {
	*h.log = append(*h.log, h.label); return nil
}
func (h *trackingHook) BeforeCreate(_ context.Context, _ *def.EntityRecord) error {
	*h.log = append(*h.log, h.label); return nil
}
func (h *trackingHook) AfterCreate(_ context.Context, _ *def.EntityRecord) error {
	*h.log = append(*h.log, h.label); return nil
}
func (h *trackingHook) BeforeUpdate(_ context.Context, _, _ *def.EntityRecord) error {
	*h.log = append(*h.log, h.label); return nil
}
func (h *trackingHook) AfterUpdate(_ context.Context, _, _ *def.EntityRecord) error {
	*h.log = append(*h.log, h.label); return nil
}
func (h *trackingHook) BeforeDelete(_ context.Context, _ *def.EntityRecord) error {
	*h.log = append(*h.log, h.label); return nil
}
func (h *trackingHook) AfterDelete(_ context.Context, _ *def.EntityRecord) error {
	*h.log = append(*h.log, h.label); return nil
}
func (h *trackingHook) BeforeSave(_ context.Context, _ *def.EntityRecord) error {
	*h.log = append(*h.log, h.label); return nil
}
func (h *trackingHook) AfterSave(_ context.Context, _ *def.EntityRecord) error {
	*h.log = append(*h.log, h.label); return nil
}

func th(label string, log *[]string) *trackingHook { return &trackingHook{label: label, log: log} }

type failHook struct{ log *[]string }

func (h *failHook) BeforeValidate(_ context.Context, _ *def.EntityRecord) error {
	*h.log = append(*h.log, "fail")
	return &ValidationError{Fields: map[string]string{"_": "injected"}}
}

type auditCapture struct{ called bool }

func (a *auditCapture) Write(_ context.Context, _ audit.AuditRecord) error {
	a.called = true; return nil
}

func assertOrder(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("hook count: got %d %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("hook[%d]: got %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

// --- Tests ---

func TestPipeline_CreateHookOrder(t *testing.T) {
	var log []string
	hooks := def.HookSet{
		BeforeValidate: []def.BeforeValidateHook{th("before_validate", &log)},
		BeforeSave:     []def.BeforeSaveHook{th("before_save", &log)},
		BeforeCreate:   []def.BeforeCreateHook{th("before_create", &log)},
		AfterCreate:    []def.AfterCreateHook{th("after_create", &log)},
		AfterSave:      []def.AfterSaveHook{th("after_save", &log)},
	}
	schema := buildPipelineSchema([]def.FieldDef{{Name: "name", Type: def.FieldTypeData}}, hooks)
	p := NewPipeline(schema, audit.NoopAuditWriter{})

	ctx := context.Background()
	record, err := p.RunBeforeCreate(&CreateContext{
		Ctx: ctx, EntityName: "test_widget",
		Data: map[string]any{"name": "x"}, Actor: &def.Actor{},
	})
	if err != nil {
		t.Fatalf("RunBeforeCreate: %v", err)
	}
	if err := p.RunAfterCreate(ctx, record); err != nil {
		t.Fatalf("RunAfterCreate: %v", err)
	}
	assertOrder(t, log, []string{
		"before_validate", "before_save", "before_create", "after_create", "after_save",
	})
}

func TestPipeline_UpdateHookOrder(t *testing.T) {
	var log []string
	hooks := def.HookSet{
		BeforeValidate: []def.BeforeValidateHook{th("before_validate", &log)},
		BeforeSave:     []def.BeforeSaveHook{th("before_save", &log)},
		BeforeUpdate:   []def.BeforeUpdateHook{th("before_update", &log)},
		AfterUpdate:    []def.AfterUpdateHook{th("after_update", &log)},
		AfterSave:      []def.AfterSaveHook{th("after_save", &log)},
	}
	schema := buildPipelineSchema([]def.FieldDef{{Name: "name", Type: def.FieldTypeData}}, hooks)
	p := NewPipeline(schema, audit.NoopAuditWriter{})

	ctx := context.Background()
	current := &def.EntityRecord{EntityName: "test_widget", Data: map[string]any{"name": "old"}}
	proposed, err := p.RunBeforeUpdate(&UpdateContext{
		Ctx: ctx, EntityName: "test_widget",
		Data: map[string]any{"name": "new"}, Actor: &def.Actor{},
	}, current)
	if err != nil {
		t.Fatalf("RunBeforeUpdate: %v", err)
	}
	if err := p.RunAfterUpdate(ctx, proposed, current); err != nil {
		t.Fatalf("RunAfterUpdate: %v", err)
	}
	assertOrder(t, log, []string{
		"before_validate", "before_save", "before_update", "after_update", "after_save",
	})
}

func TestPipeline_DeleteHookOrder(t *testing.T) {
	var log []string
	hooks := def.HookSet{
		BeforeDelete: []def.BeforeDeleteHook{th("before_delete", &log)},
		AfterDelete:  []def.AfterDeleteHook{th("after_delete", &log)},
	}
	schema := buildPipelineSchema([]def.FieldDef{{Name: "name", Type: def.FieldTypeData}}, hooks)
	p := NewPipeline(schema, audit.NoopAuditWriter{})

	ctx := context.Background()
	record := &def.EntityRecord{EntityName: "test_widget"}
	if err := p.RunBeforeDelete(ctx, record); err != nil {
		t.Fatalf("RunBeforeDelete: %v", err)
	}
	if err := p.RunAfterDelete(ctx, record); err != nil {
		t.Fatalf("RunAfterDelete: %v", err)
	}
	assertOrder(t, log, []string{"before_delete", "after_delete"})
}

func TestPipeline_ImmutableField_Rejected(t *testing.T) {
	fields := []def.FieldDef{
		{Name: "name", Type: def.FieldTypeData},
		{Name: "code", Type: def.FieldTypeData, Immutable: true},
	}
	schema := buildPipelineSchema(fields, def.HookSet{})
	p := NewPipeline(schema, audit.NoopAuditWriter{})

	current := &def.EntityRecord{EntityName: "test_widget", Data: map[string]any{"name": "foo", "code": "W-001"}}
	_, err := p.RunBeforeUpdate(&UpdateContext{
		Ctx: context.Background(), EntityName: "test_widget",
		Data: map[string]any{"code": "W-002"}, Actor: &def.Actor{},
	}, current)
	if err == nil {
		t.Fatal("expected error when updating immutable field; got nil")
	}
	if !IsValidation(err) {
		t.Errorf("expected ValidationError; got %T: %v", err, err)
	}
}

func TestPipeline_RequiredField_Missing(t *testing.T) {
	fields := []def.FieldDef{{Name: "name", Type: def.FieldTypeData, Required: true}}
	schema := buildPipelineSchema(fields, def.HookSet{})
	p := NewPipeline(schema, audit.NoopAuditWriter{})

	_, err := p.RunBeforeCreate(&CreateContext{
		Ctx: context.Background(), EntityName: "test_widget",
		Data: map[string]any{}, Actor: &def.Actor{},
	})
	if err == nil {
		t.Fatal("expected ValidationError for missing required field; got nil")
	}
	if !IsValidation(err) {
		t.Errorf("expected ValidationError; got %T: %v", err, err)
	}
}

func TestPipeline_HookError_ShortCircuits(t *testing.T) {
	var log []string
	hooks := def.HookSet{
		BeforeValidate: []def.BeforeValidateHook{&failHook{log: &log}},
		BeforeCreate:   []def.BeforeCreateHook{th("should_not_fire", &log)},
	}
	schema := buildPipelineSchema([]def.FieldDef{{Name: "name", Type: def.FieldTypeData}}, hooks)
	p := NewPipeline(schema, audit.NoopAuditWriter{})

	_, err := p.RunBeforeCreate(&CreateContext{
		Ctx: context.Background(), EntityName: "test_widget",
		Data: map[string]any{"name": "x"}, Actor: &def.Actor{},
	})
	if err == nil {
		t.Fatal("expected error from failing hook; got nil")
	}
	for _, entry := range log {
		if entry == "should_not_fire" {
			t.Error("hook after failing hook must not be invoked")
		}
	}
}

func TestPipeline_AllowAudit_False_SkipsWrite(t *testing.T) {
	cap := &auditCapture{}
	schema := mustCompile(&def.SystemDefinition{
		Name: "widget", Module: "test",
		Fields:       []def.FieldDef{{Name: "name", Type: def.FieldTypeData}},
		DisableAudit: true,
	})
	p := NewPipeline(schema, cap)

	record := &def.EntityRecord{EntityName: "test_widget", Data: map[string]any{"name": "x"}}
	if err := p.RunAuditRecord(context.Background(), record, nil, record.Data); err != nil {
		t.Fatalf("RunAuditRecord: %v", err)
	}
	if cap.called {
		t.Error("AuditWriter.Write must NOT be called when DisableAudit=true")
	}
}

func TestPipeline_AllowAudit_True_WritesAudit(t *testing.T) {
	cap := &auditCapture{}
	schema := mustCompile(&def.SystemDefinition{
		Name: "widget", Module: "test",
		Fields: []def.FieldDef{{Name: "name", Type: def.FieldTypeData}},
	})
	p := NewPipeline(schema, cap)

	record := &def.EntityRecord{EntityName: "test_widget", Data: map[string]any{"name": "x"}}
	if err := p.RunAuditRecord(context.Background(), record, nil, record.Data); err != nil {
		t.Fatalf("RunAuditRecord: %v", err)
	}
	if !cap.called {
		t.Error("AuditWriter.Write MUST be called when AllowAudit=true")
	}
}

// --- Hook panic recovery tests ---

// panicHook implements BeforeValidateHook and panics unconditionally.
type panicHook struct{}

func (h *panicHook) BeforeValidate(_ context.Context, _ *def.EntityRecord) error {
	panic("deliberate test panic")
}

func TestPipeline_HookPanic_ReturnsError(t *testing.T) {
	hooks := def.HookSet{
		BeforeValidate: []def.BeforeValidateHook{&panicHook{}},
	}
	schema := buildPipelineSchema([]def.FieldDef{{Name: "name", Type: def.FieldTypeData}}, hooks)
	p := NewPipeline(schema, audit.NoopAuditWriter{})

	_, err := p.RunBeforeCreate(&CreateContext{
		Ctx: context.Background(), EntityName: "test_widget",
		Data: map[string]any{"name": "x"}, Actor: &def.Actor{},
	})
	if err == nil {
		t.Fatal("expected error from panicking hook; got nil (panic must not crash server)")
	}
	if !IsHookPanic(err) {
		t.Errorf("expected HookPanicError in error chain; got %T: %v", err, err)
	}
}

func TestPipeline_HookPanic_ContainsStageAndMessage(t *testing.T) {
	hooks := def.HookSet{
		BeforeValidate: []def.BeforeValidateHook{&panicHook{}},
	}
	schema := buildPipelineSchema([]def.FieldDef{{Name: "name", Type: def.FieldTypeData}}, hooks)
	p := NewPipeline(schema, audit.NoopAuditWriter{})

	_, err := p.RunBeforeCreate(&CreateContext{
		Ctx: context.Background(), EntityName: "test_widget",
		Data: map[string]any{"name": "x"}, Actor: &def.Actor{},
	})

	var hpe *HookPanicError
	if !errors.As(err, &hpe) {
		t.Fatalf("expected *HookPanicError; got %T: %v", err, err)
	}
	if hpe.Stage == "" {
		t.Error("HookPanicError.Stage must not be empty")
	}
	if hpe.Panic == nil {
		t.Error("HookPanicError.Panic must carry the recovered value")
	}
}

func TestPipeline_HookPanic_DoesNotFireSubsequentHooks(t *testing.T) {
	var log []string
	hooks := def.HookSet{
		BeforeValidate: []def.BeforeValidateHook{&panicHook{}},
		BeforeCreate:   []def.BeforeCreateHook{th("must_not_fire", &log)},
	}
	schema := buildPipelineSchema([]def.FieldDef{{Name: "name", Type: def.FieldTypeData}}, hooks)
	p := NewPipeline(schema, audit.NoopAuditWriter{})

	_, _ = p.RunBeforeCreate(&CreateContext{
		Ctx: context.Background(), EntityName: "test_widget",
		Data: map[string]any{"name": "x"}, Actor: &def.Actor{},
	})
	for _, entry := range log {
		if entry == "must_not_fire" {
			t.Error("hook after panicking hook must not be invoked")
		}
	}
}
