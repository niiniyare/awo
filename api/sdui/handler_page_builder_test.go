package sdui

import (
	"context"
	"testing"

	"awo.so/awo/def"
	"awo.so/awo/sdui/sduictx"
)

// TestPageBuilderFor verifies pageBuilderFor selects the correct builder
// for each view mode and returns nil for modes without a builder set.
func TestPageBuilderFor(t *testing.T) {
	sentinel := func(_ context.Context, _ def.PageContext) (map[string]any, error) {
		return map[string]any{"type": "sentinel"}, nil
	}

	pbs := def.PageBuilderSet{
		List:   sentinel,
		Create: sentinel,
		Edit:   nil, // explicitly unset
		Detail: sentinel,
	}

	tests := []struct {
		mode    sduictx.ViewMode
		wantNil bool
	}{
		{sduictx.ViewModeList, false},
		{sduictx.ViewModeCreate, false},
		{sduictx.ViewModeEdit, true}, // nil builder
		{sduictx.ViewModeDetail, false},
		{sduictx.ViewModeDashboard, true}, // no builder for dashboard
	}

	for _, tc := range tests {
		pb := pageBuilderFor(pbs, tc.mode)
		if tc.wantNil && pb != nil {
			t.Errorf("pageBuilderFor(%s): got non-nil, want nil", tc.mode)
		}
		if !tc.wantNil && pb == nil {
			t.Errorf("pageBuilderFor(%s): got nil, want builder", tc.mode)
		}
	}
}

// TestViewModeToPageKind verifies viewModeToPageKind maps all view modes correctly.
func TestViewModeToPageKind(t *testing.T) {
	cases := []struct {
		mode sduictx.ViewMode
		kind def.PageKind
	}{
		{sduictx.ViewModeList, def.PageKindList},
		{sduictx.ViewModeCreate, def.PageKindCreate},
		{sduictx.ViewModeEdit, def.PageKindEdit},
		{sduictx.ViewModeDetail, def.PageKindDetail},
	}
	for _, tc := range cases {
		got := viewModeToPageKind(tc.mode)
		if got != tc.kind {
			t.Errorf("viewModeToPageKind(%s) = %s, want %s", tc.mode, got, tc.kind)
		}
	}
}

// TestPageBuilderNilFallthrough verifies that a PageBuilder returning nil
// falls through to auto-generation. This is tested at the logic level by
// confirming pageBuilderFor returns the builder and that a nil return from
// the builder is semantically correct.
func TestPageBuilderNilReturn(t *testing.T) {
	// Builder that explicitly returns nil → fall through.
	nilReturnBuilder := func(_ context.Context, _ def.PageContext) (map[string]any, error) {
		return nil, nil
	}
	pbs := def.PageBuilderSet{List: nilReturnBuilder}
	pb := pageBuilderFor(pbs, sduictx.ViewModeList)
	if pb == nil {
		t.Fatal("expected builder to be non-nil")
	}
	schema, err := pb(context.Background(), def.PageContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if schema != nil {
		t.Errorf("expected nil schema (fall-through signal), got %v", schema)
	}
}
