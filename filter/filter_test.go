package filter_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"awo.so/awo/filter"
)

func TestEq_String(t *testing.T) {
	f := filter.Eq("status", "Draft")
	got := f.String()
	if got != `status eq "Draft"` {
		t.Errorf("got %q", got)
	}
}

func TestEq_Nil_BecomesIsNull(t *testing.T) {
	f := filter.Eq("deleted_at", nil)
	if f.Kind != filter.KindIsNull {
		t.Errorf("expected KindIsNull, got %v", f.Kind)
	}
}

func TestAnd_SingleFilter_Unwrapped(t *testing.T) {
	inner := filter.Eq("status", "Draft")
	result := filter.And(inner)
	if result != inner {
		t.Error("And with one filter should return that filter unchanged")
	}
}

func TestAnd_NilFilters_SkipsNil(t *testing.T) {
	a := filter.Eq("status", "Draft")
	result := filter.And(a, nil, nil)
	if result != a {
		t.Error("And should skip nil filters and return the single non-nil filter")
	}
}

func TestAnd_AllNil_ReturnsNil(t *testing.T) {
	result := filter.And(nil, nil)
	if result != nil {
		t.Error("And with all-nil filters should return nil")
	}
}

func TestOr_MultipleFilters(t *testing.T) {
	a := filter.Eq("status", "Draft")
	b := filter.Eq("status", "Submitted")
	result := filter.Or(a, b)
	if result.Kind != filter.KindOr {
		t.Errorf("expected KindOr, got %v", result.Kind)
	}
	if len(result.Sub) != 2 {
		t.Errorf("expected 2 sub-filters, got %d", len(result.Sub))
	}
}

func TestNot_NilInput_ReturnsNil(t *testing.T) {
	result := filter.Not(nil)
	if result != nil {
		t.Error("Not(nil) should return nil")
	}
}

func TestNot_WrapsFilter(t *testing.T) {
	inner := filter.Eq("archived", true)
	result := filter.Not(inner)
	if result.Kind != filter.KindNot {
		t.Errorf("expected KindNot, got %v", result.Kind)
	}
	if len(result.Sub) != 1 || result.Sub[0] != inner {
		t.Error("Not should wrap inner filter in Sub[0]")
	}
}

func TestIn_Empty_StillBuilds(t *testing.T) {
	f := filter.In("status") // no values
	if f == nil {
		t.Fatal("In with no values should return a non-nil Filter")
	}
	if f.Kind != filter.KindIn {
		t.Errorf("expected KindIn, got %v", f.Kind)
	}
}

func TestBetween(t *testing.T) {
	f := filter.Between("age", int64(18), int64(65))
	if f.Kind != filter.KindBetween {
		t.Errorf("expected KindBetween, got %v", f.Kind)
	}
	if f.Lo != int64(18) || f.Hi != int64(65) {
		t.Errorf("Lo/Hi mismatch: got %v/%v", f.Lo, f.Hi)
	}
}

func TestFilterImplementsDefFilter(t *testing.T) {
	// *filter.Filter satisfies def.Filter (empty interface) — verified by the
	// compiler whenever a *Filter is assigned to a def.Filter variable.
	var _ any = (*filter.Filter)(nil)
}

// --- String predicates ---

func TestContains(t *testing.T) {
	f := filter.Contains("name", "smith")
	if f.Kind != filter.KindContains {
		t.Errorf("expected KindContains, got %v", f.Kind)
	}
	if f.Field != "name" {
		t.Errorf("field mismatch: got %q", f.Field)
	}
	if f.Value != "smith" {
		t.Errorf("value mismatch: got %v", f.Value)
	}
	if !strings.Contains(f.String(), "smith") {
		t.Errorf("String() should contain 'smith', got %q", f.String())
	}
}

func TestStartsWith(t *testing.T) {
	f := filter.StartsWith("name", "INV-")
	if f.Kind != filter.KindStartsWith {
		t.Errorf("expected KindStartsWith, got %v", f.Kind)
	}
	if f.Value != "INV-" {
		t.Errorf("value mismatch: got %v", f.Value)
	}
}

func TestEndsWith(t *testing.T) {
	f := filter.EndsWith("email", "@example.com")
	if f.Kind != filter.KindEndsWith {
		t.Errorf("expected KindEndsWith, got %v", f.Kind)
	}
	if f.Value != "@example.com" {
		t.Errorf("value mismatch: got %v", f.Value)
	}
}

// --- Comparison predicates ---

func TestGt(t *testing.T) {
	f := filter.Gt("amount", int64(100))
	if f.Kind != filter.KindGt {
		t.Errorf("expected KindGt, got %v", f.Kind)
	}
}

func TestGte(t *testing.T) {
	f := filter.Gte("amount", int64(100))
	if f.Kind != filter.KindGte {
		t.Errorf("expected KindGte, got %v", f.Kind)
	}
}

func TestLt(t *testing.T) {
	f := filter.Lt("amount", int64(100))
	if f.Kind != filter.KindLt {
		t.Errorf("expected KindLt, got %v", f.Kind)
	}
}

func TestLte(t *testing.T) {
	f := filter.Lte("amount", int64(100))
	if f.Kind != filter.KindLte {
		t.Errorf("expected KindLte, got %v", f.Kind)
	}
}

func TestNeq(t *testing.T) {
	f := filter.Neq("status", "archived")
	if f.Kind != filter.KindNeq {
		t.Errorf("expected KindNeq, got %v", f.Kind)
	}
}

// --- Between with various types ---

func TestBetween_Decimal(t *testing.T) {
	lo := decimal.NewFromInt(100)
	hi := decimal.NewFromInt(999)
	f := filter.Between("total_kes", lo, hi)
	if f.Kind != filter.KindBetween {
		t.Errorf("expected KindBetween, got %v", f.Kind)
	}
	s := f.String()
	if !strings.Contains(s, "BETWEEN") {
		t.Errorf("String() should contain BETWEEN, got %q", s)
	}
}

func TestBetween_Time(t *testing.T) {
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)
	f := filter.Between("created_at", from, to)
	if f.Lo != from || f.Hi != to {
		t.Error("Lo/Hi time mismatch")
	}
}

// --- Null predicates ---

func TestIsNull(t *testing.T) {
	f := filter.IsNull("deleted_at")
	if f.Kind != filter.KindIsNull {
		t.Errorf("expected KindIsNull, got %v", f.Kind)
	}
	if !strings.Contains(f.String(), "IS NULL") {
		t.Errorf("String() should contain IS NULL, got %q", f.String())
	}
}

func TestIsNotNull(t *testing.T) {
	f := filter.IsNotNull("submitted_at")
	if f.Kind != filter.KindIsNotNull {
		t.Errorf("expected KindIsNotNull, got %v", f.Kind)
	}
	if !strings.Contains(f.String(), "IS NOT NULL") {
		t.Errorf("String() should contain IS NOT NULL, got %q", f.String())
	}
}

// --- Set predicates ---

func TestNotIn(t *testing.T) {
	f := filter.NotIn("status", "archived", "deleted")
	if f.Kind != filter.KindNotIn {
		t.Errorf("expected KindNotIn, got %v", f.Kind)
	}
	if len(f.In) != 2 {
		t.Errorf("expected 2 values, got %d", len(f.In))
	}
}

func TestInUUIDs(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	f := filter.InUUIDs("tenant_id", ids)
	if f.Kind != filter.KindIn {
		t.Errorf("expected KindIn, got %v", f.Kind)
	}
	if len(f.In) != 3 {
		t.Errorf("expected 3 values, got %d", len(f.In))
	}
}

func TestInStrings(t *testing.T) {
	f := filter.InStrings("status", []string{"draft", "submitted", "paid"})
	if f.Kind != filter.KindIn {
		t.Errorf("expected KindIn, got %v", f.Kind)
	}
	if len(f.In) != 3 {
		t.Errorf("expected 3 values, got %d", len(f.In))
	}
}

// --- Custom field predicates ---

func TestCustomEq(t *testing.T) {
	f := filter.CustomEq("priority", "high")
	if f.Kind != filter.KindCustomEq {
		t.Errorf("expected KindCustomEq, got %v", f.Kind)
	}
	if f.Field != "priority" || f.Value != "high" {
		t.Errorf("field/value mismatch")
	}
}

func TestCustomGt(t *testing.T) {
	v := decimal.NewFromInt(500)
	f := filter.CustomGt("score", v)
	if f.Kind != filter.KindCustomGt {
		t.Errorf("expected KindCustomGt, got %v", f.Kind)
	}
}

func TestCustomLt(t *testing.T) {
	v := decimal.NewFromInt(10)
	f := filter.CustomLt("rating", v)
	if f.Kind != filter.KindCustomLt {
		t.Errorf("expected KindCustomLt, got %v", f.Kind)
	}
}

func TestCustomIn(t *testing.T) {
	f := filter.CustomIn("region", "east", "west", "north")
	if f.Kind != filter.KindCustomIn {
		t.Errorf("expected KindCustomIn, got %v", f.Kind)
	}
	if len(f.In) != 3 {
		t.Errorf("expected 3 values, got %d", len(f.In))
	}
}

func TestCustomIsNull(t *testing.T) {
	f := filter.CustomIsNull("optional_tag")
	if f.Kind != filter.KindCustomNull {
		t.Errorf("expected KindCustomNull, got %v", f.Kind)
	}
}

// --- String() output for all kinds ---

func TestString_AllKinds(t *testing.T) {
	cases := []struct {
		f    *filter.Filter
		want string
	}{
		{filter.Eq("status", "Draft"), `status eq "Draft"`},
		{filter.Neq("status", "archived"), `status neq "archived"`},
		{filter.Gt("amount", int64(0)), "amount gt 0"},
		{filter.Gte("amount", int64(0)), "amount gte 0"},
		{filter.Lt("amount", int64(0)), "amount lt 0"},
		{filter.Lte("amount", int64(0)), "amount lte 0"},
		{filter.IsNull("deleted_at"), "deleted_at IS NULL"},
		{filter.IsNotNull("submitted_at"), "submitted_at IS NOT NULL"},
		{filter.Contains("name", "foo"), `name contains "foo"`},
		{filter.StartsWith("name", "INV"), `name starts_with "INV"`},
		{filter.EndsWith("name", ".com"), `name ends_with ".com"`},
		{filter.Between("age", int64(1), int64(99)), "age BETWEEN 1 AND 99"},
	}
	for _, c := range cases {
		got := c.f.String()
		if got != c.want {
			t.Errorf("String() = %q; want %q", got, c.want)
		}
	}
}

func TestString_Nil(t *testing.T) {
	var f *filter.Filter
	if f.String() != "<nil>" {
		t.Errorf("nil filter String() should be '<nil>', got %q", f.String())
	}
}

// --- Logical combinator String() ---

func TestAnd_String(t *testing.T) {
	f := filter.And(filter.Eq("a", "x"), filter.Eq("b", "y"))
	s := f.String()
	if !strings.HasPrefix(s, "AND(") {
		t.Errorf("And String() should start with AND(, got %q", s)
	}
}

func TestOr_String(t *testing.T) {
	f := filter.Or(filter.Eq("a", "x"), filter.Eq("b", "y"))
	s := f.String()
	if !strings.HasPrefix(s, "OR(") {
		t.Errorf("Or String() should start with OR(, got %q", s)
	}
}

func TestNot_String(t *testing.T) {
	f := filter.Not(filter.Eq("archived", true))
	s := f.String()
	if !strings.HasPrefix(s, "NOT(") {
		t.Errorf("Not String() should start with NOT(, got %q", s)
	}
}

// --- In values copy (immutability) ---

func TestIn_ValuesCopied(t *testing.T) {
	vals := []any{"a", "b", "c"}
	f := filter.In("status", vals...)
	vals[0] = "MUTATED"
	if f.In[0] == "MUTATED" {
		t.Error("In() should copy values — original slice mutation should not affect filter")
	}
}
