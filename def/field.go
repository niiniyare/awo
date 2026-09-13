package def

import "context"

// FieldDef describes a single field on an entity. The compiler translates a
// FieldDef into a SQL column definition, an index (if applicable), an amis
// form widget, a CapabilityGrant scope key, and a Filter DSL predicate builder.
type FieldDef struct {
	// Name is the stable snake_case identifier for this field. It is used in:
	//   - SQL column name
	//   - API JSON key
	//   - Filter DSL predicates
	//   - Redis cache keys
	//   - CapabilityGrant entity identifiers
	// Never rename a field after data has been persisted.
	Name string

	// Type is the semantic field type. See FieldType constants.
	Type FieldType

	// Label is the human-readable display name shown in the SDUI.
	Label string

	// Description is shown as help text in the SDUI form.
	Description string

	// Default is the zero-argument function that returns the field's default
	// value when a Create operation omits the field. Return nil to signal no
	// default. The function is called once per record, not once globally.
	Default func() any

	// Required causes the compiler to add a NOT NULL constraint (SQL) and a
	// validation error when the field is absent on Create.
	Required bool

	// Unique causes the compiler to add a UNIQUE constraint.
	Unique bool

	// Immutable marks the field as set-once: updates that include this field
	// produce a BusinessError "immutable_field".
	Immutable bool

	// Sensitive excludes this field from structured logs and standard API
	// responses. It is included only in responses to callers with explicit
	// "read_sensitive" permission.
	Sensitive bool

	// Searchable causes the compiler to create a GIN trigram index on this
	// field. Only effective for FieldTypeData and FieldTypeSmallText.
	Searchable bool

	// MaxLen is the maximum string length. Only effective for FieldTypeData
	// (default 255) and FieldTypeSmallText (default 1024).
	MaxLen int

	// Min and Max constrain numeric field values. Only effective for
	// FieldTypeInt, FieldTypeFloat, and FieldTypeCurrency.
	Min *float64
	Max *float64

	// Options declares the allowed values for FieldTypeSelect and
	// FieldTypeMultiSelect. The compiler generates a SQL CHECK constraint.
	Options []string

	// LinkTarget is the entity name (e.g. "finance_customer") that this
	// FieldTypeLink or FieldTypeLinkList references. The compiler generates a
	// FK constraint.
	LinkTarget string

	// Series is the naming series pattern for FieldTypeNamingSeries fields.
	// Format: "INV-{YYYY}-{SEQ:5}" where:
	//   {YYYY}  → 4-digit year
	//   {MM}    → 2-digit month
	//   {DD}    → 2-digit day
	//   {SEQ:N} → zero-padded sequential integer reset per the ResetPeriod
	Series string

	// TenantOverridable allows tenants to customise the naming series prefix
	// through the Settings module. Only effective for FieldTypeNamingSeries.
	TenantOverridable bool

	// Validators are custom field-level validation functions. They run during
	// the VALIDATE stage of the hook pipeline, before before_save.
	Validators []FieldValidator

	// Hidden removes this field from SDUI list and form views while keeping it
	// accessible via the API. Use for internal bookkeeping fields.
	Hidden bool

	// ReadOnly renders the field as read-only in SDUI edit views.
	ReadOnly bool

	// ── AMIS expression fields ────────────────────────────────────────────────
	//
	// Expression fields accept AMIS JavaScript expression strings evaluated
	// in the browser. They reference form data via the `data` object.
	// Example: "data.status === 'active'"
	//
	// Expressions are schema-driven — no handwritten JavaScript required.
	// They are passed through to the rendered amis JSON unchanged.
	// Server-side validation is always applied regardless of expression state.

	// VisibleOn is an AMIS expression that controls visibility.
	// When truthy, the field is shown; when falsy, hidden.
	// Empty means always visible (default).
	VisibleOn string

	// HiddenOn is an AMIS expression that hides the field when truthy.
	// Inverse of VisibleOn. Empty means never hidden (default).
	HiddenOn string

	// DisabledOn is an AMIS expression that disables the field when truthy.
	// Takes precedence over ReadOnly when non-empty.
	// Example: "data.status !== 'draft'" (disable once submitted)
	DisabledOn string

	// RequiredOn is an AMIS expression that makes the field required when truthy.
	// Takes precedence over the Required boolean when non-empty.
	// Example: "data.type === 'invoice'" (conditionally required)
	RequiredOn string

	// ── SDUI display hints ────────────────────────────────────────────────────

	// Placeholder is the input placeholder text shown when the field is empty.
	// Displayed inside the input control; not shown when the field has a value.
	// Example: "Enter invoice number", "Search customers…"
	Placeholder string

	// Icon is the semantic icon name for this field.
	// Renderers map icon names to their icon library (e.g. AMIS uses fa-* names,
	// Flutter uses material icon names). Use generic semantic names such as
	// "user", "calendar", "money", "tag", "lock", "search".
	// Empty means no icon.
	Icon string

	// Width is a size hint for the field's input control.
	// Valid values: "xs", "sm", "md", "lg", "xl", "full".
	// Empty means the renderer's default width (typically "md").
	// Ignored when the section declares explicit column spans.
	Width string

	// Computed marks this field as server-computed: its value is always
	// derived from other fields or backend logic. Computed fields render as
	// read-only in forms and automatically refresh when dependent fields change.
	// The field is still included in list and detail views.
	Computed bool

	// ClearOn lists the field names whose value change causes this field to
	// reset to its zero value. Used for cascading selects and dependent lookups.
	// Example: a "product_variant" field clears when "product" changes.
	ClearOn []string
}

// FieldValidator is a custom validation function for a single field value.
// It receives the field value (typed per FieldType conventions) and returns
// an error message string. Return "" to indicate the value is valid.
//
// FieldValidators run before any database operation, inside the VALIDATE
// stage of the hook pipeline.
type FieldValidator func(ctx context.Context, value any) string
