package filter

import "fmt"

// BuilderError is returned by QueryBuilder validation methods when the caller
// provides an invalid argument (empty field name, negative limit, etc.).
// Use errors.As to detect and inspect this error type.
type BuilderError struct {
	Field  string // field name, if the error relates to a specific field
	Reason string // human-readable description
}

func (e *BuilderError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("filter: invalid argument: %s (field=%q)", e.Reason, e.Field)
	}
	return fmt.Sprintf("filter: invalid argument: %s", e.Reason)
}

// ValidateField returns a BuilderError if name is empty, or nil if valid.
// Use this in helper functions that construct *Filter nodes programmatically
// to provide a descriptive typed error instead of a panic or silent bug.
func ValidateField(name string) error {
	if name == "" {
		return &BuilderError{Reason: "field name must not be empty"}
	}
	return nil
}

// ValidateLimit returns a BuilderError if n is negative, nil if valid.
// 0 is accepted (means no explicit limit; driver default applies).
func ValidateLimit(n int) error {
	if n < 0 {
		return &BuilderError{Reason: fmt.Sprintf("limit must be non-negative, got %d", n)}
	}
	return nil
}

// ValidateOffset returns a BuilderError if n is negative, nil if valid.
func ValidateOffset(n int) error {
	if n < 0 {
		return &BuilderError{Reason: fmt.Sprintf("offset must be non-negative, got %d", n)}
	}
	return nil
}

// SortDirection specifies ascending or descending sort order.
type SortDirection string

const (
	// Asc sorts results in ascending order (A→Z, 0→9).
	Asc SortDirection = "ASC"
	// Desc sorts results in descending order (Z→A, 9→0).
	Desc SortDirection = "DESC"
)

// OrderByClause describes a single ORDER BY term.
type OrderByClause struct {
	Field     string
	Direction SortDirection
}

// Query is a composable, immutable query descriptor that accumulates WHERE
// predicates, ORDER BY clauses, LIMIT, and OFFSET. Construct with [Query]
// and chain methods to build the desired query.
//
// All methods return a new Query — the original is never mutated.
//
// Example:
//
//	q := filter.Query().
//	    Where(filter.Eq("status", "active")).
//	    Where(filter.Gt("amount", 0)).
//	    OrderBy("created_at", filter.Desc).
//	    Limit(50).
//	    Offset(0)
//
//	f := q.Filter()     // combined *Filter for EntityRepository.Query
//	ord := q.OrderBys() // []OrderByClause for SQL ORDER BY generation
type QueryBuilder struct {
	filters []*Filter
	orders  []OrderByClause
	limit   int
	offset  int
}

// NewQuery returns a new, empty QueryBuilder.
func NewQuery() *QueryBuilder {
	return &QueryBuilder{}
}

// Where adds a filter predicate. Multiple calls are ANDed together.
// Nil predicates are ignored (safe to pass PolicyFunc results directly).
func (q *QueryBuilder) Where(f *Filter) *QueryBuilder {
	if f == nil {
		return q
	}
	next := q.clone()
	next.filters = append(next.filters, f)
	return next
}

// OrderBy appends an ORDER BY clause.
func (q *QueryBuilder) OrderBy(field string, dir SortDirection) *QueryBuilder {
	next := q.clone()
	next.orders = append(next.orders, OrderByClause{Field: field, Direction: dir})
	return next
}

// Limit sets the maximum number of records to return.
// 0 means no explicit limit (driver default applies).
func (q *QueryBuilder) Limit(n int) *QueryBuilder {
	next := q.clone()
	next.limit = n
	return next
}

// Offset sets the number of records to skip before returning results.
func (q *QueryBuilder) Offset(n int) *QueryBuilder {
	next := q.clone()
	next.offset = n
	return next
}

// Filter returns the combined filter predicate. Returns nil when no Where
// clauses have been added (matches all rows).
func (q *QueryBuilder) Filter() *Filter {
	return And(q.filters...)
}

// OrderBys returns the ORDER BY clauses in declaration order.
func (q *QueryBuilder) OrderBys() []OrderByClause {
	if len(q.orders) == 0 {
		return nil
	}
	out := make([]OrderByClause, len(q.orders))
	copy(out, q.orders)
	return out
}

// GetLimit returns the LIMIT value. 0 means no explicit limit.
func (q *QueryBuilder) GetLimit() int { return q.limit }

// GetOffset returns the OFFSET value.
func (q *QueryBuilder) GetOffset() int { return q.offset }

// HasLimit reports whether an explicit limit was set.
func (q *QueryBuilder) HasLimit() bool { return q.limit > 0 }

// clone creates an independent copy of q so that chained methods do not
// mutate the original.
func (q *QueryBuilder) clone() *QueryBuilder {
	next := &QueryBuilder{
		limit:  q.limit,
		offset: q.offset,
	}
	if len(q.filters) > 0 {
		next.filters = make([]*Filter, len(q.filters))
		copy(next.filters, q.filters)
	}
	if len(q.orders) > 0 {
		next.orders = make([]OrderByClause, len(q.orders))
		copy(next.orders, q.orders)
	}
	return next
}
