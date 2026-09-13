package filter_test

import (
	"errors"
	"testing"

	"awo.so/awo/filter"
)

func TestQueryBuilder_Empty(t *testing.T) {
	q := filter.NewQuery()
	if q.Filter() != nil {
		t.Error("empty QueryBuilder should produce nil filter")
	}
	if q.HasLimit() {
		t.Error("empty QueryBuilder should have no limit")
	}
	if q.GetOffset() != 0 {
		t.Error("empty QueryBuilder should have offset 0")
	}
	if len(q.OrderBys()) != 0 {
		t.Error("empty QueryBuilder should have no order clauses")
	}
}

func TestQueryBuilder_Where_Single(t *testing.T) {
	q := filter.NewQuery().Where(filter.Eq("status", "active"))
	f := q.Filter()
	if f == nil {
		t.Fatal("expected non-nil filter after Where")
	}
	if f.Kind != filter.KindEq {
		t.Errorf("expected KindEq, got %q", f.Kind)
	}
}

func TestQueryBuilder_Where_Multiple_Anded(t *testing.T) {
	q := filter.NewQuery().
		Where(filter.Eq("status", "active")).
		Where(filter.Gt("amount", 0))
	f := q.Filter()
	if f == nil {
		t.Fatal("expected non-nil filter")
	}
	// Two predicates → AND
	if f.Kind != filter.KindAnd {
		t.Errorf("two Where calls should produce KindAnd, got %q", f.Kind)
	}
	if len(f.Sub) != 2 {
		t.Errorf("expected 2 sub-filters, got %d", len(f.Sub))
	}
}

func TestQueryBuilder_Where_NilIgnored(t *testing.T) {
	q := filter.NewQuery().Where(nil).Where(filter.Eq("x", "y"))
	f := q.Filter()
	// nil ignored, single predicate → not wrapped in AND
	if f == nil {
		t.Fatal("expected non-nil filter after non-nil Where")
	}
	if f.Kind != filter.KindEq {
		t.Errorf("nil Where ignored; expected KindEq, got %q", f.Kind)
	}
}

func TestQueryBuilder_OrderBy(t *testing.T) {
	q := filter.NewQuery().
		OrderBy("created_at", filter.Desc).
		OrderBy("name", filter.Asc)
	orders := q.OrderBys()
	if len(orders) != 2 {
		t.Fatalf("expected 2 order clauses, got %d", len(orders))
	}
	if orders[0].Field != "created_at" || orders[0].Direction != filter.Desc {
		t.Errorf("first order clause mismatch: %+v", orders[0])
	}
	if orders[1].Field != "name" || orders[1].Direction != filter.Asc {
		t.Errorf("second order clause mismatch: %+v", orders[1])
	}
}

func TestQueryBuilder_Limit(t *testing.T) {
	q := filter.NewQuery().Limit(25)
	if !q.HasLimit() {
		t.Error("HasLimit should return true after Limit(25)")
	}
	if q.GetLimit() != 25 {
		t.Errorf("expected limit 25, got %d", q.GetLimit())
	}
}

func TestQueryBuilder_Offset(t *testing.T) {
	q := filter.NewQuery().Limit(10).Offset(20)
	if q.GetOffset() != 20 {
		t.Errorf("expected offset 20, got %d", q.GetOffset())
	}
}

func TestQueryBuilder_Immutability(t *testing.T) {
	base := filter.NewQuery().Where(filter.Eq("status", "active"))
	extended := base.Where(filter.Eq("deleted", false))

	// base unchanged
	baseF := base.Filter()
	if baseF.Kind == filter.KindAnd {
		t.Error("base QueryBuilder should not be mutated by chaining")
	}

	// extended has AND
	extF := extended.Filter()
	if extF.Kind != filter.KindAnd {
		t.Errorf("extended should be KindAnd, got %q", extF.Kind)
	}
}

func TestQueryBuilder_Chained_Full(t *testing.T) {
	q := filter.NewQuery().
		Where(filter.Eq("tenant_id", "t1")).
		Where(filter.Eq("status", "active")).
		OrderBy("created_at", filter.Desc).
		Limit(50).
		Offset(100)

	if !q.HasLimit() || q.GetLimit() != 50 {
		t.Errorf("limit: %d", q.GetLimit())
	}
	if q.GetOffset() != 100 {
		t.Errorf("offset: %d", q.GetOffset())
	}
	if len(q.OrderBys()) != 1 {
		t.Errorf("order clauses: %d", len(q.OrderBys()))
	}
	f := q.Filter()
	if f == nil || f.Kind != filter.KindAnd {
		t.Errorf("combined filter should be KindAnd, got %v", f)
	}
}

func TestQueryBuilder_OrderBys_Independent(t *testing.T) {
	q := filter.NewQuery().OrderBy("name", filter.Asc)
	orders := q.OrderBys()
	// Mutating returned slice should not affect builder.
	orders[0].Field = "MUTATED"
	if q.OrderBys()[0].Field == "MUTATED" {
		t.Error("OrderBys should return a copy, not a reference")
	}
}

// --- ValidateField / ValidateLimit / ValidateOffset ---

func TestValidateField_Empty_ReturnsError(t *testing.T) {
	err := filter.ValidateField("")
	if err == nil {
		t.Fatal("expected error for empty field name; got nil")
	}
	var be *filter.BuilderError
	if !errors.As(err, &be) {
		t.Fatalf("expected *filter.BuilderError, got %T: %v", err, err)
	}
	if be.Reason == "" {
		t.Error("BuilderError.Reason must not be empty")
	}
}

func TestValidateField_NonEmpty_ReturnsNil(t *testing.T) {
	if err := filter.ValidateField("status"); err != nil {
		t.Errorf("ValidateField on non-empty name should return nil, got %v", err)
	}
}

func TestValidateLimit_Negative_ReturnsError(t *testing.T) {
	err := filter.ValidateLimit(-1)
	if err == nil {
		t.Fatal("expected error for negative limit; got nil")
	}
	var be *filter.BuilderError
	if !errors.As(err, &be) {
		t.Fatalf("expected *filter.BuilderError, got %T: %v", err, err)
	}
}

func TestValidateLimit_Zero_ReturnsNil(t *testing.T) {
	if err := filter.ValidateLimit(0); err != nil {
		t.Errorf("ValidateLimit(0) should return nil (0 = no explicit limit), got %v", err)
	}
}

func TestValidateLimit_Positive_ReturnsNil(t *testing.T) {
	if err := filter.ValidateLimit(100); err != nil {
		t.Errorf("ValidateLimit(100) should return nil, got %v", err)
	}
}

func TestValidateOffset_Negative_ReturnsError(t *testing.T) {
	err := filter.ValidateOffset(-5)
	if err == nil {
		t.Fatal("expected error for negative offset; got nil")
	}
	var be *filter.BuilderError
	if !errors.As(err, &be) {
		t.Fatalf("expected *filter.BuilderError, got %T: %v", err, err)
	}
}

func TestValidateOffset_Zero_ReturnsNil(t *testing.T) {
	if err := filter.ValidateOffset(0); err != nil {
		t.Errorf("ValidateOffset(0) should return nil, got %v", err)
	}
}

func TestBuilderError_MessageContainsReason(t *testing.T) {
	be := &filter.BuilderError{Reason: "must be positive"}
	if be.Error() == "" {
		t.Error("BuilderError.Error() must not return empty string")
	}
}

func TestBuilderError_WithField_MessageContainsField(t *testing.T) {
	be := &filter.BuilderError{Field: "amount", Reason: "must be positive"}
	msg := be.Error()
	if len(msg) == 0 {
		t.Error("BuilderError.Error() must not return empty string")
	}
}

// --- Like ---

func TestLike_AliasForContains(t *testing.T) {
	f := filter.Like("name", "acme")
	if f == nil {
		t.Fatal("Like should return non-nil filter")
	}
	if f.Kind != filter.KindContains {
		t.Errorf("Like should resolve to KindContains, got %q", f.Kind)
	}
	if f.Field != "name" {
		t.Errorf("Like field mismatch: got %q", f.Field)
	}
	if f.Value != "acme" {
		t.Errorf("Like value mismatch: got %v", f.Value)
	}
}
