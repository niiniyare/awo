package expression_test

// Fuzz tests for the expression package.
//
// The AMISSerializer must not panic for any ExpressionNode input.
// Run: go test -fuzz=FuzzAMISSerializer ./awo/sdui/expression/...

import (
	"testing"

	"awo.so/awo/sdui/expression"
)

// FuzzAMISSerializer_StringLiterals fuzzes the string literal escaping path.
// XSS payloads, null bytes, and Unicode must not cause panics.
func FuzzAMISSerializer_StringLiterals(f *testing.F) {
	f.Add("active")
	f.Add("")
	f.Add("it's alive")
	f.Add("<script>alert(1)</script>")
	f.Add("'; DROP TABLE users; --")
	f.Add("\x00null")
	f.Add("emoji 🚀 test")
	f.Add("\\backslash\\")

	s := expression.AMISSerializer{}

	f.Fuzz(func(t *testing.T, val string) {
		expr := expression.Lit(val)
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic with literal=%q: %v", val, r)
				}
			}()
			_, _ = s.Serialize(expr)
		}()
	})
}

// FuzzAMISSerializer_FieldNames fuzzes the field name in FieldRef.
func FuzzAMISSerializer_FieldNames(f *testing.F) {
	f.Add("status")
	f.Add("")
	f.Add("nested.field")
	f.Add("field with spaces")
	f.Add("'; injection")
	f.Add(string(make([]byte, 1024)))

	s := expression.AMISSerializer{}

	f.Fuzz(func(t *testing.T, fieldName string) {
		expr := expression.Field(fieldName)
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic with field=%q: %v", fieldName, r)
				}
			}()
			_, _ = s.Serialize(expr)
		}()
	})
}

// FuzzAMISSerializer_CompareOps fuzzes compare operator strings.
func FuzzAMISSerializer_CompareOps(f *testing.F) {
	f.Add("eq")
	f.Add("neq")
	f.Add("gt")
	f.Add("gte")
	f.Add("lt")
	f.Add("lte")
	f.Add("")
	f.Add("UNKNOWN")
	f.Add("===")

	s := expression.AMISSerializer{}

	f.Fuzz(func(t *testing.T, op string) {
		expr := expression.Compare{
			Left:  expression.Field("x"),
			Right: expression.Lit(0),
			Op:    expression.CompareOp(op),
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic with op=%q: %v", op, r)
				}
			}()
			_, _ = s.Serialize(expr)
		}()
	})
}
