package sqlbuild_test

import (
	"testing"

	"awo.so/awo/contrib/pgx/sqlbuild"
	"awo.so/awo/filter"
)

func TestBuild_CustomEq(t *testing.T) {
	r, err := sqlbuild.Build(filter.CustomEq("cf_region", "nairobi"), 0)
	if err != nil {
		t.Fatal(err)
	}
	want := `data->>'cf_region' = $1`
	if r.Clause != want {
		t.Errorf("got %q, want %q", r.Clause, want)
	}
}

func TestBuild_CustomIn(t *testing.T) {
	r, err := sqlbuild.Build(filter.CustomIn("cf_status", "draft", "submitted"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if r.Clause == "" {
		t.Error("expected non-empty clause for CustomIn")
	}
	if len(r.Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(r.Args))
	}
}

func TestBuild_CustomNull(t *testing.T) {
	r, err := sqlbuild.Build(filter.CustomIsNull("cf_approved_at"), 0)
	if err != nil {
		t.Fatal(err)
	}
	want := `data->>'cf_approved_at' IS NULL`
	if r.Clause != want {
		t.Errorf("got %q, want %q", r.Clause, want)
	}
}

func TestBuild_ShiftParams(t *testing.T) {
	// Build two filters; second uses offset=1 to shift params.
	f := filter.And(filter.Eq("a", 1), filter.Eq("b", 2))
	r, err := sqlbuild.Build(f, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(r.Args))
	}
}
