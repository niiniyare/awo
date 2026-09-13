package driver

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"awo.so/awo/def"
	"awo.so/awo/filter"
)

// EntityRepository is the complete persistence interface for a single entity
// type. All framework persistence goes through this interface. Module authors
// never access the database directly.
//
// The type parameter T is the Go type representing a persisted record. For
// most entities, T is *def.EntityRecord. Typed struct representations are
// introduced by the code generator in a future phase.
//
// Every method takes context.Context as the first argument. The context must
// carry the TenantContext (set by middleware) and, inside transactions, the
// active transaction handle.
type EntityRepository[T any] interface {
	// --- Read operations ---

	// Get retrieves a single record by primary key. Returns ErrNotFound if
	// no record with that ID exists in the current tenant.
	Get(ctx context.Context, id uuid.UUID, opts ...QueryOption) (T, error)

	// Query returns all records matching the filter, in the order and page
	// specified by opts. The returned PageInfo describes the total count and
	// cursor position.
	Query(ctx context.Context, f *filter.Filter, opts ...QueryOption) ([]T, PageInfo, error)

	// Exists returns true if at least one record matches the filter.
	Exists(ctx context.Context, f *filter.Filter) (bool, error)

	// Count returns the number of records matching the filter.
	Count(ctx context.Context, f *filter.Filter) (int64, error)

	// Aggregate evaluates the aggregation spec against records matching the
	// filter.
	Aggregate(ctx context.Context, f *filter.Filter, spec AggregateSpec) (AggregateResult, error)

	// --- Write operations ---

	// Create persists a new record. The framework hook pipeline (before_create,
	// after_create) runs inside this call. Returns the persisted record with
	// ID and timestamps populated.
	Create(ctx context.Context, input CreateInput) (T, error)

	// Update applies a partial patch to an existing record. The framework hook
	// pipeline (before_update, after_update) runs inside this call.
	Update(ctx context.Context, id uuid.UUID, input UpdateInput) (T, error)

	// Delete removes a record. The framework hook pipeline (before_delete,
	// after_delete) runs inside this call.
	Delete(ctx context.Context, id uuid.UUID) error

	// --- Bulk operations ---

	// BulkCreate persists multiple records atomically. All records succeed or
	// all fail. Hooks run for each record. Performance: single multi-row INSERT.
	BulkCreate(ctx context.Context, inputs []CreateInput) ([]T, error)

	// BulkUpdate applies a patch to all records matching the filter. Returns
	// the number of rows affected. Hooks do NOT run for bulk updates.
	BulkUpdate(ctx context.Context, f *filter.Filter, patch Patch) (int64, error)

	// --- Transactions ---

	// WithTx executes fn inside a database transaction. The ctx passed to fn
	// carries the transaction; all repository operations inside fn use the
	// same connection. The transaction is committed if fn returns nil, rolled
	// back otherwise.
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// CreateInput carries the field values for a Create operation.
type CreateInput struct {
	// Data maps field names to values. Types must match FieldType conventions.
	Data map[string]any

	// CustomFields maps custom field names to values (stored in JSONB).
	CustomFields map[string]any

	// Actor is the principal performing the create. Required for audit log.
	Actor *def.Actor
}

// UpdateInput carries the field values for an Update operation.
type UpdateInput struct {
	// Data maps field names to their new values. Fields absent from Data are
	// not updated (partial update semantics).
	Data map[string]any

	// CustomFields maps custom field names to their new values. Keys absent
	// from CustomFields are not updated.
	CustomFields map[string]any

	// Actor is the principal performing the update.
	Actor *def.Actor
}

// Patch carries field values for a BulkUpdate operation.
type Patch struct {
	// Set maps field names to their new values.
	Set map[string]any
}

// PageInfo describes the pagination state after a Query call.
type PageInfo struct {
	// Total is the total number of records matching the filter (before
	// pagination). May be -1 if the driver skipped the count query for
	// performance (opt-in via QueryOption).
	Total int64

	// HasNextPage is true when more records follow the current page.
	HasNextPage bool

	// HasPrevPage is true when records precede the current page.
	HasPrevPage bool

	// NextCursor is the opaque cursor for retrieving the next page via keyset
	// pagination. Empty when HasNextPage is false.
	NextCursor string

	// PrevCursor is the opaque cursor for retrieving the previous page. Empty
	// when HasPrevPage is false.
	PrevCursor string

	// Page is the 1-based current page number (offset pagination only).
	Page int

	// PageSize is the number of records per page.
	PageSize int
}

// QueryOption modifies the behaviour of a Query call.
type QueryOption func(*QueryOptions)

// QueryOptions is the resolved set of options after all QueryOption functions
// are applied.
type QueryOptions struct {
	// Pagination
	Page     int    // 1-based; 0 means use cursor
	PageSize int    // default 20, max 200
	Cursor   string // keyset cursor; overrides Page when non-empty

	// Sorting
	SortField string // field name
	SortAsc   bool   // true = ASC, false = DESC

	// Edge preloading
	Preload []string // edge names to preload

	// Field selection
	Fields []string // field names to return; nil means all

	// Locking
	ForUpdate bool // SELECT FOR UPDATE

	// Performance
	SkipCount bool // skip COUNT(*) query; Total will be -1
}

// WithPage sets offset-based pagination.
func WithPage(page, size int) QueryOption {
	return func(o *QueryOptions) {
		o.Page = page
		o.PageSize = size
	}
}

// WithCursor sets keyset cursor pagination.
func WithCursor(cursor string, size int) QueryOption {
	return func(o *QueryOptions) {
		o.Cursor = cursor
		o.PageSize = size
	}
}

// WithSort sets the sort field and direction.
func WithSort(field string, asc bool) QueryOption {
	return func(o *QueryOptions) {
		o.SortField = field
		o.SortAsc = asc
	}
}

// WithPreload specifies edge names to preload alongside the main query.
func WithPreload(edges ...string) QueryOption {
	return func(o *QueryOptions) {
		o.Preload = append(o.Preload, edges...)
	}
}

// WithFields restricts the returned fields.
func WithFields(fields ...string) QueryOption {
	return func(o *QueryOptions) {
		o.Fields = append(o.Fields, fields...)
	}
}

// WithForUpdate locks the returned rows for the duration of the transaction.
func WithForUpdate() QueryOption {
	return func(o *QueryOptions) { o.ForUpdate = true }
}

// WithSkipCount skips the COUNT(*) query. Use when total count is not needed.
func WithSkipCount() QueryOption {
	return func(o *QueryOptions) { o.SkipCount = true }
}

// ResolveOptions applies all QueryOption functions to a default QueryOptions.
func ResolveOptions(opts []QueryOption) *QueryOptions {
	o := &QueryOptions{
		Page:     1,
		PageSize: 20,
		SortAsc:  false,
	}
	for _, opt := range opts {
		opt(o)
	}
	if o.PageSize > 200 {
		o.PageSize = 200
	}
	if o.PageSize < 1 {
		o.PageSize = 20
	}
	return o
}

// AggregateSpec describes an aggregation to compute over a filtered record set.
type AggregateSpec struct {
	// Functions is the list of aggregate functions to evaluate.
	Functions []AggregateFunc

	// GroupBy is the field name to group by. Empty means no grouping.
	GroupBy string
}

// AggregateFunc describes a single aggregate function call.
type AggregateFunc struct {
	// Fn is the aggregate function (Count, Sum, Avg, Min, Max).
	Fn AggregateFn

	// Field is the field name to aggregate. Ignored for Count.
	Field string

	// Alias is the result key in AggregateResult.Values.
	Alias string
}

// AggregateFn identifies an aggregate function.
type AggregateFn string

const (
	AggregateFnCount AggregateFn = "count"
	AggregateFnSum   AggregateFn = "sum"
	AggregateFnAvg   AggregateFn = "avg"
	AggregateFnMin   AggregateFn = "min"
	AggregateFnMax   AggregateFn = "max"
)

// AggregateResult holds the output of an aggregation.
type AggregateResult struct {
	// Values maps alias → result. For Count, the value is int64.
	// For Sum/Avg/Min/Max on Currency fields, the value is decimal.Decimal.
	// For Sum/Avg/Min/Max on Int fields, the value is int64.
	// For Sum/Avg/Min/Max on Float fields, the value is float64.
	Values map[string]any

	// Groups is non-empty when AggregateSpec.GroupBy is set. Each entry is
	// the result for one group value.
	Groups []GroupResult
}

// GroupResult is one row of a GROUP BY aggregation.
type GroupResult struct {
	// Key is the group-by field value.
	Key any

	// Values maps alias → aggregate result for this group.
	Values map[string]any
}

// MoneyValue is a helper that casts an AggregateResult value to decimal.Decimal.
// Returns decimal.Zero if the alias is absent or the value has a different type.
func (r AggregateResult) MoneyValue(alias string) decimal.Decimal {
	v, _ := r.Values[alias].(decimal.Decimal)
	return v
}

// IntValue is a helper that casts an AggregateResult value to int64.
func (r AggregateResult) IntValue(alias string) int64 {
	v, _ := r.Values[alias].(int64)
	return v
}
