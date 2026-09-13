package sdk

import "awo.so/awo/def"

// FieldBuilder constructs a def.FieldDef with a fluent API.
type FieldBuilder struct {
	f def.FieldDef
}

// field starts a FieldBuilder for the given name and type.
func field(name string, typ def.FieldType) *FieldBuilder {
	return &FieldBuilder{f: def.FieldDef{Name: name, Type: typ}}
}

// --- Field type constructors ---

// Data starts a FieldTypeData field (varchar(n), short string).
func Data(name string) *FieldBuilder { return field(name, def.FieldTypeData) }

// SmallText starts a FieldTypeSmallText field (varchar(1024)).
func SmallText(name string) *FieldBuilder { return field(name, def.FieldTypeSmallText) }

// LongText starts a FieldTypeLongText field (text, free-form).
func LongText(name string) *FieldBuilder { return field(name, def.FieldTypeLongText) }

// Int starts a FieldTypeInt field (bigint). Never use for money.
func Int(name string) *FieldBuilder { return field(name, def.FieldTypeInt) }

// Currency starts a FieldTypeCurrency field (numeric(20,4)). Always use for money.
func Currency(name string) *FieldBuilder { return field(name, def.FieldTypeCurrency) }

// Bool starts a FieldTypeBool field (boolean).
func Bool(name string) *FieldBuilder { return field(name, def.FieldTypeBool) }

// Date starts a FieldTypeDate field (date).
func Date(name string) *FieldBuilder { return field(name, def.FieldTypeDate) }

// DateTime starts a FieldTypeDateTime field (timestamptz, stored UTC).
func DateTime(name string) *FieldBuilder { return field(name, def.FieldTypeDateTime) }

// Select starts a FieldTypeSelect field with the given allowed values.
func Select(name string, options ...string) *FieldBuilder {
	b := field(name, def.FieldTypeSelect)
	b.f.Options = options
	return b
}

// MultiSelect starts a FieldTypeMultiSelect field.
func MultiSelect(name string, options ...string) *FieldBuilder {
	b := field(name, def.FieldTypeMultiSelect)
	b.f.Options = options
	return b
}

// JSON starts a FieldTypeJSON field (jsonb, GIN indexed).
func JSON(name string) *FieldBuilder { return field(name, def.FieldTypeJSON) }

// Link starts a FieldTypeLink field (FK to target entity).
func Link(name, target string) *FieldBuilder {
	b := field(name, def.FieldTypeLink)
	b.f.LinkTarget = target
	return b
}

// DynamicLink starts a FieldTypeDynamicLink field (polymorphic reference).
func DynamicLink(name string) *FieldBuilder { return field(name, def.FieldTypeDynamicLink) }

// NamingSeries starts a FieldTypeNamingSeries field with the given pattern.
// Pattern format: "INV-{YYYY}-{SEQ:5}".
func NamingSeries(name, series string) *FieldBuilder {
	b := field(name, def.FieldTypeNamingSeries)
	b.f.Series = series
	return b
}

// Email is a convenience constructor for a data field named with common
// email constraints (Data, 254 chars). Marks the field as requiring an
// email-shaped value — actual format validation is a FieldValidator concern.
func Email(name string) *FieldBuilder {
	return Data(name).MaxLen(254)
}

// --- Field modifiers ---

// Label sets the human-readable label shown in the SDUI.
func (b *FieldBuilder) Label(label string) *FieldBuilder {
	b.f.Label = label
	return b
}

// Description sets the help text shown under the field in SDUI forms.
func (b *FieldBuilder) Description(desc string) *FieldBuilder {
	b.f.Description = desc
	return b
}

// Required marks the field as NOT NULL (SQL) + validation error when absent.
func (b *FieldBuilder) Required() *FieldBuilder {
	b.f.Required = true
	return b
}

// Unique adds a UNIQUE constraint.
func (b *FieldBuilder) Unique() *FieldBuilder {
	b.f.Unique = true
	return b
}

// Immutable marks the field as set-once (updates rejected).
func (b *FieldBuilder) Immutable() *FieldBuilder {
	b.f.Immutable = true
	return b
}

// Sensitive excludes the field from logs and standard API responses.
func (b *FieldBuilder) Sensitive() *FieldBuilder {
	b.f.Sensitive = true
	return b
}

// Hidden removes the field from SDUI views while keeping API access.
func (b *FieldBuilder) Hidden() *FieldBuilder {
	b.f.Hidden = true
	return b
}

// ReadOnly renders the field as read-only in SDUI edit views.
func (b *FieldBuilder) ReadOnly() *FieldBuilder {
	b.f.ReadOnly = true
	return b
}

// Searchable creates a GIN trigram index for full-text substring search.
func (b *FieldBuilder) Searchable() *FieldBuilder {
	b.f.Searchable = true
	return b
}

// MaxLen sets the maximum string length.
func (b *FieldBuilder) MaxLen(n int) *FieldBuilder {
	b.f.MaxLen = n
	return b
}

// Default sets the default value factory for the field.
func (b *FieldBuilder) Default(v any) *FieldBuilder {
	b.f.Default = func() any { return v }
	return b
}

// Validator appends a custom field-level validation function.
func (b *FieldBuilder) Validator(v def.FieldValidator) *FieldBuilder {
	b.f.Validators = append(b.f.Validators, v)
	return b
}

// TenantOverridable allows tenants to customise a NamingSeries prefix.
func (b *FieldBuilder) TenantOverridable() *FieldBuilder {
	b.f.TenantOverridable = true
	return b
}

// Build returns the completed FieldDef.
func (b *FieldBuilder) Build() def.FieldDef { return b.f }
