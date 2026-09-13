package sqlbuild

import (
	"fmt"

	"awo.so/awo/compiler"
	"awo.so/awo/filter"
)

// FieldNotAllowedError is returned when a filter references a field name that
// is not declared in the entity schema. This prevents injection via
// dynamically-constructed field names.
type FieldNotAllowedError struct {
	Field      string
	EntityName string
}

func (e *FieldNotAllowedError) Error() string {
	return fmt.Sprintf("sqlbuild: field %q is not allowed on entity %q", e.Field, e.EntityName)
}

// TranslationError is returned when a filter cannot be translated to SQL.
// This includes unsupported filter kinds and type assertion failures during
// translation. Callers should use errors.As to detect and inspect this error.
type TranslationError struct {
	// Operator is the filter.Kind string that caused the failure.
	Operator string
	// Field is the field name involved, if applicable.
	Field string
	// Value is the offending value, if applicable.
	Value any
	// Reason is a human-readable explanation.
	Reason string
}

func (e *TranslationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("sqlbuild: %s: operator=%q field=%q", e.Reason, e.Operator, e.Field)
	}
	return fmt.Sprintf("sqlbuild: %s: operator=%q", e.Reason, e.Operator)
}

// Allowlist holds the set of permitted column names for an entity.
// Construct via [NewAllowlist] from a compiled EntitySchema.
type Allowlist struct {
	entityName string
	allowed    map[string]bool
}

// NewAllowlist builds an Allowlist from an EntitySchema. The allowed set
// includes all declared field names plus the standard framework columns
// (id, tenant_id, created_at, updated_at, deleted_at).
func NewAllowlist(es *compiler.EntitySchema) *Allowlist {
	allowed := make(map[string]bool, len(es.Fields)+8)
	// Standard framework columns present on every system entity table.
	for _, col := range []string{"id", "tenant_id", "created_at", "updated_at", "deleted_at"} {
		allowed[col] = true
	}
	for _, f := range es.Fields {
		allowed[f.Name] = true
	}
	return &Allowlist{entityName: es.QualifiedName, allowed: allowed}
}

// NewManualAllowlist builds an Allowlist from an explicit field list.
// The standard framework columns are always included.
// Intended for testing and for callers that do not have an EntitySchema.
func NewManualAllowlist(entityName string, fields []string) *Allowlist {
	allowed := make(map[string]bool, len(fields)+8)
	for _, col := range []string{"id", "tenant_id", "created_at", "updated_at", "deleted_at"} {
		allowed[col] = true
	}
	for _, f := range fields {
		allowed[f] = true
	}
	return &Allowlist{entityName: entityName, allowed: allowed}
}

// Allow reports whether field is in the allowlist.
func (a *Allowlist) Allow(field string) bool {
	return a.allowed[field]
}

// Check returns a FieldNotAllowedError when field is not permitted, nil otherwise.
func (a *Allowlist) Check(field string) error {
	if !a.allowed[field] {
		return &FieldNotAllowedError{Field: field, EntityName: a.entityName}
	}
	return nil
}

// BuildWithAllowlist translates f into a WHERE clause, verifying every field
// reference against the allowlist. Returns FieldNotAllowedError if any field
// is not declared on the entity.
//
// Custom-field predicates (KindCustom*) are exempt from allowlist checks
// because they use JSONB path operators and never map to column names.
func BuildWithAllowlist(f *filter.Filter, paramOffset int, al *Allowlist) (Result, error) {
	if f == nil {
		return Result{}, nil
	}
	if err := checkFields(f, al); err != nil {
		return Result{}, err
	}
	return Build(f, paramOffset)
}

// checkFields recursively validates that every leaf field name in f is
// permitted by al. Custom-field predicates are exempt.
func checkFields(f *filter.Filter, al *Allowlist) error {
	if f == nil {
		return nil
	}
	switch f.Kind {
	case filter.KindCustomEq, filter.KindCustomGt, filter.KindCustomLt,
		filter.KindCustomIn, filter.KindCustomNull:
		// JSONB path predicates — field is a JSONB key, not a column name.
		return nil

	case filter.KindAnd, filter.KindOr, filter.KindNot:
		for _, sub := range f.Sub {
			if err := checkFields(sub, al); err != nil {
				return err
			}
		}
		return nil

	default:
		// Leaf predicate — validate column name.
		if f.Field != "" {
			if err := al.Check(f.Field); err != nil {
				return err
			}
		}
		return nil
	}
}

