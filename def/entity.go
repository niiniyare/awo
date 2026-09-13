package def

// EntityDefinition is the interface implemented by both [SystemDefinition]
// and [CustomDefinition]. The registry, compiler, and runtime work against
// this interface so they do not need to distinguish between entity kinds for
// most operations.
type EntityDefinition interface {
	// EntityName returns the module-local identifier (snake_case, no module prefix).
	// Examples: "organization", "user", "customer", "org_assignment".
	//
	// The globally unique qualified name is derived by the compiler as:
	//   module + "_" + name  (e.g. "platform_organization", "iam_user")
	//
	// Never rename after data is persisted — embedded in migration filenames,
	// Temporal workflow IDs, and Redis cache keys.
	EntityName() string

	// EntityModule returns the module this entity belongs to (e.g. "platform",
	// "iam", "finance", "inventory"). Combined with EntityName() to derive the
	// globally unique qualified identifier.
	EntityModule() string

	// EntityLabel returns the human-readable singular label.
	// Derived from EntityName() if not explicitly set.
	EntityLabel() string

	// EntityLabelPlural returns the human-readable plural label.
	// Derived from EntityLabel() if not explicitly set.
	EntityLabelPlural() string

	// EntityPluralName returns an explicit plural override for the module-local
	// name (e.g. "categories" for an entity named "category"). Empty string
	// means the compiler derives the plural automatically via standard rules.
	// Only set this when automatic pluralization produces the wrong result.
	EntityPluralName() string

	// EntityDescription returns an optional description of the entity's purpose.
	// Empty string if not set.
	EntityDescription() string

	// EntityFields returns all field definitions.
	EntityFields() []FieldDef

	// EntityEdges returns all edge definitions.
	EntityEdges() []EdgeDef

	// EntityHooks returns the hook set.
	EntityHooks() HookSet

	// EntityPermissions returns the permission set.
	EntityPermissions() PermissionSet

	// EntityActions returns custom action definitions.
	EntityActions() []ActionDef

	// EntityWorkflowTriggers returns workflow trigger bindings.
	EntityWorkflowTriggers() []WorkflowTrigger

	// EntityPageBuilders returns optional SDUI page builder overrides.
	EntityPageBuilders() PageBuilderSet

	// EntityLayout returns the SDUI layout declaration for form and detail views.
	// Zero value (LayoutDef{}) produces a flat field list (default behavior).
	// Set Tabs or Sections to group fields into tabs, sections, and columns.
	EntityLayout() LayoutDef

	// EntityIcon returns the semantic icon name for this entity.
	// Used in navigation menus, list headers, and breadcrumbs.
	// Use generic semantic names: "document", "money", "user", "tag".
	// Empty string means no icon (renderer chooses default).
	EntityIcon() string

	// EntityScope returns the data isolation boundary for this entity.
	// Controls which rows a viewer can access and what context the framework
	// injects into repository queries. See [Scope] constants.
	// Default (zero value): [ScopeTenant].
	EntityScope() Scope

	// AllowAudit returns whether the framework writes audit records for
	// mutations to this entity. Defaults to true (opt-out model).
	// Set [SystemDefinition.DisableAudit] = true to opt out for
	// high-frequency internal bookkeeping entities.
	AllowAudit() bool

	// IsSystem returns true for SQL-backed system entities (typed columns),
	// false for JSONB-backed custom entities.
	IsSystem() bool
}

// SystemDefinition declares a system entity: Go-struct-backed, typed SQL
// columns, full financial and IAM eligibility.
//
// System entities are mandatory for: ledger entries, stock moves, payments,
// users, tenants, journal entries, and tax entries. Use [CustomDefinition]
// for all other entity shapes.
type SystemDefinition struct {
	// Name is the module-local entity identifier (snake_case, no module prefix).
	// Examples: "organization", "user", "org_assignment", "customer".
	//
	// The compiler derives the globally unique qualified name as:
	//   Module + "_" + Name  (e.g. "platform_organization", "iam_user")
	//
	// Never rename after data is persisted — embedded in migration filenames,
	// Temporal workflow IDs (stored for years), and Redis cache keys.
	Name string

	// Module is the business domain this entity belongs to (e.g. "platform",
	// "iam", "finance", "inventory"). Combined with Name to derive the
	// qualified identifier used in routes, tables, and cache keys.
	Module string

	// Label is the human-readable singular display name (e.g. "Invoice").
	// Derived from Name if empty: "org_assignment" → "Org Assignment".
	Label string

	// LabelPlural is the human-readable plural display name (e.g. "Invoices").
	// Derived from Label if empty: "Org Assignment" → "Org Assignments".
	LabelPlural string

	// PluralName is an explicit plural override for the module-local name used
	// in API resource paths. Leave empty unless automatic pluralization is wrong.
	// Example: entity "category" → default "categories" (correct, no override needed).
	// Example: entity "status" → default "statuses" (correct, no override needed).
	// Example: entity "sheep" → default "sheeps" (wrong) → set PluralName: "sheep".
	PluralName string

	// Description is an optional human-readable description of the entity's
	// purpose. Used in generated documentation and OpenAPI specs.
	Description string

	// Fields declares all typed columns for this entity.
	Fields []FieldDef

	// Edges declares all relationships to other entities.
	Edges []EdgeDef

	// Hooks declares the lifecycle hook implementations.
	Hooks HookSet

	// Permissions declares the RBAC gates for CRUD operations.
	Permissions PermissionSet

	// Actions declares custom action routes beyond standard CRUD.
	Actions []ActionDef

	// WorkflowTriggers declares Temporal workflow starts bound to lifecycle
	// events.
	WorkflowTriggers []WorkflowTrigger

	// PageBuilders optionally overrides auto-generated SDUI page schemas.
	PageBuilders PageBuilderSet

	// Layout declares the SDUI layout for form and detail views.
	// Zero value produces a flat field list. Set Tabs or Sections to group
	// fields into tabs, collapsible sections, and multi-column rows.
	Layout LayoutDef

	// Icon is the semantic icon name for this entity used in SDUI navigation
	// menus, list headers, and breadcrumbs. Use generic semantic names such as
	// "document", "money", "user", "tag", "building". Empty means no icon.
	Icon string

	// Scope declares the data isolation boundary for this entity.
	// Default (zero value): [ScopeTenant].
	// See [Scope] constants for available values.
	Scope Scope

	// DisableAudit opts this entity out of framework audit logging.
	// Default (zero value): false — audit is enabled by default.
	// Set true only for high-frequency internal bookkeeping entities where
	// audit volume would be prohibitive (e.g. iam_session_event counters).
	DisableAudit bool
}

// Ensure SystemDefinition implements EntityDefinition at compile time.
var _ EntityDefinition = (*SystemDefinition)(nil)

func (d *SystemDefinition) EntityName() string   { return d.Name }
func (d *SystemDefinition) EntityModule() string { return d.Module }
func (d *SystemDefinition) EntityLabel() string {
	if d.Label != "" {
		return d.Label
	}
	return DeriveLabel(LocalName(d))
}
func (d *SystemDefinition) EntityLabelPlural() string {
	if d.LabelPlural != "" {
		return d.LabelPlural
	}
	return DerivePluralLabel(d.EntityLabel())
}
func (d *SystemDefinition) EntityPluralName() string         { return d.PluralName }
func (d *SystemDefinition) EntityDescription() string        { return d.Description }
func (d *SystemDefinition) EntityFields() []FieldDef         { return d.Fields }
func (d *SystemDefinition) EntityEdges() []EdgeDef           { return d.Edges }
func (d *SystemDefinition) EntityHooks() HookSet             { return d.Hooks }
func (d *SystemDefinition) EntityPermissions() PermissionSet { return d.Permissions }
func (d *SystemDefinition) EntityActions() []ActionDef       { return d.Actions }
func (d *SystemDefinition) EntityWorkflowTriggers() []WorkflowTrigger {
	return d.WorkflowTriggers
}
func (d *SystemDefinition) EntityPageBuilders() PageBuilderSet { return d.PageBuilders }
func (d *SystemDefinition) EntityLayout() LayoutDef            { return d.Layout }
func (d *SystemDefinition) EntityIcon() string                 { return d.Icon }
func (d *SystemDefinition) EntityScope() Scope {
	if d.Scope == "" {
		return ScopeTenant
	}
	return d.Scope
}
func (d *SystemDefinition) AllowAudit() bool { return !d.DisableAudit }
func (d *SystemDefinition) IsSystem() bool   { return true }

// CustomDefinition declares a custom entity: JSONB-backed, tenant-specific
// schema, extensible at runtime via the Metadata module.
//
// Use for tenant-specific data that evolves frequently, participates in no
// financial or inventory accounting, and has low write rates.
//
// Escalate to SystemDefinition when: >10M records, fields used in financial
// calculations, or FK constraints to system entity PKs are required.
type CustomDefinition struct {
	// Name is the module-local entity identifier. Same rules as
	// SystemDefinition.Name: snake_case, no module prefix.
	Name string

	// Module is the business domain.
	Module string

	// Label is the human-readable singular display name.
	// Derived from Name if empty.
	Label string

	// LabelPlural is the human-readable plural display name.
	// Derived from Label if empty.
	LabelPlural string

	// PluralName is an explicit plural override for the module-local name.
	// Leave empty unless automatic pluralization produces the wrong result.
	PluralName string

	// Description is an optional human-readable description.
	Description string

	// Fields declares the logical fields. Each field maps to a key in the
	// JSONB document. The compiler generates GIN indexes for Searchable fields.
	Fields []FieldDef

	// Edges declares relationships to other entities.
	Edges []EdgeDef

	// Hooks declares the lifecycle hook implementations.
	Hooks HookSet

	// Permissions declares the RBAC gates.
	Permissions PermissionSet

	// Actions declares custom action routes.
	Actions []ActionDef

	// WorkflowTriggers declares Temporal workflow starts.
	WorkflowTriggers []WorkflowTrigger

	// PageBuilders optionally overrides auto-generated SDUI pages.
	PageBuilders PageBuilderSet

	// Layout declares the SDUI layout for form and detail views.
	// Zero value produces a flat field list. Set Tabs or Sections to group
	// fields into tabs, collapsible sections, and multi-column rows.
	Layout LayoutDef

	// Icon is the semantic icon name for this entity.
	// Same semantics as SystemDefinition.Icon.
	Icon string

	// Scope declares the data isolation boundary for this entity.
	// Default (zero value): [ScopeTenant].
	Scope Scope

	// DisableAudit opts this entity out of framework audit logging.
	// Default (zero value): false — audit is enabled by default.
	DisableAudit bool
}

// Ensure CustomDefinition implements EntityDefinition at compile time.
var _ EntityDefinition = (*CustomDefinition)(nil)

func (d *CustomDefinition) EntityName() string   { return d.Name }
func (d *CustomDefinition) EntityModule() string { return d.Module }
func (d *CustomDefinition) EntityLabel() string {
	if d.Label != "" {
		return d.Label
	}
	return DeriveLabel(LocalName(d))
}
func (d *CustomDefinition) EntityLabelPlural() string {
	if d.LabelPlural != "" {
		return d.LabelPlural
	}
	return DerivePluralLabel(d.EntityLabel())
}
func (d *CustomDefinition) EntityPluralName() string         { return d.PluralName }
func (d *CustomDefinition) EntityDescription() string        { return d.Description }
func (d *CustomDefinition) EntityFields() []FieldDef         { return d.Fields }
func (d *CustomDefinition) EntityEdges() []EdgeDef           { return d.Edges }
func (d *CustomDefinition) EntityHooks() HookSet             { return d.Hooks }
func (d *CustomDefinition) EntityPermissions() PermissionSet { return d.Permissions }
func (d *CustomDefinition) EntityActions() []ActionDef       { return d.Actions }
func (d *CustomDefinition) EntityWorkflowTriggers() []WorkflowTrigger {
	return d.WorkflowTriggers
}
func (d *CustomDefinition) EntityPageBuilders() PageBuilderSet { return d.PageBuilders }
func (d *CustomDefinition) EntityLayout() LayoutDef            { return d.Layout }
func (d *CustomDefinition) EntityIcon() string                 { return d.Icon }
func (d *CustomDefinition) EntityScope() Scope {
	if d.Scope == "" {
		return ScopeTenant
	}
	return d.Scope
}
func (d *CustomDefinition) AllowAudit() bool { return !d.DisableAudit }
func (d *CustomDefinition) IsSystem() bool   { return false }
