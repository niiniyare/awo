package def

// FieldType identifies the semantic type of a field declared in [FieldDef].
// The compiler maps each FieldType to its PostgreSQL column type, index
// strategy, validation rules, and SDUI widget.
type FieldType string

const (
	// Scalar types

	// FieldTypeData maps to varchar(n). Short strings, GIN trigram index when
	// [FieldDef.Searchable] is true. Default max length: 255.
	FieldTypeData FieldType = "data"

	// FieldTypeSmallText maps to varchar(1024). Medium prose. No B-tree index.
	FieldTypeSmallText FieldType = "small_text"

	// FieldTypeLongText maps to text. Free-form. No index. Use tsvector for FTS.
	FieldTypeLongText FieldType = "long_text"

	// FieldTypeInt maps to bigint. Use for counts, quantities, sequences.
	// Never use for money.
	FieldTypeInt FieldType = "int"

	// FieldTypeFloat maps to double precision. Use for scientific values and
	// percentages only. Never use for money.
	FieldTypeFloat FieldType = "float"

	// FieldTypeCurrency maps to numeric(20,4). The only correct type for
	// monetary values. Go representation: decimal.Decimal.
	FieldTypeCurrency FieldType = "currency"

	// FieldTypeBool maps to boolean.
	FieldTypeBool FieldType = "bool"

	// FieldTypeDate maps to date.
	FieldTypeDate FieldType = "date"

	// FieldTypeDateTime maps to timestamptz. Stored as UTC. Serialized as
	// EAT-offset ISO 8601 for Kenyan tenants.
	FieldTypeDateTime FieldType = "datetime"

	// FieldTypeTime maps to time.
	FieldTypeTime FieldType = "time"

	// Structured types

	// FieldTypeSelect generates a SQL CHECK constraint enforcing the declared
	// options. Options are declared in [FieldDef.Options].
	FieldTypeSelect FieldType = "select"

	// FieldTypeMultiSelect maps to a text[] array column.
	FieldTypeMultiSelect FieldType = "multi_select"

	// FieldTypeNamingSeries generates sequential, human-readable identifiers.
	// Format: "INV-{YYYY}-{SEQ:5}". Atomic counter per tenant. Configurable
	// reset. Use [FieldDef.Series] to declare the pattern.
	FieldTypeNamingSeries FieldType = "naming_series"

	// FieldTypeJSON maps to jsonb. GIN indexed.
	FieldTypeJSON FieldType = "json"

	// Relational types

	// FieldTypeLink is a foreign key reference to another entity. Generates a
	// FK column and index. Declare the target in [FieldDef.LinkTarget].
	FieldTypeLink FieldType = "link"

	// FieldTypeLinkList is a many-reference field. Each value is a UUID
	// referencing the link target entity.
	FieldTypeLinkList FieldType = "link_list"

	// FieldTypeDynamicLink is a polymorphic reference using the link_type +
	// link_name pattern. The referenced entity type is determined at runtime.
	FieldTypeDynamicLink FieldType = "dynamic_link"
)

// EdgeType describes the cardinality and ownership semantics of an [EdgeDef].
type EdgeType string

const (
	// EdgeOneToMany is a parent–children relationship. The child entity holds a
	// FK back to the parent.
	EdgeOneToMany EdgeType = "one_to_many"

	// EdgeManyToMany is a bidirectional relationship backed by a join table.
	EdgeManyToMany EdgeType = "many_to_many"

	// EdgeOneToOne is a unique FK relationship.
	EdgeOneToOne EdgeType = "one_to_one"
)

// EventType identifies the lifecycle event that triggers a [WorkflowTrigger].
type EventType string

const (
	// EventOnCreate fires after the record is persisted for the first time.
	EventOnCreate EventType = "on_create"

	// EventOnUpdate fires after the record is updated.
	EventOnUpdate EventType = "on_update"

	// EventOnDelete fires after the record is deleted.
	EventOnDelete EventType = "on_delete"

	// EventOnSubmit fires when a custom "submit" action is invoked.
	EventOnSubmit EventType = "on_submit"

	// EventOnCancel fires when a custom "cancel" action is invoked.
	EventOnCancel EventType = "on_cancel"

	// EventOnApprove fires when a custom "approve" action is invoked.
	EventOnApprove EventType = "on_approve"
)

// ActionMethod is the HTTP method for a custom [ActionDef] route.
type ActionMethod string

const (
	ActionMethodPost   ActionMethod = "POST"
	ActionMethodPatch  ActionMethod = "PATCH"
	ActionMethodDelete ActionMethod = "DELETE"
	ActionMethodGet    ActionMethod = "GET"
)

// PageKind identifies which auto-generated page a [PageBuilder] overrides.
type PageKind string

const (
	PageKindList   PageKind = "list"
	PageKindCreate PageKind = "create"
	PageKindEdit   PageKind = "edit"
	PageKindDetail PageKind = "detail"
)
