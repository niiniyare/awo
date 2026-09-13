package expression_test

import (
	"testing"

	"awo.so/awo/sdui/expression"
)

func TestAMISSerializer_FieldEqLit(t *testing.T) {
	s := expression.AMISSerializer{}
	expr := expression.Eq(expression.Field("status"), expression.Lit("active"))
	got, err := s.Serialize(expr)
	if err != nil {
		t.Fatal(err)
	}
	want := "data.status === 'active'"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAMISSerializer_And(t *testing.T) {
	s := expression.AMISSerializer{}
	expr := expression.And(
		expression.Eq(expression.Field("status"), expression.Lit("active")),
		expression.Gt(expression.Field("amount"), expression.Lit(0)),
	)
	got, err := s.Serialize(expr)
	if err != nil {
		t.Fatal(err)
	}
	want := "(data.status === 'active' && data.amount > 0)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAMISSerializer_Not(t *testing.T) {
	s := expression.AMISSerializer{}
	expr := expression.Negate(expression.Eq(expression.Field("archived"), expression.Lit(true)))
	got, err := s.Serialize(expr)
	if err != nil {
		t.Fatal(err)
	}
	want := "!(data.archived === true)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAMISSerializer_In(t *testing.T) {
	s := expression.AMISSerializer{}
	expr := expression.IsIn(expression.Field("status"), "draft", "pending")
	got, err := s.Serialize(expr)
	if err != nil {
		t.Fatal(err)
	}
	want := "['draft','pending'].includes(data.status)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAMISSerializer_NilError(t *testing.T) {
	s := expression.AMISSerializer{}
	_, err := s.Serialize(nil)
	if err == nil {
		t.Error("expected error for nil expression")
	}
}

func TestAMISSerializer_StringEscape(t *testing.T) {
	s := expression.AMISSerializer{}
	expr := expression.Eq(expression.Field("name"), expression.Lit("it's"))
	got, err := s.Serialize(expr)
	if err != nil {
		t.Fatal(err)
	}
	want := `data.name === 'it\'s'`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
