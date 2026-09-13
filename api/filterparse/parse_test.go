package filterparse_test

import (
	"testing"

	"awo.so/awo/api/filterparse"
	"awo.so/awo/filter"
)

func TestParseQueryString_NoFilterParams(t *testing.T) {
	f, err := filterparse.ParseQueryString("page=1&limit=20")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f != nil {
		t.Error("expected nil filter when no filter params")
	}
}

func TestParseQueryString_Empty(t *testing.T) {
	f, err := filterparse.ParseQueryString("")
	if err != nil {
		t.Fatal(err)
	}
	if f != nil {
		t.Error("expected nil for empty string")
	}
}

func TestParseQueryString_Eq(t *testing.T) {
	f, err := filterparse.ParseQueryString("filter[status][eq]=active")
	if err != nil {
		t.Fatal(err)
	}
	if f == nil {
		t.Fatal("expected non-nil filter")
	}
	if f.Kind != filter.KindEq {
		t.Errorf("expected KindEq, got %v", f.Kind)
	}
	if f.Field != "status" {
		t.Errorf("expected field 'status', got %q", f.Field)
	}
	if f.Value != "active" {
		t.Errorf("expected value 'active', got %v", f.Value)
	}
}

func TestParseQueryString_Neq(t *testing.T) {
	f, err := filterparse.ParseQueryString("filter[status][neq]=archived")
	if err != nil {
		t.Fatal(err)
	}
	if f.Kind != filter.KindNeq {
		t.Errorf("expected KindNeq, got %v", f.Kind)
	}
}

func TestParseQueryString_Gt(t *testing.T) {
	f, err := filterparse.ParseQueryString("filter[amount][gt]=100")
	if err != nil {
		t.Fatal(err)
	}
	if f.Kind != filter.KindGt {
		t.Errorf("expected KindGt, got %v", f.Kind)
	}
}

func TestParseQueryString_Gte(t *testing.T) {
	f, err := filterparse.ParseQueryString("filter[amount][gte]=100")
	if err != nil {
		t.Fatal(err)
	}
	if f.Kind != filter.KindGte {
		t.Errorf("expected KindGte, got %v", f.Kind)
	}
}

func TestParseQueryString_Lt(t *testing.T) {
	f, err := filterparse.ParseQueryString("filter[score][lt]=50")
	if err != nil {
		t.Fatal(err)
	}
	if f.Kind != filter.KindLt {
		t.Errorf("expected KindLt, got %v", f.Kind)
	}
}

func TestParseQueryString_Lte(t *testing.T) {
	f, err := filterparse.ParseQueryString("filter[score][lte]=50")
	if err != nil {
		t.Fatal(err)
	}
	if f.Kind != filter.KindLte {
		t.Errorf("expected KindLte, got %v", f.Kind)
	}
}

func TestParseQueryString_Contains(t *testing.T) {
	f, err := filterparse.ParseQueryString("filter[name][contains]=acme")
	if err != nil {
		t.Fatal(err)
	}
	if f.Kind != filter.KindContains {
		t.Errorf("expected KindContains, got %v", f.Kind)
	}
	if f.Value != "acme" {
		t.Errorf("value mismatch: %v", f.Value)
	}
}

func TestParseQueryString_StartsWith(t *testing.T) {
	f, err := filterparse.ParseQueryString("filter[name][starts_with]=INV-")
	if err != nil {
		t.Fatal(err)
	}
	if f.Kind != filter.KindStartsWith {
		t.Errorf("expected KindStartsWith, got %v", f.Kind)
	}
}

func TestParseQueryString_EndsWith(t *testing.T) {
	f, err := filterparse.ParseQueryString("filter[email][ends_with]=@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if f.Kind != filter.KindEndsWith {
		t.Errorf("expected KindEndsWith, got %v", f.Kind)
	}
}

func TestParseQueryString_IsNull(t *testing.T) {
	f, err := filterparse.ParseQueryString("filter[deleted_at][is_null]=1")
	if err != nil {
		t.Fatal(err)
	}
	if f.Kind != filter.KindIsNull {
		t.Errorf("expected KindIsNull, got %v", f.Kind)
	}
}

func TestParseQueryString_IsNotNull(t *testing.T) {
	f, err := filterparse.ParseQueryString("filter[submitted_at][is_not_null]=1")
	if err != nil {
		t.Fatal(err)
	}
	if f.Kind != filter.KindIsNotNull {
		t.Errorf("expected KindIsNotNull, got %v", f.Kind)
	}
}

func TestParseQueryString_In(t *testing.T) {
	f, err := filterparse.ParseQueryString("filter[status][in]=draft,submitted,paid")
	if err != nil {
		t.Fatal(err)
	}
	if f.Kind != filter.KindIn {
		t.Errorf("expected KindIn, got %v", f.Kind)
	}
	if len(f.In) != 3 {
		t.Errorf("expected 3 in-values, got %d", len(f.In))
	}
}

func TestParseQueryString_NotIn(t *testing.T) {
	f, err := filterparse.ParseQueryString("filter[status][not_in]=archived,deleted")
	if err != nil {
		t.Fatal(err)
	}
	if f.Kind != filter.KindNotIn {
		t.Errorf("expected KindNotIn, got %v", f.Kind)
	}
	if len(f.In) != 2 {
		t.Errorf("expected 2 not_in values, got %d", len(f.In))
	}
}

func TestParseQueryString_Between(t *testing.T) {
	f, err := filterparse.ParseQueryString("filter[created_at][between]=2026-01-01,2026-12-31")
	if err != nil {
		t.Fatal(err)
	}
	if f.Kind != filter.KindBetween {
		t.Errorf("expected KindBetween, got %v", f.Kind)
	}
	if f.Lo != "2026-01-01" {
		t.Errorf("Lo mismatch: %v", f.Lo)
	}
	if f.Hi != "2026-12-31" {
		t.Errorf("Hi mismatch: %v", f.Hi)
	}
}

func TestParseQueryString_Between_MissingComma(t *testing.T) {
	// Between with only one value produces nil filter (skipped gracefully).
	f, err := filterparse.ParseQueryString("filter[amount][between]=100")
	if err != nil {
		t.Fatal(err)
	}
	// Should return nil — no valid predicates.
	if f != nil {
		t.Errorf("expected nil for malformed between, got %v", f.Kind)
	}
}

func TestParseQueryString_MultipleFilters_CombinedWithAnd(t *testing.T) {
	f, err := filterparse.ParseQueryString("filter[status][eq]=active&filter[amount][gte]=100")
	if err != nil {
		t.Fatal(err)
	}
	if f == nil {
		t.Fatal("expected non-nil filter")
	}
	if f.Kind != filter.KindAnd {
		t.Errorf("expected KindAnd for multiple filters, got %v", f.Kind)
	}
	if len(f.Sub) != 2 {
		t.Errorf("expected 2 sub-filters, got %d", len(f.Sub))
	}
}

func TestParseQueryString_SingleFilter_NotWrappedInAnd(t *testing.T) {
	// Single filter should not be wrapped in an AND node.
	f, err := filterparse.ParseQueryString("filter[status][eq]=active")
	if err != nil {
		t.Fatal(err)
	}
	if f.Kind == filter.KindAnd {
		t.Error("single filter should not be wrapped in AND")
	}
}

func TestParseQueryString_UnknownOp_Ignored(t *testing.T) {
	// Unknown operators are silently ignored (nil from buildFilter).
	f, err := filterparse.ParseQueryString("filter[name][fuzzy]=smith")
	if err != nil {
		t.Fatal(err)
	}
	if f != nil {
		t.Error("unknown op should produce nil filter (skipped)")
	}
}

func TestParseQueryString_InValues_Trimmed(t *testing.T) {
	f, err := filterparse.ParseQueryString("filter[status][in]=draft, submitted , paid")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.In) != 3 {
		t.Errorf("expected 3 values, got %d", len(f.In))
	}
	if f.In[1] != "submitted" {
		t.Errorf("expected trimmed value 'submitted', got %q", f.In[1])
	}
}
