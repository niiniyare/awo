// Package fakestore provides an in-memory implementation of
// driver.EntityRepository[*def.EntityRecord] for use in tests.
//
// The store is thread-safe and supports the full Filter DSL. It is not a
// database — there is no schema enforcement, RLS, or constraint checking.
// Its purpose is to make unit tests fast and hermetic.
package fakestore

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"awo.so/awo/def"
	"awo.so/awo/driver"
	"awo.so/awo/filter"
	"awo.so/awo/runtime/tenant"
)

// ErrNotFound is returned when a record is not found.
var ErrNotFound = fmt.Errorf("fakestore: record not found")

// Store is an in-memory implementation of driver.EntityRepository[*def.EntityRecord].
type Store struct {
	mu      sync.RWMutex
	records map[uuid.UUID]*def.EntityRecord
}

// New creates an empty Store.
func New() *Store {
	return &Store{
		records: make(map[uuid.UUID]*def.EntityRecord),
	}
}

// Reset removes all records from the store.
func (s *Store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = make(map[uuid.UUID]*def.EntityRecord)
}

// Seed pre-populates the store with the given records.
// Records without an ID get one assigned.
func (s *Store) Seed(records ...*def.EntityRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range records {
		if r.ID == uuid.Nil {
			r.ID = uuid.New()
		}
		cp := copyRecord(r)
		s.records[cp.ID] = cp
	}
}

// Get retrieves a single record by primary key.
func (s *Store) Get(ctx context.Context, id uuid.UUID, opts ...driver.QueryOption) (*def.EntityRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.records[id]
	if !ok {
		return nil, ErrNotFound
	}
	return copyRecord(r), nil
}

// Query returns all records matching the filter with pagination applied.
func (s *Store) Query(ctx context.Context, f *filter.Filter, opts ...driver.QueryOption) ([]*def.EntityRecord, driver.PageInfo, error) {
	o := driver.ResolveOptions(opts)

	s.mu.RLock()
	all := make([]*def.EntityRecord, 0, len(s.records))
	for _, r := range s.records {
		all = append(all, r)
	}
	s.mu.RUnlock()

	// Filter
	var matched []*def.EntityRecord
	for _, r := range all {
		if f == nil || evalFilter(f, r) {
			matched = append(matched, r)
		}
	}

	// Sort
	if o.SortField != "" {
		sortRecords(matched, o.SortField, o.SortAsc)
	} else {
		// Stable order by created_at then ID
		sort.Slice(matched, func(i, j int) bool {
			ti := matched[i].CreatedAt
			tj := matched[j].CreatedAt
			if ti.Equal(tj) {
				return matched[i].ID.String() < matched[j].ID.String()
			}
			if o.SortAsc {
				return ti.Before(tj)
			}
			return ti.After(tj)
		})
	}

	total := int64(len(matched))

	// Paginate
	page := o.Page
	if page < 1 {
		page = 1
	}
	size := o.PageSize
	if size < 1 {
		size = 20
	}

	start := (page - 1) * size
	if start > len(matched) {
		start = len(matched)
	}
	end := start + size
	if end > len(matched) {
		end = len(matched)
	}
	page_records := matched[start:end]

	result := make([]*def.EntityRecord, len(page_records))
	for i, r := range page_records {
		result[i] = copyRecord(r)
	}

	pi := driver.PageInfo{
		Total:       total,
		Page:        page,
		PageSize:    size,
		HasNextPage: end < len(matched),
		HasPrevPage: start > 0,
	}

	return result, pi, nil
}

// Exists returns true if at least one record matches the filter.
func (s *Store) Exists(ctx context.Context, f *filter.Filter) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.records {
		if f == nil || evalFilter(f, r) {
			return true, nil
		}
	}
	return false, nil
}

// Count returns the number of records matching the filter.
func (s *Store) Count(ctx context.Context, f *filter.Filter) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var n int64
	for _, r := range s.records {
		if f == nil || evalFilter(f, r) {
			n++
		}
	}
	return n, nil
}

// Aggregate evaluates an aggregation spec against matching records.
func (s *Store) Aggregate(ctx context.Context, f *filter.Filter, spec driver.AggregateSpec) (driver.AggregateResult, error) {
	s.mu.RLock()
	var matched []*def.EntityRecord
	for _, r := range s.records {
		if f == nil || evalFilter(f, r) {
			matched = append(matched, r)
		}
	}
	s.mu.RUnlock()

	result := driver.AggregateResult{
		Values: make(map[string]any),
	}

	for _, fn := range spec.Functions {
		result.Values[fn.Alias] = computeAggregate(fn, matched)
	}

	return result, nil
}

// Create persists a new record.
func (s *Store) Create(ctx context.Context, input driver.CreateInput) (*def.EntityRecord, error) {
	now := time.Now().UTC()
	id := uuid.New()

	tc, hasTenant := tenant.TryFromContext(ctx)
	tenantID := uuid.Nil
	if hasTenant {
		tenantID = tc.TenantID
	}

	r := &def.EntityRecord{
		ID:           id,
		TenantID:     tenantID,
		Data:         copyMap(input.Data),
		CustomFields: copyMap(input.CustomFields),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	s.mu.Lock()
	s.records[id] = copyRecord(r)
	s.mu.Unlock()

	return r, nil
}

// Update applies a partial patch to an existing record.
func (s *Store) Update(ctx context.Context, id uuid.UUID, input driver.UpdateInput) (*def.EntityRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.records[id]
	if !ok {
		return nil, ErrNotFound
	}

	cp := copyRecord(r)
	for k, v := range input.Data {
		cp.Data[k] = v
	}
	for k, v := range input.CustomFields {
		cp.CustomFields[k] = v
	}
	cp.UpdatedAt = time.Now().UTC()

	s.records[id] = cp
	return copyRecord(cp), nil
}

// Delete removes a record.
func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.records[id]; !ok {
		return ErrNotFound
	}
	delete(s.records, id)
	return nil
}

// BulkCreate inserts multiple records atomically.
func (s *Store) BulkCreate(ctx context.Context, inputs []driver.CreateInput) ([]*def.EntityRecord, error) {
	results := make([]*def.EntityRecord, 0, len(inputs))
	for _, inp := range inputs {
		r, err := s.Create(ctx, inp)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

// BulkUpdate applies a patch to all records matching the filter.
func (s *Store) BulkUpdate(ctx context.Context, f *filter.Filter, patch driver.Patch) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var count int64
	now := time.Now().UTC()
	for id, r := range s.records {
		if f == nil || evalFilter(f, r) {
			for k, v := range patch.Set {
				s.records[id].Data[k] = v
			}
			s.records[id].UpdatedAt = now
			count++
		}
	}
	return count, nil
}

// WithTx executes fn with the same store (in-memory transactions are atomic).
func (s *Store) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// --- Filter evaluator ---

func evalFilter(f *filter.Filter, r *def.EntityRecord) bool {
	if f == nil {
		return true
	}

	switch f.Kind {
	case filter.KindAnd:
		for _, sub := range f.Sub {
			if !evalFilter(sub, r) {
				return false
			}
		}
		return true

	case filter.KindOr:
		for _, sub := range f.Sub {
			if evalFilter(sub, r) {
				return true
			}
		}
		return false

	case filter.KindNot:
		if len(f.Sub) == 0 {
			return true
		}
		return !evalFilter(f.Sub[0], r)

	case filter.KindIsNull:
		return fieldValue(r, f.Field) == nil

	case filter.KindIsNotNull:
		return fieldValue(r, f.Field) != nil

	case filter.KindEq:
		return compareValues(fieldValue(r, f.Field), f.Value) == 0

	case filter.KindNeq:
		return compareValues(fieldValue(r, f.Field), f.Value) != 0

	case filter.KindGt:
		return compareValues(fieldValue(r, f.Field), f.Value) > 0

	case filter.KindGte:
		return compareValues(fieldValue(r, f.Field), f.Value) >= 0

	case filter.KindLt:
		return compareValues(fieldValue(r, f.Field), f.Value) < 0

	case filter.KindLte:
		return compareValues(fieldValue(r, f.Field), f.Value) <= 0

	case filter.KindIn:
		if len(f.In) == 0 {
			return false
		}
		fv := fieldValue(r, f.Field)
		for _, v := range f.In {
			if compareValues(fv, v) == 0 {
				return true
			}
		}
		return false

	case filter.KindNotIn:
		fv := fieldValue(r, f.Field)
		for _, v := range f.In {
			if compareValues(fv, v) == 0 {
				return false
			}
		}
		return true

	case filter.KindContains:
		sv, ok := fieldValue(r, f.Field).(string)
		if !ok {
			return false
		}
		needle, ok := f.Value.(string)
		if !ok {
			return false
		}
		return strings.Contains(strings.ToLower(sv), strings.ToLower(needle))

	case filter.KindStartsWith:
		sv, ok := fieldValue(r, f.Field).(string)
		if !ok {
			return false
		}
		needle, ok := f.Value.(string)
		if !ok {
			return false
		}
		return strings.HasPrefix(strings.ToLower(sv), strings.ToLower(needle))

	case filter.KindEndsWith:
		sv, ok := fieldValue(r, f.Field).(string)
		if !ok {
			return false
		}
		needle, ok := f.Value.(string)
		if !ok {
			return false
		}
		return strings.HasSuffix(strings.ToLower(sv), strings.ToLower(needle))

	case filter.KindBetween:
		fv := fieldValue(r, f.Field)
		return compareValues(fv, f.Lo) >= 0 && compareValues(fv, f.Hi) <= 0

	case filter.KindCustomEq:
		cv := customFieldValue(r, f.Field)
		return compareValues(cv, f.Value) == 0

	case filter.KindCustomGt:
		cv := customFieldValue(r, f.Field)
		return compareValues(cv, f.Value) > 0

	case filter.KindCustomLt:
		cv := customFieldValue(r, f.Field)
		return compareValues(cv, f.Value) < 0

	case filter.KindCustomIn:
		cv := customFieldValue(r, f.Field)
		for _, v := range f.In {
			if compareValues(cv, v) == 0 {
				return true
			}
		}
		return false

	case filter.KindCustomNull:
		cv := customFieldValue(r, f.Field)
		if f.Value == true {
			return cv == nil
		}
		return cv != nil
	}

	return true
}

func fieldValue(r *def.EntityRecord, field string) any {
	// Check special fields first
	switch field {
	case "id":
		return r.ID
	case "tenant_id":
		return r.TenantID
	case "created_at":
		return r.CreatedAt
	case "updated_at":
		return r.UpdatedAt
	}
	if r.Data == nil {
		return nil
	}
	v, ok := r.Data[field]
	if !ok {
		return nil
	}
	return v
}

func customFieldValue(r *def.EntityRecord, field string) any {
	if r.CustomFields == nil {
		return nil
	}
	return r.CustomFields[field]
}

// compareValues returns -1, 0, or 1.
func compareValues(a, b any) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}

	// Decimal comparison
	da, aIsDecimal := toDecimal(a)
	db, bIsDecimal := toDecimal(b)
	if aIsDecimal && bIsDecimal {
		return da.Cmp(db)
	}

	// String comparison
	sa, aIsStr := a.(string)
	sb, bIsStr := b.(string)
	if aIsStr && bIsStr {
		if sa < sb {
			return -1
		}
		if sa > sb {
			return 1
		}
		return 0
	}

	// Time comparison
	ta, aIsTime := a.(time.Time)
	tb, bIsTime := b.(time.Time)
	if aIsTime && bIsTime {
		if ta.Before(tb) {
			return -1
		}
		if ta.After(tb) {
			return 1
		}
		return 0
	}

	// UUID comparison
	ua, aIsUUID := a.(uuid.UUID)
	ub, bIsUUID := b.(uuid.UUID)
	if aIsUUID && bIsUUID {
		as := ua.String()
		bs := ub.String()
		if as < bs {
			return -1
		}
		if as > bs {
			return 1
		}
		return 0
	}

	// Bool comparison
	ba, aIsBool := a.(bool)
	bb, bIsBool := b.(bool)
	if aIsBool && bIsBool {
		if ba == bb {
			return 0
		}
		if !ba {
			return -1
		}
		return 1
	}

	// Fallback: string representation
	as := fmt.Sprintf("%v", a)
	bs := fmt.Sprintf("%v", b)
	if as < bs {
		return -1
	}
	if as > bs {
		return 1
	}
	return 0
}

func toDecimal(v any) (decimal.Decimal, bool) {
	switch x := v.(type) {
	case decimal.Decimal:
		return x, true
	case int64:
		return decimal.NewFromInt(x), true
	case int:
		return decimal.NewFromInt(int64(x)), true
	case float64:
		return decimal.NewFromFloat(x), true
	case int32:
		return decimal.NewFromInt(int64(x)), true
	}
	return decimal.Zero, false
}

func sortRecords(records []*def.EntityRecord, field string, asc bool) {
	sort.SliceStable(records, func(i, j int) bool {
		vi := fieldValue(records[i], field)
		vj := fieldValue(records[j], field)
		cmp := compareValues(vi, vj)
		if asc {
			return cmp < 0
		}
		return cmp > 0
	})
}

func computeAggregate(fn driver.AggregateFunc, records []*def.EntityRecord) any {
	switch fn.Fn {
	case driver.AggregateFnCount:
		return int64(len(records))

	case driver.AggregateFnSum:
		sum := decimal.Zero
		for _, r := range records {
			if d, ok := toDecimal(fieldValue(r, fn.Field)); ok {
				sum = sum.Add(d)
			}
		}
		return sum

	case driver.AggregateFnAvg:
		if len(records) == 0 {
			return decimal.Zero
		}
		sum := decimal.Zero
		for _, r := range records {
			if d, ok := toDecimal(fieldValue(r, fn.Field)); ok {
				sum = sum.Add(d)
			}
		}
		return sum.Div(decimal.NewFromInt(int64(len(records))))

	case driver.AggregateFnMin:
		var min *decimal.Decimal
		for _, r := range records {
			if d, ok := toDecimal(fieldValue(r, fn.Field)); ok {
				if min == nil || d.LessThan(*min) {
					cp := d
					min = &cp
				}
			}
		}
		if min == nil {
			return decimal.Zero
		}
		return *min

	case driver.AggregateFnMax:
		var max *decimal.Decimal
		for _, r := range records {
			if d, ok := toDecimal(fieldValue(r, fn.Field)); ok {
				if max == nil || d.GreaterThan(*max) {
					cp := d
					max = &cp
				}
			}
		}
		if max == nil {
			return decimal.Zero
		}
		return *max
	}
	return nil
}

// --- Helpers ---

func copyRecord(r *def.EntityRecord) *def.EntityRecord {
	cp := *r
	cp.Data = copyMap(r.Data)
	cp.CustomFields = copyMap(r.CustomFields)
	return &cp
}

func copyMap(m map[string]any) map[string]any {
	if m == nil {
		return make(map[string]any)
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = copyValue(v)
	}
	return out
}

func copyValue(v any) any {
	if v == nil {
		return nil
	}
	// Deep copy maps
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Map {
		m, ok := v.(map[string]any)
		if ok {
			return copyMap(m)
		}
	}
	// Slices
	if rv.Kind() == reflect.Slice {
		cp := reflect.MakeSlice(rv.Type(), rv.Len(), rv.Cap())
		reflect.Copy(cp, rv)
		return cp.Interface()
	}
	return v
}
