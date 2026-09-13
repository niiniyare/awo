// Package sdk provides fluent builder helpers that reduce boilerplate for
// module authors. The builders wrap awo/def types in a more ergonomic API
// without exposing internal packages.
//
// # Entity builder
//
//	var CustomerDef = sdk.System("crm_customer", "crm", "Customer", "Customers").
//	    Field(sdk.Data("name").Required().Searchable().MaxLen(255)).
//	    Field(sdk.Email("email").Required().Unique()).
//	    Field(sdk.Select("status", "active", "inactive").Required()).
//	    Field(sdk.Link("account_id", "crm_account").Required()).
//	    Permissions(sdk.AdminOnly()).
//	    Build()
//
// # Hook builder
//
//	type MyHook struct{}
//	func (h *MyHook) BeforeCreate(ctx context.Context, rec *def.EntityRecord) error { ... }
//
// # Workflow builder
//
//	trigger := sdk.OnCreate("InvoiceCreatedWorkflow", "finance.invoice.create").
//	    InputFn(func(rec *def.EntityRecord, tc def.TriggerContext) (any, error) {
//	        return InvoiceInput{ID: rec.ID}, nil
//	    }).
//	    Build()
package sdk

import (
	"awo.so/awo/def"
)

// --- System / Custom entity builders ---

// EntityBuilder constructs a def.SystemDefinition or def.CustomDefinition
// using a fluent API.
type EntityBuilder struct {
	name        string
	module      string
	label       string
	labelPlural string
	system      bool
	fields      []def.FieldDef
	edges       []def.EdgeDef
	hooks       def.HookSet
	perms       def.PermissionSet
	actions     []def.ActionDef
	triggers    []def.WorkflowTrigger
	pages       def.PageBuilderSet
}

// System starts building a SystemDefinition (SQL-backed, financial-eligible).
func System(name, module, label, labelPlural string) *EntityBuilder {
	return &EntityBuilder{
		name: name, module: module,
		label: label, labelPlural: labelPlural,
		system: true,
	}
}

// Custom starts building a CustomDefinition (JSONB-backed, tenant-extensible).
func Custom(name, module, label, labelPlural string) *EntityBuilder {
	return &EntityBuilder{
		name: name, module: module,
		label: label, labelPlural: labelPlural,
		system: false,
	}
}

// Field appends a field definition.
func (b *EntityBuilder) Field(f def.FieldDef) *EntityBuilder {
	b.fields = append(b.fields, f)
	return b
}

// Edge appends an edge definition.
func (b *EntityBuilder) Edge(e def.EdgeDef) *EntityBuilder {
	b.edges = append(b.edges, e)
	return b
}

// Hooks sets the hook set.
func (b *EntityBuilder) Hooks(h def.HookSet) *EntityBuilder {
	b.hooks = h
	return b
}

// Permissions sets the permission set.
func (b *EntityBuilder) Permissions(p def.PermissionSet) *EntityBuilder {
	b.perms = p
	return b
}

// Action appends a custom action definition.
func (b *EntityBuilder) Action(a def.ActionDef) *EntityBuilder {
	b.actions = append(b.actions, a)
	return b
}

// Trigger appends a workflow trigger.
func (b *EntityBuilder) Trigger(t def.WorkflowTrigger) *EntityBuilder {
	b.triggers = append(b.triggers, t)
	return b
}

// Pages sets the SDUI page builder overrides.
func (b *EntityBuilder) Pages(p def.PageBuilderSet) *EntityBuilder {
	b.pages = p
	return b
}

// Build returns the EntityDefinition. For SystemDefinition, returns
// *def.SystemDefinition; for CustomDefinition, *def.CustomDefinition.
func (b *EntityBuilder) Build() def.EntityDefinition {
	if b.system {
		return &def.SystemDefinition{
			Name: b.name, Module: b.module,
			Label: b.label, LabelPlural: b.labelPlural,
			Fields:           b.fields,
			Edges:            b.edges,
			Hooks:            b.hooks,
			Permissions:      b.perms,
			Actions:          b.actions,
			WorkflowTriggers: b.triggers,
			PageBuilders:     b.pages,
		}
	}
	return &def.CustomDefinition{
		Name: b.name, Module: b.module,
		Label: b.label, LabelPlural: b.labelPlural,
		Fields:           b.fields,
		Edges:            b.edges,
		Hooks:            b.hooks,
		Permissions:      b.perms,
		Actions:          b.actions,
		WorkflowTriggers: b.triggers,
		PageBuilders:     b.pages,
	}
}
