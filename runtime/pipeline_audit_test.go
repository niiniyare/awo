package runtime_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"awo.so/awo/audit"
	"awo.so/awo/def"
	"awo.so/awo/runtime"
)

// TestNewPipeline_NilAuditWriterPanics verifies that NewPipeline enforces the
// non-nil contract on auditWriter at construction time.
func TestNewPipeline_NilAuditWriterPanics(t *testing.T) {
	t.Parallel()

	schema := buildTestSchema(t, &def.SystemDefinition{
		Name: "panic_guard", Module: "test", Label: "Guard",
	})

	defer func() {
		if r := recover(); r == nil {
			t.Error("NewPipeline(schema, nil) must panic; got no panic")
		}
	}()
	//nolint:staticcheck — intentional nil to assert panic
	runtime.NewPipeline(schema, nil)
}

// TestNewPipeline_NoopAuditWriterAccepted verifies that NoopAuditWriter is a
// valid (non-panicking) audit writer.
func TestNewPipeline_NoopAuditWriterAccepted(t *testing.T) {
	t.Parallel()

	schema := buildTestSchema(t, &def.SystemDefinition{
		Name: "noop_guard", Module: "test", Label: "Guard",
	})
	// Must not panic.
	_ = runtime.NewPipeline(schema, audit.NoopAuditWriter{})
}

// TestRunAuditRecord_Noop verifies that RunAuditRecord with NoopAuditWriter
// always returns nil and does not affect the pipeline.
func TestRunAuditRecord_Noop(t *testing.T) {
	t.Parallel()

	schema := buildTestSchema(t, &def.SystemDefinition{
		Name: "noop_audit_entity", Module: "test", Label: "Audit",
	})
	pipeline := runtime.NewPipeline(schema, audit.NoopAuditWriter{})

	record := &def.EntityRecord{
		ID:         uuid.New(),
		TenantID:   uuid.New(),
		EntityName: "noop_audit_entity",
		Data:       map[string]any{"field": "value"},
		Meta:       def.RecordMeta{Actor: &def.Actor{UserID: uuid.New()}},
	}

	// Register entity audit config (Enabled=true by default).
	audit.Register(audit.EntityAuditConfig{
		EntityName: "noop_audit_entity",
		Enabled:    true,
		Category:   audit.CategoryData,
	})

	err := pipeline.RunAuditRecord(context.Background(), record, nil, record.Data)
	if err != nil {
		t.Errorf("RunAuditRecord with NoopAuditWriter: unexpected error %v", err)
	}
}

// TestRunAuditRecord_DisabledEntity verifies that RunAuditRecord skips the
// write when EntityAuditConfig.Enabled is false.
func TestRunAuditRecord_DisabledEntity(t *testing.T) {
	t.Parallel()

	const entityName = "disabled_audit_entity"
	schema := buildTestSchema(t, &def.SystemDefinition{
		Name: entityName, Module: "test", Label: "Disabled",
	})

	failing := &audit.FailingWriter{Err: errors.New("must not be called")}
	pipeline := runtime.NewPipeline(schema, failing)

	audit.Register(audit.EntityAuditConfig{
		EntityName: entityName,
		Enabled:    false,
		Category:   audit.CategoryData,
	})

	record := &def.EntityRecord{
		ID:         uuid.New(),
		TenantID:   uuid.New(),
		EntityName: entityName,
		Data:       map[string]any{},
		Meta:       def.RecordMeta{Actor: &def.Actor{UserID: uuid.New()}},
	}

	// FailingWriter.Write must not be called; if it is, an error is returned.
	err := pipeline.RunAuditRecord(context.Background(), record, nil, record.Data)
	if err != nil {
		t.Errorf("RunAuditRecord for disabled entity: unexpected error %v", err)
	}
}

// TestRunAuditRecord_RecordingWriter verifies that RunAuditRecord correctly
// delivers an AuditRecord to the underlying AuditWriter with the expected
// fields set.
func TestRunAuditRecord_RecordingWriter(t *testing.T) {
	t.Parallel()

	const entityName = "recording_audit_entity"
	schema := buildTestSchema(t, &def.SystemDefinition{
		Name: entityName, Module: "test", Label: "Recording",
	})

	rw := &audit.RecordingWriter{}
	pipeline := runtime.NewPipeline(schema, rw)

	audit.Register(audit.EntityAuditConfig{
		EntityName: entityName,
		Enabled:    true,
		Category:   audit.CategoryData,
	})

	tenantID := uuid.New()
	recordID := uuid.New()
	actor := &def.Actor{UserID: uuid.New()}

	record := &def.EntityRecord{
		ID:         recordID,
		TenantID:   tenantID,
		EntityName: entityName,
		Data:       map[string]any{"status": "active"},
		Meta:       def.RecordMeta{Actor: actor},
	}

	if err := pipeline.RunAuditRecord(context.Background(), record, nil, record.Data); err != nil {
		t.Fatalf("RunAuditRecord: unexpected error %v", err)
	}

	if rw.Len() != 1 {
		t.Fatalf("expected 1 audit record, got %d", rw.Len())
	}

	got := rw.Last()

	if got.TenantID != tenantID {
		t.Errorf("TenantID: got %v, want %v", got.TenantID, tenantID)
	}
	if got.RecordID != recordID {
		t.Errorf("RecordID: got %v, want %v", got.RecordID, recordID)
	}
	if got.EntityName != entityName {
		t.Errorf("EntityName: got %q, want %q", got.EntityName, entityName)
	}
	if got.Operation != audit.OperationCreate {
		t.Errorf("Operation: got %q, want %q", got.Operation, audit.OperationCreate)
	}
	if got.EventCategory != audit.CategoryData {
		t.Errorf("EventCategory: got %q, want %q", got.EventCategory, audit.CategoryData)
	}
	if got.Actor == nil || got.Actor.UserID != actor.UserID {
		t.Errorf("Actor: got %v, want UserID=%v", got.Actor, actor.UserID)
	}
	if got.BeforeData != nil {
		t.Errorf("BeforeData: got %v, want nil (Create operation)", got.BeforeData)
	}
}

// TestRunAuditRecord_OperationDerivation verifies that the operation type is
// derived correctly from the before/after snapshot arguments.
func TestRunAuditRecord_OperationDerivation(t *testing.T) {
	t.Parallel()

	const entityName = "op_derivation_entity"
	schema := buildTestSchema(t, &def.SystemDefinition{
		Name: entityName, Module: "test", Label: "OpDerivation",
	})
	audit.Register(audit.EntityAuditConfig{
		EntityName: entityName, Enabled: true, Category: audit.CategoryData,
	})

	base := &def.EntityRecord{
		ID:         uuid.New(),
		TenantID:   uuid.New(),
		EntityName: entityName,
		Data:       map[string]any{"v": 1},
		Meta:       def.RecordMeta{Actor: &def.Actor{UserID: uuid.New()}},
	}

	cases := []struct {
		name   string
		before map[string]any
		after  map[string]any
		wantOp audit.OperationType
	}{
		{"create", nil, map[string]any{"v": 1}, audit.OperationCreate},
		{"update", map[string]any{"v": 0}, map[string]any{"v": 1}, audit.OperationUpdate},
		{"delete", map[string]any{"v": 1}, nil, audit.OperationDelete},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rw := &audit.RecordingWriter{}
			pipeline := runtime.NewPipeline(schema, rw)

			if err := pipeline.RunAuditRecord(context.Background(), base, tc.before, tc.after); err != nil {
				t.Fatalf("RunAuditRecord: %v", err)
			}
			if rw.Len() == 0 {
				t.Fatal("expected 1 audit record, got 0")
			}
			if got := rw.Last().Operation; got != tc.wantOp {
				t.Errorf("Operation: got %q, want %q", got, tc.wantOp)
			}
		})
	}
}

// TestRunAuditRecord_PropagatesOnAdmin verifies that ADMIN category audit
// failures are propagated (not suppressed) — the mutation should roll back.
func TestRunAuditRecord_PropagatesOnAdmin(t *testing.T) {
	t.Parallel()

	const entityName = "admin_propagate_entity"
	schema := buildTestSchema(t, &def.SystemDefinition{
		Name: entityName, Module: "test", Label: "AdminProp",
	})
	audit.Register(audit.EntityAuditConfig{
		EntityName: entityName,
		Enabled:    true,
		Category:   audit.CategoryAdmin, // ADMIN → Propagate policy
	})

	sentinel := errors.New("audit storage unavailable")
	pipeline := runtime.NewPipeline(schema, &audit.FailingWriter{Err: sentinel})

	record := &def.EntityRecord{
		ID:         uuid.New(),
		TenantID:   uuid.New(),
		EntityName: entityName,
		Data:       map[string]any{},
		Meta:       def.RecordMeta{Actor: &def.Actor{UserID: uuid.New()}},
	}

	err := pipeline.RunAuditRecord(context.Background(), record, nil, record.Data)
	if err == nil {
		t.Error("ADMIN category failure must propagate; got nil error")
	}
}

// TestRunAuditRecord_SuppressesOnData verifies that DATA category audit
// failures are suppressed — the mutation should succeed.
func TestRunAuditRecord_SuppressesOnData(t *testing.T) {
	t.Parallel()

	const entityName = "data_suppress_entity"
	schema := buildTestSchema(t, &def.SystemDefinition{
		Name: entityName, Module: "test", Label: "DataSuppress",
	})
	audit.Register(audit.EntityAuditConfig{
		EntityName: entityName,
		Enabled:    true,
		Category:   audit.CategoryData, // DATA → SilentSuppress policy
	})

	pipeline := runtime.NewPipeline(schema, &audit.FailingWriter{Err: errors.New("transient DB error")})

	record := &def.EntityRecord{
		ID:         uuid.New(),
		TenantID:   uuid.New(),
		EntityName: entityName,
		Data:       map[string]any{},
		Meta:       def.RecordMeta{Actor: &def.Actor{UserID: uuid.New()}},
	}

	err := pipeline.RunAuditRecord(context.Background(), record, nil, record.Data)
	if err != nil {
		t.Errorf("DATA category failure must be suppressed; got error: %v", err)
	}
}
