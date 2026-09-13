package def

// EdgeDef declares a relationship between two entities. Edges are loaded
// explicitly via QueryOptions — the framework never lazy-loads them.
//
// At compile time, EdgeDef produces:
//   - A SQL FK constraint (OneToMany, OneToOne) or join table (ManyToMany)
//   - An amis sub-form or table widget in the SDUI detail view
//   - A preload clause in the generated query helpers
type EdgeDef struct {
	// Name is the stable identifier for this edge. Used in QueryOption preload
	// directives and API response keys.
	Name string

	// Target is the entity name of the related entity (e.g. "finance_invoice_line").
	Target string

	// Type is the cardinality of the relationship.
	Type EdgeType

	// ForeignKey is the column name on the child table that references this
	// entity's primary key. Defaults to "{parent_entity_name}_id" when empty.
	// Only relevant for EdgeOneToMany and EdgeOneToOne.
	ForeignKey string

	// JoinTable is the name of the join table for EdgeManyToMany. Defaults to
	// "{entity_a}_{entity_b}" (alphabetically ordered) when empty.
	JoinTable string

	// CascadeDelete causes the compiler to add ON DELETE CASCADE to the FK
	// constraint (OneToMany, OneToOne) or delete join table rows (ManyToMany).
	CascadeDelete bool

	// Label is the human-readable name shown in SDUI edge sections.
	Label string

	// Hidden removes this edge from SDUI views. The edge is still loadable
	// via QueryOption and accessible through the API.
	Hidden bool

	// OrderBy is the default sort for edge records in SDUI views.
	// Format: "field_name ASC" or "field_name DESC".
	OrderBy string
}
