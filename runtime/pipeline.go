package runtime

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"awo.so/awo/audit"
	"awo.so/awo/compiler"
	"awo.so/awo/def"
	"awo.so/awo/naming"
)

// Pipeline executes the entity lifecycle hook sequence for a mutation
// operation. It is the authoritative implementation of:
//
//	ASSEMBLE → BeforeValidate → VALIDATE → AUTHORIZE
//	         → BeforeSave / BeforeCreate / BeforeUpdate
//	         → [TX begins]  (delegated to the driver)
//	         → PERSIST      (delegated to the driver)
//	         → AUDIT RECORD (AuditWriter.Write inside same TX)
//	         → AfterSave / AfterCreate / AfterUpdate
//	         → [TX commits] (delegated to the driver)
//	         → Workflow start (outside TX)
//
// The Pipeline does not open database transactions directly. It calls the
// driver's Create/Update/Delete, which manage transactions internally and
// call the audit and after_* hook stages from within the transaction.
type Pipeline struct {
	reg         *RuntimeRegistry
	naming      *naming.NamingSeriesService
	auditWriter audit.AuditWriter
}

// NewPipeline creates a Pipeline bound to the compiled schema via RuntimeRegistry.
//
// auditWriter must not be nil. Callers that do not require audit writes must
// pass [audit.NoopAuditWriter]{}. Passing nil causes a panic at construction
// time — this is intentional to prevent silent audit gaps from misconfigured
// deployments.
func NewPipeline(schema *compiler.CompiledSchema, auditWriter audit.AuditWriter) *Pipeline {
	if auditWriter == nil {
		panic("runtime: NewPipeline: auditWriter must not be nil; pass audit.NoopAuditWriter{} to disable auditing")
	}
	return &Pipeline{reg: NewRuntimeRegistry(schema), auditWriter: auditWriter}
}

// WithNamingService attaches a NamingSeriesService to the Pipeline. When set,
// RunBeforeCreate automatically allocates naming-series values for every
// FieldTypeNamingSeries field that has no value already provided.
func (p *Pipeline) WithNamingService(svc *naming.NamingSeriesService) *Pipeline {
	p.naming = svc
	return p
}

// CreateContext carries all inputs for a Create pipeline run.
type CreateContext struct {
	// Ctx is the request context, carrying TenantContext.
	Ctx context.Context

	// EntityName identifies the entity definition to use.
	EntityName string

	// Data is the raw field values from the API request body.
	Data map[string]any

	// CustomFields is the custom field values from the API request body.
	CustomFields map[string]any

	// Actor is the authenticated principal.
	Actor *def.Actor
}

// UpdateContext carries all inputs for an Update pipeline run.
type UpdateContext struct {
	Ctx          context.Context
	EntityName   string
	Data         map[string]any
	CustomFields map[string]any
	Actor        *def.Actor
}

// DeleteContext carries all inputs for a Delete pipeline run.
type DeleteContext struct {
	Ctx        context.Context
	EntityName string
	Actor      *def.Actor
}

// RunBeforeCreate executes the pre-persist stages of the Create pipeline.
// The caller (driver) invokes this before opening the database transaction.
//
// Stages executed:
//  1. ApplyDefaults — populate missing fields from FieldDef.Default
//  2. BeforeValidate hooks
//  3. VALIDATE — field-level and FieldValidator checks
//  4. ImmutableField check (no-op on Create)
//  5. BeforeSave hooks
//  6. BeforeCreate hooks
//
// Returns the assembled EntityRecord. The driver then persists it and calls
// [RunAfterCreate] from within the database transaction.
func (p *Pipeline) RunBeforeCreate(pctx *CreateContext) (*def.EntityRecord, error) {
	es, err := p.lookupSchema(pctx.EntityName)
	if err != nil {
		return nil, err
	}

	record := &def.EntityRecord{
		EntityName:   pctx.EntityName,
		Data:         cloneMap(pctx.Data),
		CustomFields: cloneMap(pctx.CustomFields),
		Meta: def.RecordMeta{
			Actor:         pctx.Actor,
			OperationType: def.OperationCreate,
		},
	}

	// Stage 1: apply defaults for missing fields.
	p.applyDefaults(record, es)

	// Stage 1b: allocate naming-series values for unset NamingSeries fields.
	if p.naming != nil {
		if err := p.applyNamingSeries(pctx.Ctx, record, es); err != nil {
			return nil, err
		}
	}

	hooks := es.Hooks

	// Stage 2: BeforeValidate
	for _, h := range hooks.BeforeValidate {
		if err := safeCall("before_validate", fmt.Sprintf("%T", h), func() error {
			return h.BeforeValidate(pctx.Ctx, record)
		}); err != nil {
			return nil, fmt.Errorf("before_validate: %w", err)
		}
	}

	// Stage 3: VALIDATE
	if err := p.validateFields(pctx.Ctx, record, es); err != nil {
		return nil, err
	}

	// Stage 5: BeforeSave
	for _, h := range hooks.BeforeSave {
		if err := safeCall("before_save", fmt.Sprintf("%T", h), func() error {
			return h.BeforeSave(pctx.Ctx, record)
		}); err != nil {
			return nil, fmt.Errorf("before_save: %w", err)
		}
	}

	// Stage 6: BeforeCreate
	for _, h := range hooks.BeforeCreate {
		if err := safeCall("before_create", fmt.Sprintf("%T", h), func() error {
			return h.BeforeCreate(pctx.Ctx, record)
		}); err != nil {
			return nil, fmt.Errorf("before_create: %w", err)
		}
	}

	return record, nil
}

// RunAfterCreate executes the post-persist stages of the Create pipeline.
// Called by the driver FROM WITHIN the open database transaction.
//
// If any hook returns an error, the driver must roll back the transaction.
func (p *Pipeline) RunAfterCreate(ctx context.Context, record *def.EntityRecord) error {
	es, err := p.lookupSchema(record.EntityName)
	if err != nil {
		return err
	}
	hooks := es.Hooks

	// Entity-specific hook fires before the cross-cutting AfterSave.
	for _, h := range hooks.AfterCreate {
		if err := safeCall("after_create", fmt.Sprintf("%T", h), func() error {
			return h.AfterCreate(ctx, record)
		}); err != nil {
			return fmt.Errorf("after_create: %w", err)
		}
	}
	for _, h := range hooks.AfterSave {
		if err := safeCall("after_save", fmt.Sprintf("%T", h), func() error {
			return h.AfterSave(ctx, record)
		}); err != nil {
			return fmt.Errorf("after_save: %w", err)
		}
	}
	return nil
}

// RunBeforeUpdate executes the pre-persist stages of the Update pipeline.
// prev is the current record state before the update is applied.
func (p *Pipeline) RunBeforeUpdate(pctx *UpdateContext, current *def.EntityRecord) (*def.EntityRecord, error) {
	es, err := p.lookupSchema(pctx.EntityName)
	if err != nil {
		return nil, err
	}

	// Build the proposed record: current values + patch.
	proposed := &def.EntityRecord{
		ID:           current.ID,
		TenantID:     current.TenantID,
		EntityName:   current.EntityName,
		Data:         mergeMaps(current.Data, pctx.Data),
		CustomFields: mergeMaps(current.CustomFields, pctx.CustomFields),
		CreatedAt:    current.CreatedAt,
		Meta: def.RecordMeta{
			Actor:         pctx.Actor,
			OperationType: def.OperationUpdate,
		},
	}

	// Check immutable fields.
	ve := &ValidationError{}
	for field := range pctx.Data {
		if es.ImmutableFields[field] {
			ve.AddField(field, fmt.Sprintf("%s cannot be changed after creation", field))
		}
	}
	if !ve.IsEmpty() {
		return nil, ve
	}

	hooks := es.Hooks

	for _, h := range hooks.BeforeValidate {
		if err := safeCall("before_validate", fmt.Sprintf("%T", h), func() error {
			return h.BeforeValidate(pctx.Ctx, proposed)
		}); err != nil {
			return nil, fmt.Errorf("before_validate: %w", err)
		}
	}

	if err := p.validateFields(pctx.Ctx, proposed, es); err != nil {
		return nil, err
	}

	for _, h := range hooks.BeforeSave {
		if err := safeCall("before_save", fmt.Sprintf("%T", h), func() error {
			return h.BeforeSave(pctx.Ctx, proposed)
		}); err != nil {
			return nil, fmt.Errorf("before_save: %w", err)
		}
	}

	for _, h := range hooks.BeforeUpdate {
		if err := safeCall("before_update", fmt.Sprintf("%T", h), func() error {
			return h.BeforeUpdate(pctx.Ctx, proposed, current)
		}); err != nil {
			return nil, fmt.Errorf("before_update: %w", err)
		}
	}

	return proposed, nil
}

// RunAfterUpdate executes the post-persist stages of the Update pipeline.
// Called from within the open database transaction.
func (p *Pipeline) RunAfterUpdate(ctx context.Context, record, prev *def.EntityRecord) error {
	es, err := p.lookupSchema(record.EntityName)
	if err != nil {
		return err
	}
	hooks := es.Hooks

	// Entity-specific hook fires before the cross-cutting AfterSave.
	for _, h := range hooks.AfterUpdate {
		if err := safeCall("after_update", fmt.Sprintf("%T", h), func() error {
			return h.AfterUpdate(ctx, record, prev)
		}); err != nil {
			return fmt.Errorf("after_update: %w", err)
		}
	}
	for _, h := range hooks.AfterSave {
		if err := safeCall("after_save", fmt.Sprintf("%T", h), func() error {
			return h.AfterSave(ctx, record)
		}); err != nil {
			return fmt.Errorf("after_save: %w", err)
		}
	}
	return nil
}

// RunBeforeDelete executes the pre-delete stages.
func (p *Pipeline) RunBeforeDelete(ctx context.Context, record *def.EntityRecord) error {
	es, err := p.lookupSchema(record.EntityName)
	if err != nil {
		return err
	}
	hooks := es.Hooks
	for _, h := range hooks.BeforeDelete {
		if err := safeCall("before_delete", fmt.Sprintf("%T", h), func() error {
			return h.BeforeDelete(ctx, record)
		}); err != nil {
			return fmt.Errorf("before_delete: %w", err)
		}
	}
	return nil
}

// RunAfterDelete executes the post-delete stages from within the transaction.
func (p *Pipeline) RunAfterDelete(ctx context.Context, record *def.EntityRecord) error {
	es, err := p.lookupSchema(record.EntityName)
	if err != nil {
		return err
	}
	hooks := es.Hooks
	for _, h := range hooks.AfterDelete {
		if err := safeCall("after_delete", fmt.Sprintf("%T", h), func() error {
			return h.AfterDelete(ctx, record)
		}); err != nil {
			return fmt.Errorf("after_delete: %w", err)
		}
	}
	return nil
}

// RunAuditRecord writes an audit record for the given entity mutation within
// the caller's active database transaction (the txCtx carries the transaction
// set by repo.WithTx). It is called by EntityService between PERSIST and the
// after_save hook stage, always inside the transaction.
//
// before holds the field snapshot before the mutation (nil on Create).
// after holds the field snapshot after the mutation (nil on Delete).
//
// If the entity's audit configuration has Enabled == false, RunAuditRecord
// returns nil immediately without calling the AuditWriter.
//
// Failure policy is applied by [audit.Apply]: ADMIN and SECURITY categories
// propagate the error (causing the transaction to roll back); all other
// categories suppress the error (mutation succeeds, failure is logged).
func (p *Pipeline) RunAuditRecord(ctx context.Context, record *def.EntityRecord, before, after map[string]any) error {
	// Check EntitySchema.AllowAudit first (ADR-023). This is the compile-time
	// opt-out set via DisableAudit: true on the definition struct. It takes
	// precedence over the audit.EntityAuditConfig registry.
	if es, err := p.lookupSchema(record.EntityName); err == nil && !es.AllowAudit {
		return nil
	}

	cfg := audit.ConfigFor(record.EntityName)
	if !cfg.Enabled {
		return nil
	}

	rec := audit.AuditRecord{
		TenantID:      record.TenantID,
		EntityName:    record.EntityName,
		RecordID:      record.ID,
		Operation:     auditOperationFor(before, after),
		EventCategory: cfg.Category,
		Actor:         record.Meta.Actor,
		BeforeData:    before,
		AfterData:     after,
	}

	// Populate HTTP correlation fields from the audit.RequestContext injected
	// by RequireAuth. Background operations (Temporal, cron) do not carry a
	// RequestContext; those fields are left as zero values.
	if rc, ok := audit.RequestContextFromContext(ctx); ok {
		rec.RequestID = rc.RequestID
		rec.IPAddress = rc.IPAddress
		rec.SessionID = rc.SessionID
	}

	return audit.Apply(ctx, p.auditWriter, rec)
}

// auditOperationFor derives the audit OperationType from the presence of the
// before/after snapshots. Nil before → Create; nil after → Delete; both set → Update.
func auditOperationFor(before, after map[string]any) audit.OperationType {
	if before == nil {
		return audit.OperationCreate
	}
	if after == nil {
		return audit.OperationDelete
	}
	return audit.OperationUpdate
}

// --- Internal helpers ---

func (p *Pipeline) lookupSchema(entityName string) (*compiler.EntitySchema, error) {
	return p.reg.FindEntity(entityName)
}

func (p *Pipeline) applyDefaults(record *def.EntityRecord, es *compiler.EntitySchema) {
	for name, defaultFn := range es.DefaultValues {
		if _, exists := record.Data[name]; !exists {
			if record.Data == nil {
				record.Data = make(map[string]any)
			}
			record.Data[name] = defaultFn()
		}
	}
}

func (p *Pipeline) validateFields(ctx context.Context, record *def.EntityRecord, es *compiler.EntitySchema) error {
	ve := &ValidationError{}

	for name := range es.RequiredFields {
		val := record.Get(name)
		if val == nil || val == "" {
			ve.AddField(name, "this field is required")
		}
	}

	for fieldName, validators := range es.FieldValidators {
		val := record.Get(fieldName)
		for _, validator := range validators {
			if msg := validator(ctx, val); msg != "" {
				ve.AddField(fieldName, msg)
			}
		}
	}

	if !ve.IsEmpty() {
		return ve
	}
	return nil
}

func (p *Pipeline) applyNamingSeries(ctx context.Context, record *def.EntityRecord, es *compiler.EntitySchema) error {
	tenantID := record.TenantID
	if tenantID == uuid.Nil {
		// Extract from context if not set on record yet (middleware sets it later).
		// Skip silently — the driver will set tenant_id before persist.
		return nil
	}
	for _, f := range es.Fields {
		if f.Type != def.FieldTypeNamingSeries {
			continue
		}
		// Allow manual override: if a value is already set, keep it.
		if v := record.GetString(f.Name); v != "" {
			continue
		}
		if f.Series == "" {
			continue
		}
		orgCode := record.GetString("organization_code")
		if orgCode == "" {
			orgCode = record.GetString("org_code")
		}
		id, err := p.naming.AllocateForRecord(ctx, naming.NamingFieldContext{
			FieldName: f.Name,
			Pattern:   f.Series,
			TenantID:  tenantID,
			OrgCode:   orgCode,
		})
		if err != nil {
			return fmt.Errorf("naming_series %q: %w", f.Name, err)
		}
		record.Set(f.Name, id)
	}
	return nil
}

// safeCall invokes fn and recovers any panic, converting it to a
// *HookPanicError so the pipeline can propagate a clean error instead of
// crashing the server.
//
// Parameters:
//   - stage: hook stage name for diagnostic messages (e.g. "before_create")
//   - hookName: human-readable hook type name (fmt.Sprintf("%T", impl))
//   - fn: the hook call to execute
func safeCall(stage, hookName string, fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &HookPanicError{Stage: stage, HookName: hookName, Panic: r}
		}
	}()
	return fn()
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// mergeMaps returns a new map with base values overlaid by patch.
func mergeMaps(base, patch map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(patch))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range patch {
		out[k] = v
	}
	return out
}
