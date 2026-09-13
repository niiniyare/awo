package filter

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Kind identifies the operation a filter node represents.
type Kind string

const (
	// Leaf predicates
	KindEq         Kind = "eq"          // field = value
	KindNeq        Kind = "neq"         // field != value
	KindGt         Kind = "gt"          // field > value
	KindGte        Kind = "gte"         // field >= value
	KindLt         Kind = "lt"          // field < value
	KindLte        Kind = "lte"         // field <= value
	KindIn         Kind = "in"          // field IN (values...)
	KindNotIn      Kind = "not_in"      // field NOT IN (values...)
	KindIsNull     Kind = "is_null"     // field IS NULL
	KindIsNotNull  Kind = "is_not_null" // field IS NOT NULL
	KindContains   Kind = "contains"    // ILIKE '%value%'  (trigram-friendly)
	KindStartsWith Kind = "starts_with" // ILIKE 'value%'
	KindEndsWith   Kind = "ends_with"   // ILIKE '%value'
	KindBetween    Kind = "between"     // field BETWEEN low AND high

	// Logical combinators
	KindAnd Kind = "and" // all children must match
	KindOr  Kind = "or"  // at least one child must match
	KindNot Kind = "not" // child must not match

	// Custom-field predicates (JSONB path)
	KindCustomEq   Kind = "custom_eq" // custom_fields->>'field' = value
	KindCustomGt   Kind = "custom_gt" // (custom_fields->>'field')::numeric > value
	KindCustomLt   Kind = "custom_lt"
	KindCustomIn   Kind = "custom_in"
	KindCustomNull Kind = "custom_null"
)

// Filter is an immutable predicate node. All constructor functions return
// *Filter. The zero value is invalid — always use a constructor.
//
// Filter implements [def.Filter] so it can be returned from PolicyFunc
// without the def package importing this package.
type Filter struct {
	Kind  Kind
	Field string    // set for leaf predicates
	Value any       // set for single-value predicates
	Lo    any       // set for KindBetween (lower bound)
	Hi    any       // set for KindBetween (upper bound)
	In    []any     // set for KindIn, KindNotIn
	Sub   []*Filter // set for KindAnd, KindOr, KindNot
}

// --- Equality predicates ---

// Eq matches rows where field equals value.
// value must be a scalar: string, int64, bool, uuid.UUID, decimal.Decimal,
// time.Time, or nil (equivalent to IsNull).
func Eq(field string, value any) *Filter {
	if value == nil {
		return IsNull(field)
	}
	return &Filter{Kind: KindEq, Field: field, Value: value}
}

// Neq matches rows where field does not equal value.
func Neq(field string, value any) *Filter {
	return &Filter{Kind: KindNeq, Field: field, Value: value}
}

// --- Comparison predicates ---

// Gt matches rows where field > value.
func Gt(field string, value any) *Filter {
	return &Filter{Kind: KindGt, Field: field, Value: value}
}

// Gte matches rows where field >= value.
func Gte(field string, value any) *Filter {
	return &Filter{Kind: KindGte, Field: field, Value: value}
}

// Lt matches rows where field < value.
func Lt(field string, value any) *Filter {
	return &Filter{Kind: KindLt, Field: field, Value: value}
}

// Lte matches rows where field <= value.
func Lte(field string, value any) *Filter {
	return &Filter{Kind: KindLte, Field: field, Value: value}
}

// Between matches rows where low <= field <= high.
func Between(field string, low, high any) *Filter {
	return &Filter{Kind: KindBetween, Field: field, Lo: low, Hi: high}
}

// --- Set predicates ---

// In matches rows where field is in the provided values.
// values must be a non-empty slice of homogeneous scalars.
func In(field string, values ...any) *Filter {
	if len(values) == 0 {
		// Empty IN is always false — use a sentinel that generates FALSE.
		return &Filter{Kind: KindIn, Field: field, In: nil}
	}
	cp := make([]any, len(values))
	copy(cp, values)
	return &Filter{Kind: KindIn, Field: field, In: cp}
}

// NotIn matches rows where field is not in the provided values.
func NotIn(field string, values ...any) *Filter {
	cp := make([]any, len(values))
	copy(cp, values)
	return &Filter{Kind: KindNotIn, Field: field, In: cp}
}

// InUUIDs is a convenience wrapper for In with []uuid.UUID.
func InUUIDs(field string, ids []uuid.UUID) *Filter {
	vals := make([]any, len(ids))
	for i, id := range ids {
		vals[i] = id
	}
	return In(field, vals...)
}

// InStrings is a convenience wrapper for In with []string.
func InStrings(field string, strs []string) *Filter {
	vals := make([]any, len(strs))
	for i, s := range strs {
		vals[i] = s
	}
	return In(field, vals...)
}

// --- Null predicates ---

// IsNull matches rows where field IS NULL.
func IsNull(field string) *Filter {
	return &Filter{Kind: KindIsNull, Field: field}
}

// IsNotNull matches rows where field IS NOT NULL.
func IsNotNull(field string) *Filter {
	return &Filter{Kind: KindIsNotNull, Field: field}
}

// --- String predicates ---

// Contains matches rows where field ILIKE '%value%'.
// Trigram index is used when the field has Searchable: true.
func Contains(field, value string) *Filter {
	return &Filter{Kind: KindContains, Field: field, Value: value}
}

// StartsWith matches rows where field ILIKE 'value%'.
func StartsWith(field, value string) *Filter {
	return &Filter{Kind: KindStartsWith, Field: field, Value: value}
}

// EndsWith matches rows where field ILIKE '%value'.
func EndsWith(field, value string) *Filter {
	return &Filter{Kind: KindEndsWith, Field: field, Value: value}
}

// Like matches rows where field ILIKE '%pattern%' (case-insensitive substring).
// It is an alias for [Contains] and resolves to KindContains.
// Use [StartsWith] or [EndsWith] for prefix/suffix matching.
func Like(field, pattern string) *Filter {
	return Contains(field, pattern)
}

// --- Logical combinators ---

// And requires all sub-filters to match. Returns the single filter unchanged
// if only one is provided. Returns nil if no filters are provided.
func And(filters ...*Filter) *Filter {
	nonNil := nonNilFilters(filters)
	switch len(nonNil) {
	case 0:
		return nil
	case 1:
		return nonNil[0]
	}
	return &Filter{Kind: KindAnd, Sub: nonNil}
}

// Or requires at least one sub-filter to match. Returns the single filter
// unchanged if only one is provided. Returns nil if no filters are provided.
func Or(filters ...*Filter) *Filter {
	nonNil := nonNilFilters(filters)
	switch len(nonNil) {
	case 0:
		return nil
	case 1:
		return nonNil[0]
	}
	return &Filter{Kind: KindOr, Sub: nonNil}
}

// Not negates the sub-filter.
func Not(f *Filter) *Filter {
	if f == nil {
		return nil
	}
	return &Filter{Kind: KindNot, Sub: []*Filter{f}}
}

// --- Custom field predicates (JSONB) ---

// CustomEq matches custom_fields->>'field' = value for JSONB custom fields.
func CustomEq(field string, value any) *Filter {
	return &Filter{Kind: KindCustomEq, Field: field, Value: value}
}

// CustomGt matches (custom_fields->>'field')::numeric > value.
func CustomGt(field string, value decimal.Decimal) *Filter {
	return &Filter{Kind: KindCustomGt, Field: field, Value: value}
}

// CustomLt matches (custom_fields->>'field')::numeric < value.
func CustomLt(field string, value decimal.Decimal) *Filter {
	return &Filter{Kind: KindCustomLt, Field: field, Value: value}
}

// CustomIn matches custom_fields->>'field' IN (values...).
func CustomIn(field string, values ...string) *Filter {
	vals := make([]any, len(values))
	for i, v := range values {
		vals[i] = v
	}
	return &Filter{Kind: KindCustomIn, Field: field, In: vals}
}

// CustomIsNull matches custom_fields->>'field' IS NULL.
func CustomIsNull(field string) *Filter {
	return &Filter{Kind: KindCustomNull, Field: field, Value: true}
}

// --- Helpers ---

func nonNilFilters(filters []*Filter) []*Filter {
	out := filters[:0]
	for _, f := range filters {
		if f != nil {
			out = append(out, f)
		}
	}
	return out
}

// String returns a human-readable representation of the filter tree for
// debugging. Not intended for production logging of user data.
func (f *Filter) String() string {
	if f == nil {
		return "<nil>"
	}
	switch f.Kind {
	case KindAnd:
		parts := make([]string, len(f.Sub))
		for i, s := range f.Sub {
			parts[i] = s.String()
		}
		return fmt.Sprintf("AND(%s)", strings.Join(parts, ", "))
	case KindOr:
		parts := make([]string, len(f.Sub))
		for i, s := range f.Sub {
			parts[i] = s.String()
		}
		return fmt.Sprintf("OR(%s)", strings.Join(parts, ", "))
	case KindNot:
		return fmt.Sprintf("NOT(%s)", f.Sub[0].String())
	case KindIsNull:
		return fmt.Sprintf("%s IS NULL", f.Field)
	case KindIsNotNull:
		return fmt.Sprintf("%s IS NOT NULL", f.Field)
	case KindIn, KindNotIn:
		return fmt.Sprintf("%s %s (%d values)", f.Field, f.Kind, len(f.In))
	case KindBetween:
		return fmt.Sprintf("%s BETWEEN %v AND %v", f.Field, f.Lo, f.Hi)
	default:
		return fmt.Sprintf("%s %s %v", f.Field, f.Kind, formatValue(f.Value))
	}
}

func formatValue(v any) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case uuid.UUID:
		return val.String()
	case decimal.Decimal:
		return val.String()
	case time.Time:
		return val.Format(time.RFC3339)
	default:
		return fmt.Sprintf("%v", val)
	}
}
